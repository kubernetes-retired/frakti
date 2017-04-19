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
	"time"

	"k8s.io/frakti/test/e2e/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Test streaming in container", func() {
	f := framework.NewDefaultFramework("test")

	var (
		runtimeClient internalapi.RuntimeService
		imageClient   internalapi.ImageManagerService
	)

	BeforeEach(func() {
		runtimeClient = f.Client.FraktiRuntimeService
		imageClient = f.Client.FraktiImageService
	})

	It("test exec a command in container synchronously and successfully", func() {
		podID, cID := startLongRunningContainer(runtimeClient, imageClient)
		defer func(podId string) {
			By("delete pod sandbox")
			runtimeClient.RemovePodSandbox(podID)
		}(podID)

		By("exec command in container synchronously")
		magicWords := "blablabla"
		stdout, stderr, err := runtimeClient.ExecSync(cID, []string{"echo", magicWords}, 0)
		framework.ExpectNoError(err, "Failed to exec cmd in container: %v", err)
		framework.Logf("stdout: %q, stderr: %q", string(stdout), string(stderr))
		Expect(len(stderr)).To(Equal(0), "stderr should not have content")
		Expect(string(stdout)).To(BeIdenticalTo(magicWords+"\n"), "stdout should be same as defined")
	})

	It("test exec a command in container synchronously and failed", func() {
		podID, cID := startLongRunningContainer(runtimeClient, imageClient)
		defer func(podID string) {
			By("delete pod sandbox")
			runtimeClient.RemovePodSandbox(podID)
		}(podID)

		By("exec command in container synchronously")
		magicCmd := "blablabla"
		stdout, stderr, err := runtimeClient.ExecSync(cID, []string{magicCmd}, 0)
		Expect(err).NotTo(Equal(nil), "Exec non-exist cmd should failed")
		framework.Logf("stdout: %q, stderr: %q", string(stdout), string(stderr))
		Expect(len(stderr)).NotTo(Equal(0), "stderr should have content")
	})

	It("test get a exec url", func() {
		podID, cID := startLongRunningContainer(runtimeClient, imageClient)
		defer func(podID string) {
			By("delete pod sandbox")
			runtimeClient.RemovePodSandbox(podID)
		}(podID)

		By("prepare exec command url in container")
		magicCmd := []string{"blablabla"}
		execReq := &kubeapi.ExecRequest{
			ContainerId: cID,
			Cmd:         magicCmd,
		}
		resp, err := runtimeClient.Exec(execReq)
		framework.ExpectNoError(err, "Failed to get exec url in container: %v", err)
		framework.Logf("ExecUrl: %q", resp.Url)
		Expect(len(resp.Url)).NotTo(Equal(0), "exec url should not be null")
	})

	It("test get a attach url", func() {
		podID, cID := startLongRunningContainer(runtimeClient, imageClient)
		defer func(podID string) {
			By("delete pod sandbox")
			runtimeClient.RemovePodSandbox(podID)
		}(podID)

		By("prepare attach command url in container")
		stdin := true
		attachReq := &kubeapi.AttachRequest{
			ContainerId: cID,
			Stdin:       stdin,
		}
		resp, err := runtimeClient.Attach(attachReq)
		framework.ExpectNoError(err, "Failed to get attach url in container: %v", err)
		framework.Logf("AttachUrl: %q", resp.Url)
		Expect(len(resp.Url)).NotTo(Equal(0), "attach url should not be null")
	})
})

func startLongRunningContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService) (podId, containerId string) {
	podName := "simple-sandbox-" + framework.NewUUID()
	By("create a podSandbox")
	podConfig := &kubeapi.PodSandboxConfig{
		Metadata: &kubeapi.PodSandboxMetadata{
			Name: podName,
		},
	}
	podId, err := rc.RunPodSandbox(podConfig)
	framework.ExpectNoError(err, "Failed to create podsandbox: %v", err)

	By("pull necessary image")
	imageSpec := &kubeapi.ImageSpec{
		Image: latestTestImageRef,
	}
	_, err = ic.PullImage(imageSpec, nil)
	framework.ExpectNoError(err, "Failed to pull image: %v", err)

	By("create container in pod")
	containerName := "simple-container-" + framework.NewUUID()
	containerConfig := &kubeapi.ContainerConfig{
		Metadata: &kubeapi.ContainerMetadata{
			Name: containerName,
		},
		Image:   imageSpec,
		Command: []string{"sh", "-c", "top"},
	}
	containerId, err = rc.CreateContainer(podId, containerConfig, podConfig)
	framework.ExpectNoError(err, "Failed to create container: %v", err)

	By("start container")
	err = rc.StartContainer(containerId)
	framework.ExpectNoError(err, "Failed to start container: %v", err)

	// sleep 2s to make sure container start is ready, workaround for random failed in travis
	// TODO: remove this
	time.Sleep(2 * time.Second)

	return podId, containerId
}
