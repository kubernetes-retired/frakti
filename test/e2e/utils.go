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

package e2e

import (
	e2eframework "k8s.io/frakti/test/e2e/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	defaultUid                  string = "e2e-cri-uid"
	defaultNamespace            string = "e2e-cri-namespace"
	defaultAttempt              uint32 = 2
	defaultContainerImage       string = "busybox:latest"
	defaultStopContainerTimeout int64  = 60
)

// buildPodSandboxMetadata builds default PodSandboxMetadata with podSandboxName.
func buildPodSandboxMetadata(podSandboxName *string) *runtimeapi.PodSandboxMetadata {
	return &runtimeapi.PodSandboxMetadata{
		Name:      podSandboxName,
		Uid:       &defaultUid,
		Namespace: &defaultNamespace,
		Attempt:   &defaultAttempt,
	}
}

// buildContainerMetadata builds default PodSandboxMetadata with containerName.
func buildContainerMetadata(containerName *string) *runtimeapi.ContainerMetadata {
	return &runtimeapi.ContainerMetadata{
		Name:    containerName,
		Attempt: &defaultAttempt,
	}
}

// createPodSandboxForContainer creates a PodSandbox for creating containers.
func createPodSandboxForContainer(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	By("create a PodSandbox for creating containers")
	podName := "PodSandbox-for-create-container-" + e2eframework.NewUUID()
	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(&podName),
	}
	podID, err := c.RunPodSandbox(podConfig)
	e2eframework.ExpectNoError(err, "Failed to create PodSandbox: %v", err)
	e2eframework.Logf("Created PodSandbox %s\n", podID)
	return podID, podConfig
}

// listPodSanboxforID lists PodSandbox for podID.
func listPodSanboxForID(c internalapi.RuntimeService, podID string) ([]*runtimeapi.PodSandbox, error) {
	By("list PodSandbox for podID")
	filter := &runtimeapi.PodSandboxFilter{
		Id: &podID,
	}
	return c.ListPodSandbox(filter)
}

// listContainerforID lists container for podID.
func listContainerForID(c internalapi.RuntimeService, containerID string) ([]*runtimeapi.Container, error) {
	By("list containers for containerID")
	filter := &runtimeapi.ContainerFilter{
		Id: &containerID,
	}
	return c.ListContainers(filter)
}

// listContainerforID lists container for podID and fails if it gets error.
func listContainerForIDOrFail(c internalapi.RuntimeService, containerID string) []*runtimeapi.Container {
	containers, err := listContainerForID(c, containerID)
	e2eframework.ExpectNoError(err, "Failed to list containers %s status: %v", containerID, err)
	return containers
}

// createContainer creates a container with the prefix of containerName.
func createContainer(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, error) {
	By("create a container with name")
	containerName := prefix + e2eframework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(&containerName),
		Image:    &runtimeapi.ImageSpec{Image: &defaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
	}
	return c.CreateContainer(podID, containerConfig, podConfig)
}

// createVolContainer creates a container with volume and the prefix of containerName.
func createVolContainer(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig, volPath, flagFile string) (string, error) {
	By("create a container with volume and name")
	containerName := prefix + e2eframework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(&containerName),
		Image:    &runtimeapi.ImageSpec{Image: &defaultContainerImage},
		// mount host path to the same directory in container, and check if flag file exists
		Command: []string{"sh", "-c", "while [ -f " + volPath + "/" + flagFile + " ]; do sleep 1; done;"},
		Mounts: []*runtimeapi.Mount{
			{
				HostPath:      &volPath,
				ContainerPath: &volPath,
			},
		},
	}
	return c.CreateContainer(podID, containerConfig, podConfig)
}

// createContainerOrFail creates a container with the prefix of containerName and fails if it gets error.
func createContainerOrFail(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID, err := createContainer(c, prefix, podID, podConfig)
	e2eframework.ExpectNoError(err, "Failed to create container: %v", err)
	e2eframework.Logf("Created container %s\n", containerID)
	return containerID
}

