/*
Copyright 2016 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"k8s.io/frakti/pkg/hyper"
	"k8s.io/frakti/pkg/manager"
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
	fraktiVersion = "0.1"
	// TODO(resouer) frakti use cni by default, should we make this configurable?
	networkPluginName = "cni"
	networkPluginMTU  = 1460

	// use port 22522 for dockershim streaming
	alternativeStreamingServerPort = 22522
)

var (
	version = flag.Bool("version", false, "Print version and exit")
	listen  = flag.String("listen", "/var/run/frakti.sock",
		"The sockets to listen on, e.g. /var/run/frakti.sock")
	hyperEndpoint = flag.String("hyper-endpoint", "127.0.0.1:22318",
		"The endpoint for connecting hyperd, e.g. 127.0.0.1:22318")
	streamingServerPort = flag.String("streaming-server-port", "22521",
		"The port for the streaming server to serve on, e.g. 22521")
	streamingServerAddress = flag.String("streaming-server-addr", "0.0.0.0",
		"The IP address for the streaming server to serve on, e.g. 0.0.0.0")
	cniNetDir = flag.String("cni-net-dir", "/etc/cni/net.d",
		"The directory for putting cni configuration file")
	cniPluginDir = flag.String("cni-plugin-dir", "/opt/cni/bin",
		"The directory for putting cni plugin binary file")
	alternativeRuntimeEndpoint = flag.String("alternative-runtime", "unix:///var/run/docker.sock",
		"The endpoint of alternative runtime to communicate with")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("frakti version: %s\n", fraktiVersion)
		os.Exit(0)
	}

	// 1. Initialize hyper runtime and streaming server
	streamingConfig := getStreamingConfig()
	hyperRuntime, streamingServer, err := hyper.NewHyperRuntime(*hyperEndpoint, streamingConfig, *cniNetDir, *cniPluginDir)
	if err != nil {
		fmt.Println("Initialize hyper runtime failed: ", err)
		os.Exit(1)
	}

	// 2. Initialize alternative runtime and start its own streaming server
	alternativeStreamingConfig := getAlternativeStreamingConfig()
	ds, err := initAlternativeRuntimeService(alternativeStreamingConfig)
	if err != nil {
		// TODO(harry) Do we want to make alternative runtime optional?
		fmt.Println("Initialize alternative runtime failed: ", err)
		os.Exit(1)
	}
	startAlternativeStreamingServer(alternativeStreamingConfig, ds)

	// 3. Initialize frakti manager with two runtimes above
	server, err := manager.NewFraktiManager(hyperRuntime, hyperRuntime, streamingServer, ds, ds)
	if err != nil {
		fmt.Println("Initialize frakti server failed: ", err)
		os.Exit(1)
	}

	fmt.Println(server.Serve(*listen))
}

func startAlternativeStreamingServer(streamingConfig *streaming.Config, ds dockershim.DockerService) {
	httpServer := &http.Server{
		Addr:    streamingConfig.Addr,
		Handler: ds,
	}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			fmt.Printf("Failed to start streaming server for alternative runtime: %v", err)
			os.Exit(1)
		}
	}()
}

func initAlternativeRuntimeService(streamingConfig *streaming.Config) (dockershim.DockerService, error) {
	// For now we use docker as the only supported alternative runtime
	fmt.Printf("Initialize alternative runtime: docker runtime\n")
	kubeCfg := &componentconfigv1alpha1.KubeletConfiguration{}
	componentconfigv1alpha1.SetDefaults_KubeletConfiguration(kubeCfg)
	dockerClient := dockertools.ConnectToDockerOrDie(
		kubeCfg.DockerEndpoint,
		kubeCfg.RuntimeRequestTimeout.Duration,
		kubeCfg.ImagePullProgressDeadline.Duration,
	)
	// TODO(resouer) is it fine to reuse the CNI plug-in?
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:       componentconfig.HairpinMode(kubeCfg.HairpinMode),
		NonMasqueradeCIDR: kubeCfg.NonMasqueradeCIDR,
		PluginName:        networkPluginName,
		PluginConfDir:     *cniNetDir,
		PluginBinDir:      *cniPluginDir,
		MTU:               networkPluginMTU,
	}
	var nl *noOpLegacyHost
	pluginSettings.LegacyRuntimeHost = nl
	return dockershim.NewDockerService(
		dockerClient,
		kubeCfg.SeccompProfileRoot,
		kubeCfg.PodInfraContainerImage,
		streamingConfig,
		&pluginSettings,
		kubeCfg.RuntimeCgroups,
		kubeCfg.CgroupDriver,
		&dockertools.NativeExecHandler{},
	)
}

func generateStreamingConfigInternal() *streaming.Config {
	return &streaming.Config{
		StreamIdleTimeout:               streaming.DefaultConfig.StreamIdleTimeout,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
		// TODO: add TLSConfig
	}
}

// Gets the streaming server configuration to use with in-process CRI shims.
func getStreamingConfig() *streaming.Config {
	config := generateStreamingConfigInternal()
	config.Addr = fmt.Sprintf("%s:%s", *streamingServerAddress, *streamingServerPort)
	return config
}

// Gets the streaming server configuration to use with in-process alternative shims.
func getAlternativeStreamingConfig() *streaming.Config {
	config := generateStreamingConfigInternal()
	config.Addr = fmt.Sprintf("%s:%d", *streamingServerAddress, alternativeStreamingServerPort)
	return config
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
