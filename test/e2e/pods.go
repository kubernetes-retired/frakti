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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/frakti/test/e2e/framework"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var _ = framework.KubeDescribe("Test PodSandbox", func() {
	f := framework.NewDefaultFramework("test")

	var c *framework.FraktiClient

	BeforeEach(func() {
		c = f.Client
	})

	It("test create simple podsandbox", func() {
		name := "create-simple-sandbox-" + framework.NewUUID()
		By("create a podSandbox with name")
		config := &kubeapi.PodSandboxConfig{
			Metadata: &kubeapi.PodSandboxMetadata{
				Name: &name,
			},
		}
		podId, err := c.RunPodSandbox(config)
		framework.ExpectNoError(err, "Failed to create podsandbox: %v", err)
		framework.Logf("Created Podsanbox %s\n", podId)
		defer func() {
			By("delete pod sandbox")
			c.RemovePodSandbox(podId)
		}()
		By("get podSandbox status")
		status, err := c.PodSandboxStatus(podId)
		framework.ExpectNoError(err, "Failed to get podsandbox %s status: %v", podId, err)
		Expect(framework.PodReady(status)).To(BeTrue(), "pod state shoud be ready")
	})

	It("test stop PodSandbox", func() {
		name := "create-simple-sandbox-for-stop" + framework.NewUUID()

		By("create a podSandbox with name")
		config := &kubeapi.PodSandboxConfig{
			Metadata: &kubeapi.PodSandboxMetadata{
				Name: &name,
			},
		}
		podId, err := c.RunPodSandbox(config)
		framework.ExpectNoError(err, "Failed to create podsandbox: %v", err)
		framework.Logf("Created Podsanbox %s\n", podId)
		defer func() {
			By("delete pod sandbox")
			c.RemovePodSandbox(podId)
		}()

		By("get podSandbox status")
		status, err := c.PodSandboxStatus(podId)
		framework.ExpectNoError(err, "Failed to get podsandbox %s status: %v", podId, err)
		Expect(framework.PodReady(status)).To(BeTrue(), "pod state should be ready")

		By("stop podSandbox with poId")
		err = c.StopPodSandbox(podId)
		framework.ExpectNoError(err, "Failed to stop podsandbox: %v", err)
		framework.Logf("Stoped Podsanbox %s\n", podId)

		By("get podSandbox status")
		status, err = c.PodSandboxStatus(podId)
		framework.ExpectNoError(err, "Failed to get podsandbox %s status: %v", podId, err)
		Expect(framework.PodReady(status)).To(BeFalse(), "pod state should be not ready")
	})

	It("test remove podsandbox", func() {
		name := "create-simple-sandbox-for-remove" + framework.NewUUID()
		By("create a podSandbox with name")
		config := &kubeapi.PodSandboxConfig{
			Metadata: &kubeapi.PodSandboxMetadata{
				Name: &name,
			},
		}
		podId, err := c.RunPodSandbox(config)
		framework.ExpectNoError(err, "Failed to create podsandbox: %v", err)
		framework.Logf("Created Podsanbox %s\n", podId)

		By("get podSandbox status")
		status, err := c.PodSandboxStatus(podId)
		framework.ExpectNoError(err, "Failed to get podsandbox %s status: %v", podId, err)
		Expect(framework.PodReady(status)).To(BeTrue(), "pod state should be ready")

		By("remove podSandbox with podId")
		err = c.RemovePodSandbox(podId)
		framework.ExpectNoError(err, "Failed to remove podsandbox: %v", err)
		framework.Logf("Removed Podsanbox %s\n", podId)

		By("list podSandbox with podId")
		filter := &kubeapi.PodSandboxFilter{
			Id: &podId,
		}
		podsandboxs, err := c.ListPodSandbox(filter)
		framework.ExpectNoError(err, "Failed to list podsandbox %s status: %v", podId, err)
		Expect(framework.PodFound(podsandboxs, podId)).To(BeFalse(), "pod should be removed")
	})

	It("test list podsandbox", func() {
		name := "create-simple-sandbox-for-list" + framework.NewUUID()
		By("create a podSandbox with name")
		config := &kubeapi.PodSandboxConfig{
			Metadata: &kubeapi.PodSandboxMetadata{
				Name: &name,
			},
		}
		podId, err := c.RunPodSandbox(config)
		framework.ExpectNoError(err, "Failed to create podsandbox: %v", err)
		framework.Logf("Created Podsanbox %s\n", podId)
		defer func() {
			By("delete pod sandbox")
			c.RemovePodSandbox(podId)
		}()
		By("get podSandbox status")
		status, err := c.PodSandboxStatus(podId)
		framework.ExpectNoError(err, "Failed to get podsandbox %s status: %v", podId, err)
		Expect(framework.PodReady(status)).To(BeTrue(), "pod state should be ready")

		By("list podSandbox with podId")
		filter := &kubeapi.PodSandboxFilter{
			Id: &podId,
		}
		podsandboxs, err := c.ListPodSandbox(filter)
		framework.ExpectNoError(err, "Failed to list podsandbox %s status: %v", podId, err)
		Expect(framework.PodFound(podsandboxs, podId)).To(BeTrue(), "pod should be listed")
	})
})
