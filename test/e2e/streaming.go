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
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/frakti/test/e2e/framework"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	remotecommandconsts "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	nginxContainerPort int32  = 80
	nginxHostPort      int32  = 8000
	nginxImage         string = "nginx"
)

var _ = framework.KubeDescribe("Streaming", func() {
	f := framework.NewDefaultFramework("test")

	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	BeforeEach(func() {
		rc = f.Client.FraktiRuntimeService
		ic = f.Client.FraktiImageService
	})

	Context("runtime should support streaming interfaces", func() {
		var podID string
		var podConfig *kubeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = createPodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support exec [Conformance]", func() {
			By("start a default container")
			containerID := startDefaultContainer(rc, ic, podID, podConfig)
			req := createDefaultExec(rc, containerID)
			By("check the output of exec")
			checkExec(rc, req)
		})

		It("runtime should support attach [Conformance]", func() {
			By("start a default container")
			containerID := startDefaultContainer(rc, ic, podID, podConfig)
			req := createDefaultAttach(rc, containerID)
			By("check the output of attach")
			checkAttach(rc, req)
		})

		It("runtime should support portforward [Conformance]", func() {
			By("create a PodSandbox with host port and container port port mapping")
			var podConfig *kubeapi.PodSandboxConfig
			portMappings := []*kubeapi.PortMapping{
				{
					ContainerPort: nginxContainerPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings)
			_ = startNginxContainer(rc, ic, podID, podConfig)
			req := createDefaultPortForward(rc, podID)

			By("check the output of attach")
			checkPortForward(rc, req)
		})
	})
})

func createDefaultExec(c internalapi.RuntimeService, containerID string) string {
	By("exec default command in container: " + containerID)
	req := &kubeapi.ExecRequest{
		ContainerId: containerID,
		Cmd:         []string{"echo", "hello"},
	}

	resp, err := c.Exec(req)
	framework.ExpectNoError(err, "failed to exec in container %q", containerID)
	framework.Logf("Get exec url: " + resp.Url)
	return resp.Url
}

