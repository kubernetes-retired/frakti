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
	"bytes"
	"fmt"
	"io"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/util/ioutils"
	"k8s.io/kubernetes/pkg/util/term"
)

type streamingRuntime struct {
	client *Client
}

// Exec execute a command in the container.
func (sr *streamingRuntime) Exec(rawContainerID string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan term.Size) error {
	return fmt.Errorf("Not implemented")
}

// Attach attach to a running container.
func (sr *streamingRuntime) Attach(rawContainerID string, stdin io.Reader, stdout, stderr io.WriteCloser, resize <-chan term.Size) error {
	return fmt.Errorf("Not implemented")
}

// PortForward forward ports from a PodSandbox.
func (sr *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	return fmt.Errorf("Not implemented")
}

// ExecSync runs a command in a container synchronously.
func (h *Runtime) ExecSync(rawContainerID string, cmd []string, timeout int64) (stdout, stderr []byte, exitCode int32, err error) {
	var (
		stdoutBuffer bytes.Buffer
		stderrBuffer bytes.Buffer
	)

	// check if container is running
	err = h.client.CheckIfContainerRunning(rawContainerID)
	if err != nil {
		return nil, nil, -1, err
	}

	exitCode, err = h.client.ExecInContainer(rawContainerID, cmd,
		nil, // doesn't need stdin here
		ioutils.WriteCloserWrapper(&stdoutBuffer),
		ioutils.WriteCloserWrapper(&stderrBuffer),
		false, // doesn't need tty in ExecSync
		timeout)

	if err != nil {
		return nil, nil, -1, err
	}

	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), exitCode, nil
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (h *Runtime) Exec(req *kubeapi.ExecRequest) (*kubeapi.ExecResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (h *Runtime) Attach(req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (h *Runtime) PortForward(req *kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}
