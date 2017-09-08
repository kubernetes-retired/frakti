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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func newTestRuntimeWithCheckpoint() (*Runtime, *fakeClientInterface, CheckpointHandler) {
	publicClient := newFakeClientInterface(nil)
	memStore := &MemStore{
		mem: make(map[string][]byte),
	}
	checkpointHandler := &PersistentCheckpointHandler{
		store: memStore,
	}
	client := &Client{
		client: publicClient,
	}
	return &Runtime{
		client:            client,
		checkpointHandler: checkpointHandler,
	}, publicClient, checkpointHandler
}

func TestListPodSandbox(t *testing.T) {
	r, fakeClient, checkpointHandler := newTestRuntimeWithCheckpoint()
	podId, checkPoint := "p", "c"
	podName, namespace := "foo", "bar"
	containerName := "sidecar"
	pods := []*FakePod{}
	podIDs := []string{}
	//Create runnning pods for test
	for i := 0; i < 3; i++ {
		podID := fmt.Sprintf("%s%s%d", podId, "*", i)
		container := fmt.Sprintf("%s%d", containerName, i)
		podN := fmt.Sprintf("%s%d", podName, i)
		s := []string{"k8s", container, podN, namespace, podID, "1"}
		podname := strings.Join(s, "_")
		pod := &FakePod{
			PodID:   podID,
			PodName: podname,
			Status:  "running",
		}
		pods = append(pods, pod)
		checkPointName := fmt.Sprintf("%s%s%d", checkPoint, "*", i)
		checkpoint := &PodSandboxCheckpoint{
			Name: checkPointName,
		}
		err := checkpointHandler.CreateCheckpoint(podID, checkpoint)
		assert.NoError(t, err)
		podIDs = append(podIDs, podID)
	}
	fakeClient.SetFakePod(pods)
	//Test PodSandboxStatus
	for i := 0; i < 3; i++ {
		podID := fmt.Sprintf("%s%s%d", podId, "*", i)
		podN := fmt.Sprintf("%s%d", podName, i)
		podStatus, err := r.PodSandboxStatus(podID)
		metadata := &kubeapi.PodSandboxMetadata{
			Name:      podN,
			Uid:       podID,
			Namespace: namespace,
			Attempt:   uint32(0),
		}
		network := &kubeapi.PodSandboxNetworkStatus{Ip: ""}
		expected := &kubeapi.PodSandboxStatus{
			Id:          podID,
			State:       kubeapi.PodSandboxState_SANDBOX_READY,
			Metadata:    metadata,
			Network:     network,
			Annotations: make(map[string]string),
		}
		assert.Equal(t, expected, podStatus)
		assert.NoError(t, err)
	}
	//Test ListPodSandbox
	podStateValue := kubeapi.PodSandboxStateValue{
		State: kubeapi.PodSandboxState_SANDBOX_READY,
	}
	filter := &kubeapi.PodSandboxFilter{
		State: &podStateValue,
	}
	podsList, err := r.ListPodSandbox(filter)
	assert.NoError(t, err)
	assert.Len(t, podsList, 3)
	assert.Len(t, fakeClient.podInfoMap, 3)
	expected := []*kubeapi.PodSandbox{}
	for i := 0; i < 3; i++ {
		podID := fmt.Sprintf("%s%s%d", podId, "*", i)
		podN := fmt.Sprintf("%s%d", podName, i)

		metadata := &kubeapi.PodSandboxMetadata{
			Name:      podN,
			Uid:       podID,
			Namespace: namespace,
			Attempt:   uint32(0),
		}

		podSandbox := kubeapi.PodSandbox{
			Id:       "",
			Metadata: metadata,
			State:    kubeapi.PodSandboxState_SANDBOX_READY,
		}
		expected = append(expected, &podSandbox)
		assert.Contains(t, podsList, &podSandbox)
	}
	assert.Len(t, expected, 3)
	//Test RemovePodSandbox
	err = r.RemovePodSandbox(podIDs[0])
	assert.NoError(t, err)
	podsList, err = r.ListPodSandbox(filter)
	assert.NoError(t, err)
	assert.Len(t, podsList, 2)
	assert.Len(t, fakeClient.podInfoMap, 2)
}
