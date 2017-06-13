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
	"testing"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func TestBuildSandboxName(t *testing.T) {
	var attempt uint32 = 3
	podUID := "12345678"
	podName := "foo"
	podNamespace := "bar"
	sandboxConfig := &kubeapi.PodSandboxConfig{
		Metadata: &kubeapi.PodSandboxMetadata{
			Uid:       podUID,
			Name:      podName,
			Namespace: podNamespace,
			Attempt:   attempt,
		},
	}

	sandboxName := buildSandboxName(sandboxConfig)
	podNameActual, podNamespaceActual, podUIDActual, attempActual, err := parseSandboxName(sandboxName)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if podNamespaceActual != podNamespace {
		t.Errorf("Expected: %q, but got %q", podNamespace, podNamespaceActual)
	}
	if podNameActual != podName {
		t.Errorf("Expected: %q, but got %q", podName, podNameActual)
	}
	if podUIDActual != podUID {
		t.Errorf("Expected: %q, but got %q", podUID, podUIDActual)
	}
	if attempActual != attempt {
		t.Errorf("Expected: %q, but got %q", attempt, attempActual)
	}
}

func TestBuildContainerName(t *testing.T) {
	var attempt uint32 = 3
	podUID := "12345678"
	podName := "foo"
	podNamespace := "bar"
	containerName := "foo1"
	sandboxConfig := &kubeapi.PodSandboxConfig{
		Metadata: &kubeapi.PodSandboxMetadata{
			Uid:       podUID,
			Name:      podName,
			Namespace: podNamespace,
		},
	}
	containerConfig := &kubeapi.ContainerConfig{
		Metadata: &kubeapi.ContainerMetadata{
			Attempt: attempt,
			Name:    containerName,
		},
	}

	generatedContainerName := buildContainerName(sandboxConfig, containerConfig)
	podNameActual, podNamespaceActual, podUIDActual, containerNameActual, attempActual, err := parseContainerName(generatedContainerName)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if containerNameActual != containerName {
		t.Errorf("Expected: %q, but got %q", containerName, containerNameActual)
	}
	if podNamespaceActual != podNamespace {
		t.Errorf("Expected: %q, but got %q", podNamespace, podNamespaceActual)
	}
	if podNameActual != podName {
		t.Errorf("Expected: %q, but got %q", podName, podNameActual)
	}
	if podUIDActual != podUID {
		t.Errorf("Expected: %q, but got %q", podUID, podUIDActual)
	}
	if attempActual != attempt {
		t.Errorf("Expected: %q, but got %q", attempt, attempActual)
	}
}
