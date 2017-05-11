/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alternativeruntime

import (
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	componentconfigv1alpha1 "k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"

	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	networkPluginName = "cni"
	networkPluginMTU  = 1460
)

type AternativeRuntime struct {
	dockershim.DockerService
}

func (a *AternativeRuntime) ServiceName() string {
	return "alternative runtime service"
}

func NewAlternativeRuntimeService(alternativeRuntimeEndpoint string, streamingConfig *streaming.Config, cniNetDir, cniPluginDir, cgroupDriver, alternativeRuntimeRootDir string) (*AternativeRuntime, error) {
	// For now we use docker as the only supported alternative runtime
	glog.Infof("Initialize alternative runtime: docker runtime\n")

	extKubeCfg := &componentconfigv1alpha1.KubeletConfiguration{}
	componentconfigv1alpha1.SetDefaults_KubeletConfiguration(extKubeCfg)
	kubeCfg := &componentconfig.KubeletConfiguration{}
	if err := api.Scheme.Convert(extKubeCfg, kubeCfg, nil); err != nil {
		return nil, err
	}
	dockerClient := libdocker.ConnectToDockerOrDie(
		// alternativeRuntimeEndpoint defaults to kubeCfg.DockerEndpoint
		alternativeRuntimeEndpoint,
		kubeCfg.RuntimeRequestTimeout.Duration,
		kubeCfg.ImagePullProgressDeadline.Duration,
	)
	// TODO(resouer) is it fine to reuse the CNI plug-in?
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:       componentconfig.HairpinMode(kubeCfg.HairpinMode),
		NonMasqueradeCIDR: kubeCfg.NonMasqueradeCIDR,
		PluginName:        networkPluginName,
		PluginConfDir:     cniNetDir,
		PluginBinDir:      cniPluginDir,
		MTU:               networkPluginMTU,
	}
	var nl *noOpLegacyHost
	pluginSettings.LegacyRuntimeHost = nl
	// set cgroup driver to dockershim
	dockerInfo, err := dockerClient.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get info from docker: %v", err)
	}
	if len(dockerInfo.CgroupDriver) == 0 {
		glog.Warningf("No cgroup driver is set in Docker, use frakti configuration: %q", cgroupDriver)
	} else if dockerInfo.CgroupDriver != cgroupDriver {
		return nil, fmt.Errorf("misconfiguration: frakti cgroup driver: %q is different from docker cgroup driver: %q", dockerInfo.CgroupDriver, cgroupDriver)
	}
	ds, err := dockershim.NewDockerService(
		dockerClient,
		kubeCfg.SeccompProfileRoot,
		kubeCfg.PodInfraContainerImage,
		streamingConfig,
		&pluginSettings,
		kubeCfg.RuntimeCgroups,
		cgroupDriver,
		kubeCfg.DockerExecHandlerName,
		alternativeRuntimeRootDir,
		kubeCfg.DockerDisableSharedPID,
	)
	if err != nil {
		return nil, err
	}

	// start streaming server by using dockerService
	startAlternativeStreamingServer(streamingConfig, ds)

	return &AternativeRuntime{ds}, nil
}

func startAlternativeStreamingServer(streamingConfig *streaming.Config, ds dockershim.DockerService) {
	httpServer := &http.Server{
		Addr:    streamingConfig.Addr,
		Handler: ds,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			glog.Errorf("Failed to start streaming server for alternative runtime: %v", err)
			os.Exit(1)
		}
	}()
}

// noOpLegacyHost implements the network.LegacyHost interface for the remote
// runtime shim by just returning empties. It doesn't support legacy features
// like host port and bandwidth shaping.
type noOpLegacyHost struct{}

func (n *noOpLegacyHost) GetPodByName(namespace, name string) (*v1.Pod, bool) {
	return nil, true
}

func (n *noOpLegacyHost) GetKubeClient() clientset.Interface {
	return nil
}

func (n *noOpLegacyHost) GetRuntime() kubecontainer.Runtime {
	return nil
}

func (nh *noOpLegacyHost) SupportsLegacyFeatures() bool {
	return false
}
