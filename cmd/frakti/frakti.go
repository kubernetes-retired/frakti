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
	"os"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/alternativeruntime"
	"k8s.io/frakti/pkg/hyper"
	"k8s.io/frakti/pkg/manager"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	fraktiVersion = "0.1"

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
	alternativeRuntimeEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock",
		"The endpoint of alternative runtime to communicate with")
	enableAlternativeRuntime = flag.Bool("enable-alternative-runtime", true, "Enable alternative runtime to handle OS containers, default is true")
)

func main() {
	flag.Parse()

	if *version {
		glog.Infof("frakti version: %s\n", fraktiVersion)
		os.Exit(0)
	}

	// 1. Initialize hyper runtime and streaming server
	streamingConfig := getStreamingConfig()
	hyperRuntime, streamingServer, err := hyper.NewHyperRuntime(*hyperEndpoint, streamingConfig, *cniNetDir, *cniPluginDir)
	if err != nil {
		glog.Errorf("Initialize hyper runtime failed: %v", err)
		os.Exit(1)
	}

	// 2. Initialize alternative runtime and start its own streaming server
	alternativeRuntime, err := alternativeruntime.NewAlternativeRuntimeService(
		*alternativeRuntimeEndpoint,
		getAlternativeStreamingConfig(),
		*cniNetDir,
		*cniPluginDir,
	)
	if err != nil && *enableAlternativeRuntime {
		glog.Errorf("Initialize alternative runtime failed: %v", err)
		os.Exit(1)
	}

	// 3. Initialize frakti manager with two runtimes above
	server, err := manager.NewFraktiManager(hyperRuntime, hyperRuntime, streamingServer, alternativeRuntime, alternativeRuntime)
	if err != nil {
		glog.Errorf("Initialize frakti server failed: %v", err)
		os.Exit(1)
	}

	fmt.Println(server.Serve(*listen))
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
