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
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

func newTestStreamingRuntime() (*streamingRuntime, *fakeClientInterface) {
	publicClient := newFakeClientInterface(nil)
	client := &Client{
		client: publicClient,
	}
	return &streamingRuntime{
		client: client,
	}, publicClient
}

func TestExec(t *testing.T) {
	r, fakeClient := newTestStreamingRuntime()
	container := "sidecar"
	containerId, PodId := "c", "p"
	containers := []*FakeContainer{}
	//Create runnning containers for test
	for i := 0; i < 2; i++ {
		containerID := fmt.Sprintf("%s%s%d", containerId, "*", i)
		podID := fmt.Sprintf("%s%s%d", PodId, "*", i)
		containerName := fmt.Sprintf("%s%d", container, i)
		container := &FakeContainer{
			ID:     containerID,
			Name:   containerName,
			Status: "running",
			PodID:  podID,
		}
		containers = append(containers, container)
	}
	fakeClient.SetFakeContainers(containers)
	//Create a temporaty empty file
	file, err := ioutil.TempFile("", "tmp")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	rawContainerID := fmt.Sprintf("%s%s%d", containerId, "*", 0)
	cmd := []string{
		"ls",
	}
	//Test streamingRuntime Exec
	err = r.Exec(rawContainerID, cmd, file, nil, nil, false, nil)
	assert.NoError(t, err)
	//Test streamingRuntime Attach
	rawContainerID = fmt.Sprintf("%s%s%d", containerId, "*", 1)
	err = r.Attach(rawContainerID, file, nil, nil, false, nil)
	assert.NoError(t, err)
}

func TestPortForward(t *testing.T) {
	r, fakeClient := newTestStreamingRuntime()
	podId := "p"
	pods := []*FakePod{}
	//Create running pods for test

	podID := fmt.Sprintf("%s%s%d", podId, "*", 0)
	pod := &FakePod{
		PodID:     podID,
		Status:    "Running",
		PodVolume: nil,
	}
	pods = append(pods, pod)

	fakeClient.SetFakePod(pods)

	//Create a temporaty empty file
	file, err := ioutil.TempFile("", "tmp")
	assert.NoError(t, err)
	defer os.Remove(file.Name())
	err = r.PortForward(podID, int32(0), file)
	assert.NoError(t, err)

}

func newTestRuntimeWithStreaming(url *url.URL) (*Runtime, *fakeClientInterface) {
	publicClient := newFakeClientInterface(nil)
	config := streaming.Config{
		BaseURL: url,
	}
	streamingServer, _ := streaming.NewServer(config, nil)
	client := &Client{
		client: publicClient,
	}
	return &Runtime{
		client:          client,
		streamingServer: streamingServer,
	}, publicClient
}

func TestRuntimeExec(t *testing.T) {
	host, scheme := "127.0.0.1", "http"
	url := &url.URL{
		Scheme: scheme,
		Host:   host,
	}
	r, fakeClient := newTestRuntimeWithStreaming(url)
	container := "sidecar"
	containerId, PodId := "c", "p"
	containers := []*FakeContainer{}
	//Create runnning containers for test
	for i := 0; i < 2; i++ {
		containerID := fmt.Sprintf("%s%s%d", containerId, "*", i)
		podID := fmt.Sprintf("%s%s%d", PodId, "*", i)
		containerName := fmt.Sprintf("%s%d", container, i)
		container := &FakeContainer{
			ID:     containerID,
			Name:   containerName,
			Status: "running",
			PodID:  podID,
		}
		containers = append(containers, container)
	}
	fakeClient.SetFakeContainers(containers)
	//Test Runtime Exec
	rawContainerID := fmt.Sprintf("%s%s%d", containerId, "*", 0)
	execRequest := &kubeapi.ExecRequest{
		ContainerId: rawContainerID,
	}
	execResponse, err := r.Exec(execRequest)
	assert.NoError(t, err)
	//We cann't knew the token before it's created,eg:"http://127.0.0.1/exec/-BnwYvAM",-BnwYvAM is the token
	urlRep := deleteToken(execResponse.Url)
	expected := fmt.Sprintf("%s%s%s%s%s", scheme, "://", host, "/", "exec")
	assert.Equal(t, urlRep, expected)

	//Test Runtime Attach
	rawContainerID = fmt.Sprintf("%s%s%d", containerId, "*", 1)
	attachRequest := &kubeapi.AttachRequest{
		ContainerId: rawContainerID,
	}
	attachResponse, err := r.Attach(attachRequest)
	assert.NoError(t, err)
	urlRep = deleteToken(attachResponse.Url)
	expected = fmt.Sprintf("%s%s%s%s%s", scheme, "://", host, "/", "attach")
	assert.Equal(t, urlRep, expected)
}

func TestRuntimePortForward(t *testing.T) {
	host, scheme := "127.0.0.1", "http"
	url := &url.URL{
		Scheme: scheme,
		Host:   host,
	}
	r, fakeClient := newTestRuntimeWithStreaming(url)
	podId := "p"
	pods := []*FakePod{}
	//Create running pods for test
	podID := fmt.Sprintf("%s%s%d", podId, "*", 0)
	pod := &FakePod{
		PodID:     podID,
		Status:    "Running",
		PodVolume: nil,
	}
	pods = append(pods, pod)

	fakeClient.SetFakePod(pods)
	//Test Runtime PortForward
	portForwardRequest := &kubeapi.PortForwardRequest{
		PodSandboxId: podID,
	}
	portForwardResponse, err := r.PortForward(portForwardRequest)
	assert.NoError(t, err)
	//We cann't knew the token before it's created,eg:"http://127.0.0.1/exec/-BnwYvAM",-BnwYvAM is the token
	urlRep := deleteToken(portForwardResponse.Url)
	expected := fmt.Sprintf("%s%s%s%s%s", scheme, "://", host, "/", "portforward")
	assert.Equal(t, urlRep, expected)

}

//Used to remove the token from the string
func deleteToken(url string) string {
	urlSplit := strings.Split(url, "/")
	//urlRep := fmt.Sprintf("%s%s", urlSplit[0], "/")
	urlRep := urlSplit[0]
	for i := 1; i < len(urlSplit)-1; i++ {
		urlRep = fmt.Sprintf("%s%s%s", urlRep, "/", urlSplit[i])
	}
	return urlRep
}
