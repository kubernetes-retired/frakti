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
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/frakti/pkg/hyper/types"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	utilexec "k8s.io/utils/exec"
)

const (
	//timeout in second for creating context with timeout.
	hyperContextTimeout = 2 * time.Minute

	//timeout for image pulling progress report
	defaultImagePullingStuckTimeout = 1 * time.Minute

	// errorCodePodNotFound is the response code of PodRemove,
	// when the pod can not be found.
	errorCodePodNotFound = -2
)

// Client is the gRPC client for hyperd
type Client struct {
	client  types.PublicAPIClient
	timeout time.Duration
}

// NewClient creates a new hyper client
func NewClient(server string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.Dial(server, grpc.WithInsecure(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp", addr, timeout)
		}))
	if err != nil {
		return nil, err
	}

	return &Client{
		client:  types.NewPublicAPIClient(conn),
		timeout: timeout,
	}, nil
}

// GetVersion gets hyperd version
func (c *Client) GetVersion() (string, string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	resp, err := c.client.Version(ctx, &types.VersionRequest{})
	if err != nil {
		return "", "", err
	}

	return resp.Version, resp.ApiVersion, nil
}

// CreatePod creates a pod and returns the pod ID.
func (c *Client) CreatePod(spec *types.UserPod) (string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	resp, err := c.client.PodCreate(ctx, &types.PodCreateRequest{
		PodSpec: spec,
	})
	if err != nil {
		return "", err
	}

	return resp.PodID, nil
}

// StartPod starts a pod by podID.
func (c *Client) StartPod(podID string) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	_, err := c.client.PodStart(ctx, &types.PodStartRequest{
		PodID: podID,
	})

	return err
}

// StopPod stops a pod.
func (c *Client) StopPod(podID string) (int, string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	isRunning, err := isPodSandboxRunning(c, podID)
	if err != nil {
		return 0, "", err
	}
	if !isRunning {
		glog.V(3).Infof("PodSandbox %q is already stopped, skip", podID)
		return 0, "", nil
	}

	resp, err := c.client.PodStop(ctx, &types.PodStopRequest{
		PodID: podID,
	})
	if err != nil {
		return 0, "", err
	}

	return int(resp.Code), resp.Cause, nil
}

// RemovePod removes a pod by podID
func (c *Client) RemovePod(podID string) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	resp, err := c.client.PodRemove(
		ctx,
		&types.PodRemoveRequest{PodID: podID},
	)
	if err != nil {
		if isPodNotFoundError(err, podID) || (resp != nil && resp.Code == errorCodePodNotFound) {
			return nil
		}
		return err
	}

	return nil
}

// GetPodInfo gets pod info by podID
func (c *Client) GetPodInfo(podID string) (*types.PodInfo, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	request := types.PodInfoRequest{
		PodID: podID,
	}
	pod, err := c.client.PodInfo(ctx, &request)
	if err != nil {
		return nil, err
	}

	return pod.PodInfo, nil
}

// GetPodList get a list of Pods
func (c *Client) GetPodList() ([]*types.PodListResult, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	request := types.PodListRequest{}
	podList, err := c.client.PodList(ctx, &request)
	if err != nil {
		return nil, err
	}

	return podList.PodList, nil
}

// GetContainerInfo gets container info by container name or id
func (c *Client) GetContainerInfo(container string) (*types.ContainerInfo, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ContainerInfoRequest{
		Container: container,
	}
	cinfo, err := c.client.ContainerInfo(ctx, &req)
	if err != nil {
		return nil, err
	}

	return cinfo.ContainerInfo, nil
}

// StartContainer starts a hyper container
func (c *Client) StartContainer(containerID string) error {
	isRunning, err := isContainerRunning(c, containerID)
	if err != nil {
		return err
	}
	if isRunning {
		glog.V(3).Infof("Container %q is already running, skip", containerID)
		return nil
	}

	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	_, err = c.client.ContainerStart(ctx, &types.ContainerStartRequest{ContainerId: containerID})
	if err != nil {
		return err
	}

	return nil
}

