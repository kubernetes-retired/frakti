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

package hyper

import (
	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	// default resources while the resource limit of kubelet pod is not specified.
	defaultCPUNumber         = 1
	defaultMemoryinMegabytes = 64
)

// buildUserPod builds hyperd's UserPod based kubelet PodSandboxConfig.
// TODO: support pod-level portmapping (depends on hyperd).
// TODO: support resource limits via pod-level cgroups, ref https://github.com/kubernetes/kubernetes/issues/27097.
func buildUserPod(config *kubeapi.PodSandboxConfig) (*types.UserPod, error) {
	spec := &types.UserPod{
		Id:       buildSandboxName(config),
		Hostname: config.GetHostname(),
		Labels:   buildLabelsWithAnnotations(config.Labels, config.Annotations),
		Resource: &types.UserResource{
			Vcpu:   int32(defaultCPUNumber),
			Memory: int32(defaultMemoryinMegabytes),
		},
	}

	// Make dns
	if config.DnsOptions != nil {
		// TODO: support DNS search domains in upstream hyperd
		spec.Dns = config.DnsOptions.Servers
	}

	return spec, nil
}
