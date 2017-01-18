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
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/frakti/test/e2e/framework"
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
	defaultLogContext           string = "hello world"
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
	podName := "PodSandbox-for-create-container-" + framework.NewUUID()
	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(&podName),
	}
	return createPodSandboxOrFail(c, podConfig), podConfig
}

//
func createPodSandboxWithLogDirectory(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	By("create a PodSandbox with log directory")
	podName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	dir := fmt.Sprintf("/var/log/pods/%s/", podName)
	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata:     buildPodSandboxMetadata(&podName),
		LogDirectory: &dir,
	}
	return createPodSandboxOrFail(c, podConfig), podConfig
}

// createPodSandboxOrFail creates a PodSandbox and fails if it gets error.
func createPodSandboxOrFail(c internalapi.RuntimeService, podConfig *runtimeapi.PodSandboxConfig) string {
	podID, err := c.RunPodSandbox(podConfig)
	framework.ExpectNoError(err, "Failed to create PodSandbox: %v", err)
	framework.Logf("Created PodSandbox %s\n", podID)
	return podID
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
	framework.ExpectNoError(err, "Failed to list containers %s status: %v", containerID, err)
	return containers
}

// createContainer creates a container with the prefix of containerName.
func createContainer(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, error) {
	By("create a container with name")
	containerName := prefix + framework.NewUUID()
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
	containerName := prefix + framework.NewUUID()
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

// createLogContainer creates a container with log and the prefix of containerName.
func createLogContainer(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, string, error) {
	By("create a container with log and name")
	containerName := prefix + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(&containerName),
		Image:    &runtimeapi.ImageSpec{Image: &defaultContainerImage},
		Command:  []string{"echo", "hello world"},
		LogPath:  &path,
	}
	containerID, err := c.CreateContainer(podID, containerConfig, podConfig)
	return *containerConfig.LogPath, containerID, err
}

// createContainerOrFail creates a container with the prefix of containerName and fails if it gets error.
func createContainerOrFail(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID, err := createContainer(c, prefix, podID, podConfig)
	framework.ExpectNoError(err, "Failed to create container: %v", err)
	framework.Logf("Created container %s\n", containerID)
	return containerID
}

// createVolContainerOrFail creates a container with volume and the prefix of containerName and fails if it gets error.
func createVolContainerOrFail(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig, hostPath, flagFile string) string {
	containerID, err := createVolContainer(c, prefix, podID, podConfig, hostPath, flagFile)
	framework.ExpectNoError(err, "Failed to create container: %v", err)
	framework.Logf("Created container %s\n", containerID)
	return containerID
}

// createLogContainerOrFail creates a container with log and the prefix of containerName and fails if it gets error.
func createLogContainerOrFail(c internalapi.RuntimeService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, string) {
	logPath, containerID, err := createLogContainer(c, prefix, podID, podConfig)
	framework.ExpectNoError(err, "Failed to create container: %v", err)
	framework.Logf("Created container %s\n", containerID)
	return logPath, containerID
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
	framework.ExpectNoError(err, "Failed to start container: %v", err)
	framework.Logf("Start container %s\n", containerID)
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
	framework.ExpectNoError(err, "Failed to stop container: %v", err)
	framework.Logf("Stop container %s\n", containerID)
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
	framework.ExpectNoError(err, "Failed to get container %s status: %v", containerID, err)
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
	framework.ExpectNoError(err, "Failed to remove container: %v", err)
	framework.Logf("Removed container %s\n", containerID)
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

// verifyLogContents verifies the contents of container log.
func verifyLogContents(podConfig *runtimeapi.PodSandboxConfig, logPath string, expectedContext string) {
	path := *podConfig.LogDirectory + logPath
	f, err := os.Open(path)
	framework.ExpectNoError(err, "Failed to open log file: %v", err)
	framework.Logf("Open log file %s\n", path)
	defer f.Close()

	logContext, err := ioutil.ReadAll(f)
	framework.ExpectNoError(err, "Failed to read log file: %v", err)
	framework.Logf("Log file context is %s\n", logContext)

	Expect(string(logContext)).To(ContainSubstring(expectedContext), "Log context should be %s", expectedContext)
}