// RemoveContainer removes a hyper container
func (c *Client) RemoveContainer(containerID string) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	_, err := c.client.ContainerRemove(ctx, &types.ContainerRemoveRequest{ContainerId: containerID})
	if err != nil {
		if strings.Contains(err.Error(), "cannot find container") {
			return nil
		}
		return err
	}
	return nil
}

// StopContainer stops a hyper container
func (c *Client) StopContainer(containerID string, timeout int64) error {
	ctxTimeout := time.Duration(timeout) * time.Second
	if ctxTimeout < hyperContextTimeout {
		ctxTimeout = hyperContextTimeout
	} else {
		// extend 5s give hyperd enough time to response if timeout
		ctxTimeout += time.Duration(5) * time.Second
	}

	// do checks about container status
	isRunning, err := isContainerRunning(c, containerID)
	if err != nil {
		return err
	}
	if !isRunning {
		glog.V(3).Infof("Container %q is already stopped, skip", containerID)
		return nil
	}
	ctx, cancel := getContextWithTimeout(ctxTimeout)
	defer cancel()

	_, err = c.client.ContainerStop(ctx, &types.ContainerStopRequest{ContainerID: containerID, Timeout: timeout})

	return err
}

// GetImageInfo gets the information of the image.
func (c *Client) GetImageInfo(image, tag string) (*types.ImageInfo, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	// check if tag is digest
	repoSep := ":"
	if strings.Index(tag, ":") > 0 {
		repoSep = "@"
	}

	// TODO: use filter to get the image.
	req := types.ImageListRequest{}
	imageList, err := c.client.ImageList(ctx, &req)
	if err != nil {
		return nil, err
	}
	fullImageName := fmt.Sprintf("%s%s%s", image, repoSep, tag)
	for _, image := range imageList.ImageList {
		for _, i := range image.RepoDigests {
			if i == fullImageName {
				return image, nil
			}
		}
		for _, i := range image.RepoTags {
			if i == fullImageName {
				return image, nil
			}
		}
	}

	return nil, fmt.Errorf("image %q with tag %q not found", image, tag)
}

// GetImages gets a list of images
func (c *Client) GetImages() ([]*types.ImageInfo, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ImageListRequest{}
	imageList, err := c.client.ImageList(ctx, &req)
	if err != nil {
		return nil, err
	}

	return imageList.ImageList, nil
}

// PullImage pulls a image from registry
func (c *Client) PullImage(image, tag string, auth *types.AuthConfig, out io.Writer) error {
	ctx, cancel := getContextWithCancel()
	defer cancel()

	request := types.ImagePullRequest{
		Image: image,
		Tag:   tag,
		Auth:  auth,
	}
	stream, err := c.client.ImagePull(ctx, &request)
	if err != nil {
		return err
	}

	errC := make(chan error)
	progressC := make(chan struct{})
	ticker := time.NewTicker(defaultImagePullingStuckTimeout)
	defer ticker.Stop()

	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				errC <- nil
				return
			}
			if err != nil {
				errC <- err
				return
			}
			progressC <- struct{}{}

			if out != nil {
				n, err := out.Write(res.Data)
				if err != nil {
					errC <- err
					return
				}
				if n != len(res.Data) {
					errC <- io.ErrShortWrite
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			// pulling image timeout, cancel it
			return fmt.Errorf("Cancel pulling image %q because of no progress for %v", image, defaultImagePullingStuckTimeout)
		case err = <-errC:
			// if nil, got EOF, session finished
			// else return err
			return err
		case <-progressC:
			// got progress from pulling image, reset the clock
			ticker.Stop()
			ticker = time.NewTicker(defaultImagePullingStuckTimeout)
		}
	}
}

// RemoveImage removes a image from hyperd
func (c *Client) RemoveImage(image, tag string) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	// check if tag is digest
	repoSep := ":"
	if strings.Index(tag, ":") > 0 {
		repoSep = "@"
	}

	_, err := c.client.ImageRemove(ctx, &types.ImageRemoveRequest{Image: fmt.Sprintf("%s%s%s", image, repoSep, tag), Force: true})
	return err
}

