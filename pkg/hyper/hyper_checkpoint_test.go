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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckpoint(t *testing.T) {
	testPath := os.TempDir()
	persistentCheckpointHandler, err := NewPersistentCheckpointHandler(testPath)
	assert.NoError(t, err)
	podName, namespaceName, podSandbox := "foo", "bar", "sandbox"
	for i := 0; i < 3; i++ {
		pod := fmt.Sprintf("%s%d", podName, i)
		namespace := fmt.Sprintf("%s%d", namespaceName, i)
		podSandboxID := fmt.Sprintf("%s%d", podSandbox, i)
		podSandboxCheckpoint := NewPodSandboxCheckpoint(namespace, pod)
		//Test CreateCheckpoint
		err = persistentCheckpointHandler.CreateCheckpoint(podSandboxID, podSandboxCheckpoint)
		assert.NoError(t, err)
	}
	//Test ListCheckpoints
	checkpointsList := persistentCheckpointHandler.ListCheckpoints()
	expected := []string{}
	for i := 0; i < 3; i++ {
		podSandboxID := fmt.Sprintf("%s%d", podSandbox, i)
		expected = append(expected, podSandboxID)
	}
	assert.Len(t, checkpointsList, 3)
	assert.Equal(t, expected, checkpointsList)
	//Test Remove Checkpints
	podSandboxID := fmt.Sprintf("%s%d", podSandbox, 0)
	err = persistentCheckpointHandler.RemoveCheckpoint(podSandboxID)
	assert.NoError(t, err)
	checkpointsList = persistentCheckpointHandler.ListCheckpoints()
	assert.Len(t, checkpointsList, 2)
	//Test GetCheckpoint
	podSandboxID = fmt.Sprintf("%s%d", podSandbox, 1)
	pod := fmt.Sprintf("%s%d", podName, 1)
	namespace := fmt.Sprintf("%s%d", namespaceName, 1)
	checkpoint, err := persistentCheckpointHandler.GetCheckpoint(podSandboxID)
	assert.NoError(t, err)
	expectedCheckpoint := &PodSandboxCheckpoint{
		Version:   schemaVersion,
		Name:      pod,
		Namespace: namespace,
		Data:      &CheckpointData{},
	}
	assert.Equal(t, expectedCheckpoint, checkpoint)
	defer cleanUpTestPath(t, testPath+"/"+sandboxCheckpointDir)
}