// createVolContainerOrFail creates a container with volume and the prefix of containerName and fails if it gets error.
func createVolContainerOrFail(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig, hostPath, flagFile string) string {
	containerID, err := createVolContainer(c, prefix, podID, podConfig, hostPath, flagFile)
	e2eframework.ExpectNoError(err, "Failed to create container: %v", err)
	e2eframework.Logf("Created container %s\n", containerID)
	return containerID
}

// testCreateContainer creates a container in the pod which ID is podID and make sure it be ready.
func testCreateContainer(c internalapi.RuntimeService, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID := createContainerOrFail(c, "container-for-create-test-", podID, podConfig)
	verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_CREATED, "created")
	return containerID
}

// startContainer start the container for containerID.
func startContainer(c internalapi.RuntimeService, containerID string) error {
	By("start container")
	return c.StartContainer(containerID)
}

// startcontainerOrFail starts the container for containerID and fails if it gets error.
func startContainerOrFail(c internalapi.RuntimeService, containerID string) {
	err := startContainer(c, containerID)
	e2eframework.ExpectNoError(err, "Failed to start container: %v", err)
	e2eframework.Logf("Start container %s\n", containerID)
}

// testStartContainer starts the container for containerID and make sure it be running.
func testStartContainer(c internalapi.RuntimeService, containerID string) {
	startContainerOrFail(c, containerID)
	verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_RUNNING, "running")
}

// stopContainer stops the container for containerID.
func stopContainer(c internalapi.RuntimeService, containerID string, timeout int64) error {
	By("stop container")
	return c.StopContainer(containerID, timeout)
}

// stopContainerOrFail stops the container for containerID and fails if it gets error.
func stopContainerOrFail(c internalapi.RuntimeService, containerID string, timeout int64) {
	err := stopContainer(c, containerID, timeout)
	e2eframework.ExpectNoError(err, "Failed to stop container: %v", err)
	e2eframework.Logf("Stop container %s\n", containerID)
}

// testStopContainer stops the container for containerID and make sure it be exited.
func testStopContainer(c internalapi.RuntimeService, containerID string) {
	stopContainerOrFail(c, containerID, defaultStopContainerTimeout)
	verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_EXITED, "exited")
}

// verifyContainerStatus verifies whether status for given containerID matches.
func verifyContainerStatus(c internalapi.RuntimeService, containerID string, expectedStatus runtimeapi.ContainerState, stateName string) {
	status := getContainerStatusOrFail(c, containerID)
	Expect(*status.State).To(Equal(expectedStatus), "Container state should be %s", stateName)
}

// getPodSandboxStatusOrFail gets ContainerState for containerID and fails if it gets error.
func getContainerStatusOrFail(c internalapi.RuntimeService, containerID string) *runtimeapi.ContainerStatus {
	status, err := getContainerStatus(c, containerID)
	e2eframework.ExpectNoError(err, "Failed to get container %s status: %v", containerID, err)
	return status
}

// removePodSandbox removes the container for containerID.
func removeContainer(c internalapi.RuntimeService, containerID string) error {
	By("remove container for containerID")
	return c.RemoveContainer(containerID)
}

// removeContainerOrFail removes the container for containerID and fails if it gets error.
func removeContainerOrFail(c internalapi.RuntimeService, containerID string) {
	err := removeContainer(c, containerID)
	e2eframework.ExpectNoError(err, "Failed to remove container: %v", err)
	e2eframework.Logf("Removed container %s\n", containerID)
}

// getContainerStatus gets ContainerState for containerID.
func getContainerStatus(c internalapi.RuntimeService, containerID string) (*runtimeapi.ContainerStatus, error) {
	By("get container status")
	return c.ContainerStatus(containerID)
}

// containerFound returns whether containers is found.
func containerFound(containers []*runtimeapi.Container, containerID string) bool {
	if len(containers) == 1 && containers[0].GetId() == containerID {
		return true

	}
	return false
}