func checkExec(c internalapi.RuntimeService, execServerURL string) {
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	url := parseURL(execServerURL)
	e, err := remotecommand.NewExecutor(&rest.Config{}, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", execServerURL)

	err = e.Stream(remotecommand.StreamOptions{
		SupportedProtocols: remotecommandconsts.SupportedStreamingProtocols,
		Stdout:             localOut,
		Stderr:             localErr,
		Tty:                false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", execServerURL)

	Expect(localOut.String()).To(Equal("hello\n"), "The stdout of exec should be hello")
	Expect(localErr.String()).To(BeEmpty(), "The stderr of exec should be empty")
	framework.Logf("Check exec url %q succeed", execServerURL)
}

func parseURL(serverURL string) *url.URL {
	url, err := url.Parse(serverURL)
	framework.ExpectNoError(err, "failed to parse url:  %q", serverURL)
	Expect(url.Host).NotTo(BeEmpty(), "The host of url should not be empty")
	framework.Logf("Parse url %q succeed", serverURL)
	return url
}

func createDefaultAttach(c internalapi.RuntimeService, containerID string) string {
	By("attach container: " + containerID)
	req := &kubeapi.AttachRequest{
		ContainerId: containerID,
		Stdin:       true,
		Tty:         false,
	}

	resp, err := c.Attach(req)
	framework.ExpectNoError(err, "failed to attach in container %q", containerID)
	framework.Logf("Get attach url: " + resp.Url)
	return resp.Url
}

func checkAttach(c internalapi.RuntimeService, attachServerURL string) {
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}
	reader, writer := io.Pipe()
	var out string

	go func() {
		writer.Write([]byte("echo hello\n"))
		Eventually(func() string {
			out = localOut.String()
			return out
		}).ShouldNot(BeEmpty())
		writer.Close()
	}()

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	url := parseURL(attachServerURL)
	e, err := remotecommand.NewExecutor(&rest.Config{}, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", attachServerURL)

	err = e.Stream(remotecommand.StreamOptions{
		SupportedProtocols: remotecommandconsts.SupportedStreamingProtocols,
		Stdin:              reader,
		Stdout:             localOut,
		Stderr:             localErr,
		Tty:                false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", attachServerURL)

	Expect(out).To(Equal("hello\n"), "The stdout of exec should be hello")
	Expect(localErr.String()).To(BeEmpty(), "The stderr of attach should be empty")
	framework.Logf("Check attach url %q succeed", attachServerURL)
}

func createDefaultPortForward(c internalapi.RuntimeService, podID string) string {
	By("port forward PodSandbox: " + podID)
	req := &kubeapi.PortForwardRequest{
		PodSandboxId: podID,
	}

	resp, err := c.PortForward(req)
	framework.ExpectNoError(err, "failed to port forward PodSandbox %q", podID)
	framework.Logf("Get port forward url: " + resp.Url)
	return resp.Url
}

func checkPortForward(c internalapi.RuntimeService, portForwardSeverURL string) {
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	defer close(stopChan)

	url := parseURL(portForwardSeverURL)
	e, err := remotecommand.NewExecutor(&rest.Config{}, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", portForwardSeverURL)

	pf, err := portforward.New(e, []string{"8000:80"}, stopChan, readyChan, os.Stdout, os.Stderr)
	framework.ExpectNoError(err, "failed to create port forward for %q", portForwardSeverURL)

	go func() {
		By("start port forward")
		err = pf.ForwardPorts()
		framework.ExpectNoError(err, "failed to start port forward for %q", portForwardSeverURL)
	}()

	By("check if we can get nginx main page via localhost:8000")
	checkNginxMainPage(c, "")
	framework.Logf("Check port forward url %q succeed", portForwardSeverURL)
}

func startDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *kubeapi.PodSandboxConfig) string {
	By("pull necessary image")
	imageSpec := &kubeapi.ImageSpec{
		Image: latestTestImageRef,
	}
	_, err := ic.PullImage(imageSpec, nil)
	framework.ExpectNoError(err, "Failed to pull image: %v", err)
	containerName := "simple-container-" + framework.NewUUID()
	containerConfig := &kubeapi.ContainerConfig{
		Metadata: &kubeapi.ContainerMetadata{
			Name: containerName,
		},
		Image:   imageSpec,
		Command: []string{"/bin/sh"},
		Linux:   &kubeapi.LinuxContainerConfig{},
		Stdin:   true,
		Tty:     false,
	}
	containerID, err := rc.CreateContainer(podID, containerConfig, podConfig)
	framework.ExpectNoError(err, "Failed to create container: %v", err)

	By("start container")
	err = rc.StartContainer(containerID)
	framework.ExpectNoError(err, "Failed to start container: %v", err)

	// sleep 2s to make sure container start is ready, workaround for random failed in travis
	// TODO: remove this
	time.Sleep(2 * time.Second)
	return containerID
}

func startNginxContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *kubeapi.PodSandboxConfig) string {
	By("pull necessary image")
	imageSpec := &kubeapi.ImageSpec{
		Image: nginxImage,
	}
	_, err := ic.PullImage(imageSpec, nil)
	framework.ExpectNoError(err, "Failed to pull image: %v", err)
	containerName := "simple-container-" + framework.NewUUID()
	containerConfig := &kubeapi.ContainerConfig{
		Metadata: &kubeapi.ContainerMetadata{
			Name: containerName,
		},
		Image: imageSpec,
	}
	containerID, err := rc.CreateContainer(podID, containerConfig, podConfig)
	framework.ExpectNoError(err, "Failed to create container: %v", err)

	By("start container")
	err = rc.StartContainer(containerID)
	framework.ExpectNoError(err, "Failed to start container: %v", err)

	// sleep 2s to make sure container start is ready, workaround for random failed in travis
	// TODO: remove this
	time.Sleep(2 * time.Second)
	return containerID
}

// checkNginxMainPage check if the we can get the main page of nginx via given IP:port.
func checkNginxMainPage(c internalapi.RuntimeService, podID string) {
	By("get the IP:port needed to be checked")
	var err error
	var resp *http.Response

	url := "http://localhost:" + strconv.Itoa(int(nginxHostPort))

	framework.Logf("the IP:port is " + url)

	By("check the content of " + url)

	Eventually(func() error {
		resp, err = http.Get(url)
		return err

	}, time.Minute, time.Second).Should(BeNil())

	Expect(resp.StatusCode).To(Equal(200), "The status code of response should be 200.")
	framework.Logf("check port mapping succeed")

}

// createPodSandboxWithPortMapping create a PodSandbox with port mapping.
func createPodSandboxWithPortMapping(c internalapi.RuntimeService, portMappings []*kubeapi.PortMapping) (string, *kubeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-port-mapping" + framework.NewUUID()
	config := &kubeapi.PodSandboxConfig{
		Metadata:     buildPodSandboxMetadata(podSandboxName),
		PortMappings: portMappings,
		Linux:        &kubeapi.LinuxPodSandboxConfig{},
	}
	podID := createPodSandboxOrFail(c, config)
	return podID, config
}
