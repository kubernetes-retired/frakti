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

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	componentconfigv1alpha1 "k8s.io/kubernetes/pkg/apis/componentconfig/v1alpha1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"

	"k8s.io/kubernetes/pkg/kubelet/dockertools"
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

func NewAlternativeRuntimeService(alternativeRuntimeEndpoint string, streamingConfig *streaming.Config, cniNetDir string, cniPluginDir string) (*AternativeRuntime, error) {
	// For now we use docker as the only supported alternative runtime
	glog.Infof("Initialize alternative runtime: docker runtime\n")

	// If we use alternative runtime, we should use CNI. So let's check if CNI directories are properly configured
	if _, err := os.Stat(cniNetDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("dockershim requires CNI network, but %s does not exist", cniNetDir)
	}
	if _, err := os.Stat(cniPluginDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("dockershim requires CNI network, but %s does not exist", cniPluginDir)
	}

	kubeCfg := &componentconfigv1alpha1.KubeletConfiguration{}
	componentconfigv1alpha1.SetDefaults_KubeletConfiguration(kubeCfg)
	dockerClient := dockertools.ConnectToDockerOrDie(
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
	ds, err := dockershim.NewDockerService(
		dockerClient,
		kubeCfg.SeccompProfileRoot,
		kubeCfg.PodInfraContainerImage,
		streamingConfig,
		&pluginSettings,
		kubeCfg.RuntimeCgroups,
		kubeCfg.CgroupDriver,
		&dockertools.NativeExecHandler{},
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
