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

	"k8s.io/kubernetes/pkg/kubelet/util/ioutils"
)

// ExecSync runs a command in a container synchronously.
func (h *Runtime) ExecSync(rawContainerID string, cmd []string, timeout int64) (stdout, stderr []byte, exitCode int32, err error) {
	var stdoutBuffer bytes.Buffer

	// check if container is running
	err = h.client.CheckContainerRunningStatus(rawContainerID)
	if err != nil {
		return nil, nil, -1, err
	}

	exitCode, err = h.client.ExecInContainer(rawContainerID, cmd,
		nil, // doesn't need stdin here
		ioutils.WriteCloserWrapper(&stdoutBuffer),
		nil,   // TODO: For now, hyper combine stdout and stderr to one
		false, // doesn't need tty in ExecSync
		timeout)

	if err != nil {
		return nil, nil, -1, err
	}

	if exitCode != 0 {
		return nil, stdoutBuffer.Bytes(), exitCode, nil
	}

	return stdoutBuffer.Bytes(), nil, 0, nil
}

// Exec execute a command in the container.
func (h *Runtime) Exec(rawContainerID string, cmd []string, tty bool, stdin io.Reader, stdout, stderr io.WriteCloser) error {
	return fmt.Errorf("Not implemented")
}

// Attach prepares a streaming endpoint to attach to a running container.
func (h *Runtime) Attach() error {
	return fmt.Errorf("Not implemented")
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (h *Runtime) PortForward() error {
	return fmt.Errorf("Not implemented")
}
