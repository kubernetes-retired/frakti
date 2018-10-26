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
	"net/http"
	"os"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/util/alternativeruntime"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	kubeletconfiginternal "k8s.io/kubernetes/pkg/kubelet/apis/config"
	kubeletscheme "k8s.io/kubernetes/pkg/kubelet/apis/config/scheme"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	dockerremote "k8s.io/kubernetes/pkg/kubelet/dockershim/remote"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	// NOTE(harry): all consts defined here mean user configure of kubelet like NonMasqueradeCIDR, will be ignored.
	// This can be fixed when dockershim become independent, then we can delete all these default values.
	networkPluginName = "cni"
	networkPluginMTU  = 1460
	nonMasqueradeCIDR = "10.0.0.0/8"
)

type PrivilegedRuntime struct {
	dockershim.DockerService
}

func (p *PrivilegedRuntime) ServiceName() string {
	return alternativeruntime.PrivilegedRuntimeName
}

func NewPrivilegedRuntimeService(privilegedRuntimeEndpoint string, streamingConfig *streaming.Config, cniNetDir, cniPluginDir, cgroupDriver, privilegedRuntimeRootDir, podSandboxImage string) (*PrivilegedRuntime, error) {
	// For now we use docker as the only supported privileged runtime
	glog.Infof("Initialize privileged runtime: docker runtime\n")

	kubeletScheme, _, err := kubeletscheme.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	external := &kubeletconfiginternal.KubeletConfiguration{}
	kubeletScheme.Default(external)
	kubeCfg := &kubeletconfig.KubeletConfiguration{}
	if err := kubeletScheme.Convert(external, kubeCfg, nil); err != nil {
		return nil, err
	}

	crOption := options.NewContainerRuntimeOptions()

	dockerClientConfig := &dockershim.ClientConfig{
		DockerEndpoint:            privilegedRuntimeEndpoint,
		RuntimeRequestTimeout:     kubeCfg.RuntimeRequestTimeout.Duration,
		ImagePullProgressDeadline: crOption.ImagePullProgressDeadline.Duration,
	}

	// NOTE(harry): pluginSettings should be arguments for dockershim, not part of kubelet.
	// But standalone dockershim is not ready yet, so we use default values here.
	pluginSettings := dockershim.NetworkPluginSettings{
		//HairpinMode:        kubeletconfiginternal.HairpinMode(kubeCfg.HairpinMode),
		HairpinMode:        kubeletconfiginternal.HairpinMode(kubeletconfig.HairpinNone),
		NonMasqueradeCIDR:  nonMasqueradeCIDR,
		PluginName:         networkPluginName,
		PluginConfDir:      cniNetDir,
		PluginBinDirString: cniPluginDir,
		MTU:                networkPluginMTU,
	}

	if len(podSandboxImage) != 0 {
		crOption.PodSandboxImage = podSandboxImage
	}

	ds, err := dockershim.NewDockerService(
		dockerClientConfig,
		crOption.PodSandboxImage,
		streamingConfig,
		&pluginSettings,
		// RuntimeCgroups is optional, so we will not set it here.
		"",
		// If dockershim detected this cgroupDriver is different with dockerd, it will fail.
		cgroupDriver,
		privilegedRuntimeRootDir,
		false,
	)
	if err != nil {
		return nil, err
	}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(fraktiDockerShim, ds)
	if err := server.Start(); err != nil {
		return nil, err
	}

	// start streaming server by using dockerService
	startPrivilegedStreamingServer(streamingConfig, ds)

	// init client
	if err := getRuntimeClient(); err != nil {
		return nil, err
	}
	if err := getImageClient(); err != nil {
		return nil, err
	}

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