// GetContainerList gets a list of containers
func (c *Client) GetContainerList() ([]*types.ContainerListResult, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ContainerListRequest{}
	containerList, err := c.client.ContainerList(ctx, &req)
	if err != nil {
		return nil, err
	}

	return containerList.ContainerList, nil
}

// CreateContainer creates a container
func (c *Client) CreateContainer(podID string, spec *types.UserContainer) (string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ContainerCreateRequest{
		PodID:         podID,
		ContainerSpec: spec,
	}

	resp, err := c.client.ContainerCreate(ctx, &req)
	if err != nil {
		return "", err
	}

	return resp.ContainerID, nil
}

// ContainerExecCreate creates exec in a container
func (c *Client) ContainerExecCreate(containerId string, cmd []string, tty bool) (string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ExecCreateRequest{
		ContainerID: containerId,
		Command:     cmd,
		Tty:         tty,
	}
	resp, err := c.client.ExecCreate(ctx, &req)
	if err != nil {
		return "", err
	}

	return resp.ExecID, nil
}

// TTYResize resizes the tty of the specified container
func (c *Client) TTYResize(containerID, execID string, height, width int32) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := &types.TTYResizeRequest{
		ContainerID: containerID,
		ExecID:      execID,
		Height:      height,
		Width:       width,
	}
	_, err := c.client.TTYResize(ctx, req)

	return err
}

