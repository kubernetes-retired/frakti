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
	"k8s.io/frakti/test/e2e/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Test CNI", func() {
	f := framework.NewDefaultFramework("test")

	var c internalapi.RuntimeService

	BeforeEach(func() {
		c = f.Client.FraktiRuntimeService
	})

	It("test create simple pods with cni", func() {
		name := "create-simple-sandbox-" + framework.NewUUID()
		By("create a podSandbox with name")
		metadata := buildPodSandboxMetadata(name)
		config := &kubeapi.PodSandboxConfig{
			Metadata: metadata,
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
		Expect(framework.CniWork(status.GetNetwork())).To(BeTrue(), "cni not work")
	})

})
