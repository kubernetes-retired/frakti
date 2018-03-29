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
	"fmt"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/frakti/pkg/docker"
	"k8s.io/frakti/pkg/hyper"
	"k8s.io/frakti/pkg/manager"
	unikernel "k8s.io/frakti/pkg/unikernel/service"
	"k8s.io/frakti/pkg/util/flags"
	"k8s.io/frakti/pkg/util/network"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	fraktiVersion = "1.9.2"

	// use port 22522 for dockershim streaming
	privilegedStreamingServerPort = "22522"
)

var (
	listen = pflag.String("listen", "/var/run/frakti.sock",
		"The sockets to listen on, e.g. /var/run/frakti.sock")
	hyperEndpoint = pflag.String("hyper-endpoint", "127.0.0.1:22318",
		"The endpoint for connecting hyperd, e.g. 127.0.0.1:22318")
	streamingServerPort = pflag.String("streaming-server-port", "22521",
		"The port for the streaming server to serve on, e.g. 22521")
	streamingServerAddress = pflag.String("streaming-server-addr", "",
		"The IP address for the streaming server to serve on, should not be 0.0.0.0 or 127.0.0.1")
	cniNetDir = pflag.String("cni-net-dir", "/etc/cni/net.d",
		"The directory for putting cni configuration file")
	cniPluginDir = pflag.String("cni-plugin-dir", "/opt/cni/bin",
		"The directory for putting cni plugin binary file")
	privilegedRuntimeEndpoint = pflag.String("docker-endpoint", "unix:///var/run/docker.sock",
		"The endpoint of privileged runtime to communicate with")
	enablePrivilegedRuntime = pflag.Bool("enable-privileged-runtime", true, "Enable privileged runtime to handle OS containers, default is true")
	enableUnikernelRuntime  = pflag.Bool("enable-unikernel-runtime", false, "Enable unikernel runtime to run containers using unikernel image, default is false. Still under development.")
	enableUnikernelLog      = pflag.Bool("enable-unikernel-log", true, "Enable unikernel runtime's log allow print VM's log to kubelet specified file, while disable some ability when `virsh console` to VM.")
	cgroupDriver            = pflag.String("cgroup-driver", "cgroupfs", "Driver that the frakti uses to manipulate cgroups on the host. *SHOULD BE SAME AS* kubelet cgroup driver configuration.  Possible values: 'cgroupfs', 'systemd'")
	rootDir                 = pflag.String("root-directory", "/var/lib/frakti", "Path to the frakti root directory")
	defaultCPUNum           = pflag.Int32("cpu", 1, "Default CPU in number for HyperVM when cpu limit is not specified for the pod")
	defaultMemoryMB         = pflag.Int32("memory", 64, "Default memory in MB for HyperVM when memory limit is not specified for the pod")
	podSandboxImage         = pflag.String("pod-infra-container-image", "", "The image whose network/ipc namespaces containers in each pod will use.")
)

func main() {
	flags.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	// Print out frakti version
	glog.Infof("frakti version: %s\n", fraktiVersion)

	if *cgroupDriver != "cgroupfs" && *cgroupDriver != "systemd" {
		glog.Fatalf("cgroup-driver flag should only be set as 'cgroupfs' or 'systemd'")
	}

	// 1. Initialize hyper runtime and streaming server
	streamingConfig := getStreamingConfig(*streamingServerPort)
	hyperRuntime, streamingServer, err := hyper.NewHyperRuntime(*hyperEndpoint, streamingConfig, *cniNetDir, *cniPluginDir, *rootDir, *defaultCPUNum, *defaultMemoryMB)
	if err != nil {
		glog.Fatalf("Initialize hyper runtime failed: %v", err)
	}

	// 2. Initialize privileged runtime and start its own streaming server
	privilegedRuntime, err := docker.NewPrivilegedRuntimeService(
		*privilegedRuntimeEndpoint,
		getStreamingConfig(privilegedStreamingServerPort),
		*cniNetDir,
		*cniPluginDir,
		*cgroupDriver,
		filepath.Join(*rootDir, "privileged"),
		*podSandboxImage,
	)
	if err != nil && *enablePrivilegedRuntime {
		glog.Fatalf("Initialize privileged runtime failed: %v", err)
	}

	// 3. Initialize unikernel runtime if enabled
	var unikernelRuntime *unikernel.UnikernelRuntime
	if *enableUnikernelRuntime {
		unikernelRuntime, err = unikernel.NewUnikernelRuntimeService(*cniNetDir, *cniPluginDir, *rootDir, *defaultCPUNum, *defaultMemoryMB, *enableUnikernelLog)
		if err != nil {
			glog.Fatalf("Initialize unikernel runtime failed: %v", err)
		}
	}

	// 4. Initialize frakti manager with two runtimes above
	server, err := manager.NewFraktiManager(hyperRuntime, hyperRuntime, streamingServer, privilegedRuntime, privilegedRuntime, unikernelRuntime, unikernelRuntime)
	if err != nil {
		glog.Fatalf("Initialize frakti server failed: %v", err)
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

// getStreamingConfig returns the streaming server configuration to use with in-process CRI shims.
func getStreamingConfig(port string) *streaming.Config {
	config := generateStreamingConfigInternal()
	var (
		addr string
		err  error
	)
	if len(*streamingServerAddress) == 0 {
		addr, err = network.GetLocalIPAddress()
		if err != nil {
			glog.Fatalf("failed to get local IP address of host machine: %v", err)
		}
	} else {
		addr = *streamingServerAddress
	}
	config.Addr = fmt.Sprintf("%s:%s", addr, port)

	glog.V(3).Infof("Streaming server is listening on: %v", config.Addr)
	return config
}