// ExecInContainer exec a command in container with specified io, tty and timeout
func (c *Client) ExecInContainer(containerId string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	execID, err := c.ContainerExecCreate(containerId, cmd, tty)
	if err != nil {
		return err
	}

	req := types.ExecStartRequest{
		ContainerID: containerId,
		ExecID:      execID,
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if timeout > 0 {
		ctx, cancel = getContextWithTimeout(timeout)
	} else if timeout == 0 {
		ctx, cancel = getContextWithCancel()
	} else {
		return fmt.Errorf("timeout should be non-negative")
	}
	defer cancel()

	kubecontainer.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		if err := c.TTYResize(containerId, execID, int32(size.Height), int32(size.Width)); err != nil {
			glog.Errorf("Resize tty failed: %v", err)
		}
	})

	stream, err := c.client.ExecStart(ctx)
	if err != nil {
		return err
	}
	if err := stream.Send(&req); err != nil {
		return err
	}

	var recvStdoutError chan error
	extractor := NewExtractor(tty)

	if stdout != nil || stderr != nil {
		recvStdoutError = promiseGo(func() (err error) {
			for {
				out, err := stream.Recv()
				if err != nil && err != io.EOF {
					return err
				}
				if out != nil && out.Stdout != nil {
					so, se, ee := extractor.Extract(out.Stdout)
					if ee != nil {
						return ee
					}
					if len(so) > 0 && stdout != nil {
						nw, ew := stdout.Write(so)
						if ew != nil {
							return ew
						}
						if nw != len(so) {
							return io.ErrShortWrite
						}
					}
					if len(se) > 0 && stderr != nil {
						nw, ew := stderr.Write(se)
						if ew != nil {
							return ew
						}
						if nw != len(se) {
							return io.ErrShortWrite
						}
					}
				}
				if err == io.EOF {
					break
				}
			}
			return nil
		})
	}

	if stdin != nil {
		go func() error {
			defer stream.CloseSend()
			buf := make([]byte, 1024)
			for {
				nr, err := stdin.Read(buf)
				if nr > 0 {
					if err := stream.Send(&types.ExecStartRequest{Stdin: buf[:nr]}); err != nil {
						return err
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
			return nil
		}()
	}

	if stdout != nil || stderr != nil {
		if err := <-recvStdoutError; err != nil {
			return err
		}
	}

	// get exit code
	exitCode, err := c.Wait(containerId, execID, false)
	if err != nil {
		return err
	}

	return utilexec.CodeExitError{Err: fmt.Errorf("Exit with code %d", exitCode), Code: int(exitCode)}
}

// Wait gets exit code by containerID and execID
func (c *Client) Wait(containerId, execId string, noHang bool) (int32, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.WaitRequest{
		Container: containerId,
		ProcessId: execId,
		NoHang:    noHang,
	}

	resp, err := c.client.Wait(ctx, &req)
	if err != nil {
		return -1, err
	}

	return resp.ExitCode, nil
}

// AttachContainer attach a container with id, io stream and resize
func (c *Client) AttachContainer(containerID string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	kubecontainer.HandleResizing(resize, func(size remotecommand.TerminalSize) {
		if err := c.TTYResize(containerID, "", int32(size.Height), int32(size.Width)); err != nil {
			glog.Errorf("Resize tty failed: %v", err)
		}
	})

	ctx, cancel := getContextWithCancel()
	defer cancel()

	stream, err := c.client.Attach(ctx)
	if err != nil {
		return err
	}

	req := &types.AttachMessage{
		ContainerID: containerID,
	}
	err = stream.Send(req)
	if err != nil {
		return err
	}

	var recvStdoutError chan error
	extractor := NewExtractor(tty)

	if stdout != nil || stderr != nil {
		recvStdoutError = promiseGo(func() (err error) {
			for {
				out, err := stream.Recv()
				if err != nil && err != io.EOF {
					return err
				}
				if out != nil && out.Data != nil {
					so, se, ee := extractor.Extract(out.Data)
					if ee != nil {
						return ee
					}
					if len(so) > 0 && stdout != nil {
						nw, ew := stdout.Write(so)
						if ew != nil {
							return ew
						}
						if nw != len(so) {
							return io.ErrShortWrite
						}
					}
					if len(se) > 0 && stderr != nil {
						nw, ew := stderr.Write(se)
						if ew != nil {
							return ew
						}
						if nw != len(se) {
							return io.ErrShortWrite
						}
					}
				}
				if err == io.EOF {
					break
				}
			}
			return nil
		})
	}

	if stdin != nil {
		go func() error {
			defer stream.CloseSend()
			buf := make([]byte, 1024)
			for {
				nr, err := stdin.Read(buf)
				if nr > 0 {
					if err := stream.Send(&types.AttachMessage{Data: buf[:nr]}); err != nil {
						return err
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
			return nil
		}()
	}

	if stdout != nil || stderr != nil {
		if err := <-recvStdoutError; err != nil {
			return err
		}
	}
	return nil
}

// ExecInSandbox exec a command in sandbox with specified io, tty and timeout
func (c *Client) ExecInSandbox(sandboxID string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, timeout time.Duration) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if timeout > 0 {
		ctx, cancel = getContextWithTimeout(timeout)
	} else if timeout == 0 {
		ctx, cancel = getContextWithCancel()
	} else {
		return fmt.Errorf("timeout should be non-negative")
	}
	defer cancel()

	stream, err := c.client.ExecVM(ctx)
	if err != nil {
		return err
	}
	if err = stream.Send(&types.ExecVMRequest{
		PodID:   sandboxID,
		Command: cmd,
	}); err != nil {
		return err
	}

	var recvStdoutError chan error
	if stdout != nil || stderr != nil {
		recvStdoutError = promiseGo(func() (err error) {
			for {
				out, err := stream.Recv()
				if out != nil {
					if len(out.Stdout) > 0 && stdout != nil {
						nw, ew := stdout.Write(out.Stdout)
						if ew != nil {
							return ew
						}
						if nw != len(out.Stdout) {
							return io.ErrShortWrite
						}
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	if stdin != nil {
		go func() error {
			defer stream.CloseSend()
			buf := make([]byte, 1024)
			for {
				nr, err := stdin.Read(buf)
				if nr > 0 {
					if err = stream.Send(&types.ExecVMRequest{
						Stdin: buf[:nr],
					}); err != nil {
						return err
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}
			return nil
		}()
	}

	if stdout != nil || stderr != nil {
		if err = <-recvStdoutError; err != nil {
			return err
		}
	}

	return nil
}
