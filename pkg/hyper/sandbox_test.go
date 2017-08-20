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

package hyper

import (
	//	"testing"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func makeSandboxConfig(name, namespace, uid string, attempt uint32) *kubeapi.PodSandboxConfig {
	return makeSandboxConfigWithLabelsAndAnnotations(name, namespace, uid, attempt, map[string]string{}, map[string]string{})
}

func makeSandboxConfigWithLabelsAndAnnotations(name, namespace, uid string, attempt uint32, labels, annotations map[string]string) *kubeapi.PodSandboxConfig {
	return &kubeapi.PodSandboxConfig{
		Metadata: &kubeapi.PodSandboxMetadata{
			Name:      name,
			Namespace: namespace,
			Uid:       uid,
			Attempt:   attempt,
		},
		Labels:      labels,
		Annotations: annotations,
	}
}
