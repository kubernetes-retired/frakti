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

package docker

import (
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/util/alternativeruntime"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet"
	"k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
	kubeletconfiginternal "k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
	kubeletscheme "k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/scheme"
	kubeletconfigv1alpha1 "k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	networkPluginName = "cni"
	networkPluginMTU  = 1460
)

type PrivilegedRuntime struct {
	dockershim.DockerService
}

func (p *PrivilegedRuntime) ServiceName() string {
	return alternativeruntime.PrivilegedRuntimeName
}

func NewPrivilegedRuntimeService(privilegedRuntimeEndpoint string, streamingConfig *streaming.Config, cniNetDir, cniPluginDir, cgroupDriver, privilegedRuntimeRootDir string) (*PrivilegedRuntime, error) {
	// For now we use docker as the only supported privileged runtime
	glog.Infof("Initialize privileged runtime: docker runtime\n")

	kubeletScheme, _, err := kubeletscheme.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	external := &kubeletconfigv1alpha1.KubeletConfiguration{}
	kubeletScheme.Default(external)
	kubeCfg := &kubeletconfig.KubeletConfiguration{}
	if err := kubeletScheme.Convert(external, kubeCfg, nil); err != nil {
		return nil, err
	}

	crOption := options.NewContainerRuntimeOptions()
	dockerClient := libdocker.ConnectToDockerOrDie(
		// privilegedRuntimeEndpoint defaults to kubeCfg.DockerEndpoint
		privilegedRuntimeEndpoint,
		kubeCfg.RuntimeRequestTimeout.Duration,
		crOption.ImagePullProgressDeadline.Duration,
	)
	// TODO(resouer) is it fine to reuse the CNI plug-in?
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:       kubeletconfiginternal.HairpinMode(kubeCfg.HairpinMode),
		NonMasqueradeCIDR: kubeCfg.NonMasqueradeCIDR,
		PluginName:        networkPluginName,
		PluginConfDir:     cniNetDir,
		PluginBinDir:      cniPluginDir,
		MTU:               networkPluginMTU,
	}
	var nl *kubelet.NoOpLegacyHost
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
		crOption.PodSandboxImage,
		streamingConfig,
		&pluginSettings,
		kubeCfg.RuntimeCgroups,
		cgroupDriver,
		crOption.DockerExecHandlerName,
		privilegedRuntimeRootDir,
		crOption.DockerDisableSharedPID,
	)
	if err != nil {
		return nil, err
	}

	// start streaming server by using dockerService
	startPrivilegedStreamingServer(streamingConfig, ds)

	return &PrivilegedRuntime{ds}, nil
}

func startPrivilegedStreamingServer(streamingConfig *streaming.Config, ds dockershim.DockerService) {
	httpServer := &http.Server{
		Addr:    streamingConfig.Addr,
		Handler: ds,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			glog.Errorf("Failed to start streaming server for privileged runtime: %v", err)
			os.Exit(1)
		}
	}()
}
