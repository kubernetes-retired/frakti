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
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/tools/remotecommand"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	"k8s.io/kubernetes/pkg/kubelet/util/ioutils"
	utilexec "k8s.io/utils/exec"
)

type streamingRuntime struct {
	client *Client
}

// emphasize streamingRuntime should implement streaming.Runtime interface.
var _ streaming.Runtime = &streamingRuntime{}

// Exec execute a command in the container.
func (sr *streamingRuntime) Exec(rawContainerID string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	err := ensureContainerRunning(sr.client, rawContainerID)
	if err != nil {
		return err
	}
	err = sr.client.ExecInContainer(rawContainerID, cmd, stdin, stdout, stderr, tty, resize, 0)
	if _, ok := err.(utilexec.CodeExitError); ok {
		return nil
	}
	return err
}

// Attach attach to a running container.
func (sr *streamingRuntime) Attach(rawContainerID string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	err := ensureContainerRunning(sr.client, rawContainerID)
	if err != nil {
		return err
	}

	return sr.client.AttachContainer(rawContainerID, stdin, stdout, stderr, tty, resize)
}

// PortForward forward ports from a PodSandbox.
func (sr *streamingRuntime) PortForward(podSandboxID string, port int32, stream io.ReadWriteCloser) error {
	running, err := isPodSandboxRunning(sr.client, podSandboxID)
	if err != nil {
		glog.Errorf("isPodSandboxRunning for sandbox %q: %v", podSandboxID, err)
		return err
	}
	if !running {
		return fmt.Errorf("sandbox %s is not running", podSandboxID)
	}

	cmds := []string{"/sbin/socat", "-", fmt.Sprintf("TCP4:127.0.0.1:%d", port)}
	return sr.client.ExecInSandbox(podSandboxID, cmds, stream, stream, stream, false, 0)
}

// ExecSync runs a command in a container synchronously.
func (h *Runtime) ExecSync(rawContainerID string, cmd []string, timeout time.Duration) (stdout, stderr []byte, err error) {
	var (
		stdoutBuffer bytes.Buffer
		stderrBuffer bytes.Buffer
	)

	// check if container is running
	err = ensureContainerRunning(h.client, rawContainerID)
	if err != nil {
		return nil, nil, err
	}

	err = h.client.ExecInContainer(rawContainerID, cmd,
		nil, // don't need stdin here
		ioutils.WriteCloserWrapper(&stdoutBuffer),
		ioutils.WriteCloserWrapper(&stderrBuffer),
		false, // don't need tty in ExecSync
		nil,   // don't need resize
		timeout)

	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (h *Runtime) Exec(req *kubeapi.ExecRequest) (*kubeapi.ExecResponse, error) {
	if h.streamingServer == nil {
		return nil, streaming.ErrorStreamingDisabled("exec")
	}
	err := ensureContainerRunning(h.client, req.ContainerId)
	if err != nil {
		return nil, err
	}

	return h.streamingServer.GetExec(req)
}

// Attach prepares a streaming endpoint to attach to a running container.
func (h *Runtime) Attach(req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	if h.streamingServer == nil {
		return nil, streaming.ErrorStreamingDisabled("attach")
	}
	err := ensureContainerRunning(h.client, req.ContainerId)
	if err != nil {
		return nil, err
	}

	return h.streamingServer.GetAttach(req)
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (h *Runtime) PortForward(req *kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	if h.streamingServer == nil {
		return nil, streaming.ErrorStreamingDisabled("portforward")
	}

	running, err := isPodSandboxRunning(h.client, req.PodSandboxId)
	if err != nil {
		glog.Errorf("isPodSandboxRunning for sandbox %q: %v", req.PodSandboxId, err)
		return nil, err
	}
	if !running {
		return nil, fmt.Errorf("sandbox %s is not running", req.PodSandboxId)
	}

	return h.streamingServer.GetPortForward(req)
}
