/*
Copyright 2018 The Kubernetes Authors.

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

package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/containerd/containerd/runtime"
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/pkg/annotations"
	errors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CreateSandbox creates a kata-runtime sandbox
func CreateSandbox(id string) (*vc.Sandbox, error) {
	envs := []vc.EnvVar{
		{
			Var:   "PATH",
			Value: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		{
			Var:   "PATH",
			Value: "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
	}

	configFile := "/run/containerd/io.containerd.runtime.v1.kata-runtime/k8s.io/" + id + "/config.json"
	configJ, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Print(err)
	}
	str := string(configJ)
	str = strings.Replace(str, "bounding", "Bounding", -1)
	str = strings.Replace(str, "effective", "Effective", -1)
	str = strings.Replace(str, "inheritable", "Inheritable", -1)
	str = strings.Replace(str, "permitted", "Permitted", -1)
	str = strings.Replace(str, "true", "true,\"Ambient\":null", -1)

	cmd := vc.Cmd{
		// Args:    strings.Split("sh", " "),
		Envs:    envs,
		WorkDir: "/",
		Capabilities: vc.LinuxCapabilities{
			Bounding: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER", "CAP_MKNOD",
				"CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID", "CAP_SETFCAP", "CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL", "CAP_AUDIT_WRITE",
			},
			Effective: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER", "CAP_MKNOD",
				"CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID", "CAP_SETFCAP", "CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL", "CAP_AUDIT_WRITE",
			},
			Inheritable: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER", "CAP_MKNOD",
				"CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID", "CAP_SETFCAP", "CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL", "CAP_AUDIT_WRITE",
			},
			Permitted: []string{
				"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FSETID", "CAP_FOWNER", "CAP_MKNOD",
				"CAP_NET_RAW", "CAP_SETGID", "CAP_SETUID", "CAP_SETFCAP", "CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE", "CAP_SYS_CHROOT", "CAP_KILL", "CAP_AUDIT_WRITE",
			},
		},
		User:            "0",
		PrimaryGroup:    "0",
		NoNewPrivileges: true,
	}

	// Define the container command and bundle.
	container := vc.ContainerConfig{
		ID:     id,
		RootFs: "/run/containerd/io.containerd.runtime.v1.kata-runtime/k8s.io/" + id + "/rootfs",
		Cmd:    cmd,
		Annotations: map[string]string{
			annotations.ConfigJSONKey:    str,
			annotations.BundlePathKey:    "/run/containerd/io.containerd.runtime.v1.kata-runtime/k8s.io/" + id,
			annotations.ContainerTypeKey: string(vc.PodSandbox),
		},
		Mounts: []vc.Mount{
			{
				Source:      "proc",
				Destination: "/proc",
				Type:        "proc",
				Options:     nil,
				ReadOnly:    false,
			},
			{
				Source:      "tmpfs",
				Destination: "/dev",
				Type:        "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
				ReadOnly:    false,
			},
			{
				Source:      "devpts",
				Destination: "/dev/pts",
				Type:        "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
				ReadOnly:    false,
			},
			{
				Source:      "shm",
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
				ReadOnly:    false,
			},
			{
				Source:      "mqueue",
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
				ReadOnly:    false,
			},
			{
				Source:      "sysfs",
				Destination: "/sys",
				Type:        "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
				ReadOnly:    false,
			},
			{
				Source:      "tmpfs",
				Destination: "/run",
				Type:        "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
				ReadOnly:    false,
			},
		},
	}

	// Sets the hypervisor configuration.
	hypervisorConfig := vc.HypervisorConfig{
		KernelParams: []vc.Param{
			{
				Key:   "agent.log",
				Value: "debug",
			},
			{
				Key:   "qemu.cmdline",
				Value: "-D <logfile>",
			},
			{
				Key:   "ip",
				Value: "::::::" + id + "::off::",
			},
		},
		KernelPath:     "/usr/share/kata-containers/vmlinuz.container",
		InitrdPath:     "/usr/share/kata-containers/kata-containers-initrd.img",
		HypervisorPath: "/usr/bin/qemu-lite-system-x86_64",

		BlockDeviceDriver: "virtio-scsi",

		HypervisorMachineType: "pc",

		DefaultVCPUs:    uint32(1),
		DefaultMaxVCPUs: uint32(4),

		DefaultMemSz: uint32(128),

		DefaultBridges: uint32(1),

		Mlock:   true,
		Msize9p: uint32(8192),

		Debug: true,
	}

	// Use KataAgent for the agent.
	agConfig := vc.KataAgentConfig{
		LongLiveConn: true,
	}

	// VM resources
	vmConfig := vc.Resources{
		Memory: uint(128),
	}

	// The sandbox configuration:
	// - One container
	// - Hypervisor is QEMU
	// - Agent is KataContainers
	sandboxConfig := vc.SandboxConfig{
		ID: id,

		VMConfig: vmConfig,

		HypervisorType:   vc.QemuHypervisor,
		HypervisorConfig: hypervisorConfig,

		AgentType:   vc.KataContainersAgent,
		AgentConfig: agConfig,

		ProxyType:   vc.KataBuiltInProxyType,
		ProxyConfig: vc.ProxyConfig{},

		ShimType:   vc.KataBuiltInShimType,
		ShimConfig: vc.ShimConfig{},

		NetworkModel: vc.CNMNetworkModel,
		NetworkConfig: vc.NetworkConfig{
			NumInterfaces:     1,
			InterworkingModel: 2,
		},

		Containers: []vc.ContainerConfig{container},

		Annotations: map[string]string{
			annotations.BundlePathKey: "/run/containerd/io.containerd.runtime.v1.kata-runtime/k8s.io/" + id,
		},
	}

	sandbox, err := vc.CreateSandbox(sandboxConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not create sandbox")
	}

	logrus.FieldLogger(logrus.New()).WithFields(logrus.Fields{
		"sandbox": sandbox,
	}).Info("Run Sandbox Successfully")

	return sandbox.(*vc.Sandbox), err
}

// StartSandbox starts a kata-runtime sandbox
func StartSandbox(id string) (*vc.Sandbox, error) {
	sandbox, err := vc.StartSandbox(id)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not start sandbox")
	}

	return sandbox.(*vc.Sandbox), err
}

// StopSandbox stops a kata-runtime sandbox
func StopSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("stop not implemented")
}

// DeleteSandbox deletes a kata-runtime sandbox
func DeleteSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("delete not implemented")
}
