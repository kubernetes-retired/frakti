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
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/frakti/pkg/hyper/types"
)

const (
	//timeout in second for creating context with timeout.
	hyperContextTimeout = 2 * time.Minute

	//timeout for image pulling progress report
	defaultImagePullingStuckTimeout = 1 * time.Minute

	//response code of PodRemove, when the pod can not be found.
	E_NOT_FOUND = -2
)

// Client is the gRPC client for hyperd
type Client struct {
	client  types.PublicAPIClient
	timeout time.Duration
}

// NewClient creates a new hyper client
func NewClient(server string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.Dial(server, grpc.WithInsecure())
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

	stream, err := c.client.PodStart(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&types.PodStartMessage{PodID: podID}); err != nil {
		return err
	}

	if _, err := stream.Recv(); err != nil {
		return err
	}

	return nil
}

// StopPod stops a pod.
func (c *Client) StopPod(podID string) (int, string, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

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

	if resp.Code == E_NOT_FOUND {
		return nil
	}

	if err != nil {
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

// StopContainer stops a hyper container
func (c *Client) StopContainer(containerID string, timeout int64) error {
	if timeout <= 0 {
		return fmt.Errorf("Timeout can not be %d, it must be greater than zero.", timeout)
	}

	// do checks about container status
	err := c.CheckIfContainerRunning(containerID)
	if err != nil {
		return err
	}

	ch := make(chan error, 1)

	go func(containerID string) {
		ctx, cancel := getContextWithTimeout(hyperContextTimeout)
		defer cancel()

		_, err := c.client.ContainerStop(ctx, &types.ContainerStopRequest{ContainerID: containerID})
		ch <- err
	}(containerID)

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("Stop container %s timeout", containerID)
	case err := <-ch:
		return err
	}
}

// GetImageInfo gets the information of the image.
func (c *Client) GetImageInfo(image, tag string) (*types.ImageInfo, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ImageListRequest{Filter: fmt.Sprintf("%s:%s", image, tag)}
	imageList, err := c.client.ImageList(ctx, &req)
	if err != nil {
		return nil, err
	}
	if len(imageList.ImageList) == 0 {
		return nil, fmt.Errorf("image %q not found", image)
	}

	return imageList.ImageList[0], nil
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

	_, err := c.client.ImageRemove(ctx, &types.ImageRemoveRequest{Image: fmt.Sprintf("%s%s%s", image, repoSep, tag)})
	return err
}

// GetContainerList gets a list of containers
func (c *Client) GetContainerList(auxiliary bool) ([]*types.ContainerListResult, error) {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	req := types.ContainerListRequest{
		Auxiliary: auxiliary,
	}
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

// ExecInContainer exec a command in container with specified io, tty and timeout
func (c *Client) ExecInContainer(containerId string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, timeout int64) (int32, error) {
	execID, err := c.ContainerExecCreate(containerId, cmd, tty)
	if err != nil {
		return -1, err
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
		ctx, cancel = getContextWithTimeout(time.Duration(timeout) * time.Second)
	} else if timeout == 0 {
		ctx, cancel = getContextWithCancel()
	} else {
		return -1, fmt.Errorf("timeout should be non-negative")
	}
	defer cancel()

	stream, err := c.client.ExecStart(ctx)
	if err != nil {
		return -1, err
	}
	if err := stream.Send(&req); err != nil {
		return -1, err
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
			buf := make([]byte, 32)
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
			return -1, err
		}
	}

	// get exit code
	exitCode, err := c.Wait(containerId, execID, false)
	if err != nil {
		return -1, err
	}

	return exitCode, nil
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

// CheckIfContainerRunning check if container is running by containerID
func (c *Client) CheckIfContainerRunning(containerID string) error {
	containerInfo, err := c.GetContainerInfo(containerID)
	if err != nil {
		return err
	}

	if containerInfo.Status.Phase != "running" {
		return fmt.Errorf("Container %s is not running.", containerID)
	}

	return nil
}
