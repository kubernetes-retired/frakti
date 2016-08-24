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
	"time"

	"google.golang.org/grpc"
	"k8s.io/frakti/pkg/hyper/types"
)

const (
	//timeout in second for creating context with timeout.
	hyperContextTimeout = 15 * time.Second

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
		return int(resp.Code), resp.Cause, err
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
	podList, err := c.client.PodList(
		ctx,
		&request,
	)
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
	cinfo, err := c.client.ContainerInfo(
		ctx,
		&req,
	)
	if err != nil {
		return nil, err
	}

	return cinfo.ContainerInfo, nil
}

// StopContainer stops a hyper container
func (c *Client) StopContainer(containerID string, timeout int64) error {
	//do checks about container and pod status
	containerInfo, err := c.GetContainerInfo(containerID)
	if err != nil {
		return err
	}

	podInfo, err := c.GetPodInfo(containerInfo.PodID)
	if err != nil {
		return err
	}

	if podInfo.Status.Phase != "running" && podInfo.Status.Phase != "Running" {
		return fmt.Errorf("Pod %s is not running.", containerInfo.PodID)
	}

	if containerInfo.Status.Phase != "running" {
		return fmt.Errorf("Container %s is not running.", containerID)
	}

	ch := make(chan error, 1)

	go func(containerID string) {
		ch <- nil
		ctx, cancel := getContextWithTimeout(hyperContextTimeout)
		defer cancel()

		_, err := c.client.ContainerStop(ctx, &types.ContainerStopRequest{ContainerID: containerID})
		ch <- err
	}(containerID)

	<-ch

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
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
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

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if out != nil {
			n, err := out.Write(res.Data)
			if err != nil {
				return err
			}
			if n != len(res.Data) {
				return io.ErrShortWrite
			}
		}
	}

	return nil
}

// RemoveImage removes a image from hyperd
func (c *Client) RemoveImage(image, tag string) error {
	ctx, cancel := getContextWithTimeout(hyperContextTimeout)
	defer cancel()

	_, err := c.client.ImageRemove(ctx, &types.ImageRemoveRequest{Image: fmt.Sprintf("%s:%s", image, tag)})
	return err
}
