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

package manager

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/frakti/pkg/runtime"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	runtimeAPIVersion = "0.1.0"
)

// FraktiManager serves the kubelet runtime gRPC api which will be
// consumed by kubelet
type FraktiManager struct {
	// The grpc server.
	server *grpc.Server
	// The streaming server.
	streamingServer streaming.Server

	runtimeService runtime.RuntimeService
	imageService   runtime.ImageService
}

// NewFraktiManager creates a new FraktiManager
func NewFraktiManager(runtimeService runtime.RuntimeService, imageService runtime.ImageService, streamingServer streaming.Server) (*FraktiManager, error) {
	s := &FraktiManager{
		server:          grpc.NewServer(),
		runtimeService:  runtimeService,
		imageService:    imageService,
		streamingServer: streamingServer,
	}
	s.registerServer()

	return s, nil
}

// Serve starts gRPC server at unix://addr
func (s *FraktiManager) Serve(addr string) error {
	glog.V(1).Infof("Start frakti at %s", addr)

	if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
		return err
	}

	if s.streamingServer != nil {
		go func() {
			if err := s.streamingServer.Start(true); err != nil {
				glog.Fatalf("Failed to start streaming server: %v", err)
			}
		}()
	}

	lis, err := net.Listen("unix", addr)
	if err != nil {
		glog.Fatalf("Failed to listen %s: %v", addr, err)
		return err
	}

	defer lis.Close()
	return s.server.Serve(lis)
}

func (s *FraktiManager) registerServer() {
	kubeapi.RegisterRuntimeServiceServer(s.server, s)
	kubeapi.RegisterImageServiceServer(s.server, s)
}

// Version returns the runtime name, runtime version and runtime API version.
func (s *FraktiManager) Version(ctx context.Context, req *kubeapi.VersionRequest) (*kubeapi.VersionResponse, error) {
	resp, err := s.runtimeService.Version(runtimeAPIVersion)
	if err != nil {
		glog.Errorf("Get version from runtime service failed: %v", err)
		return nil, err
	}

	return resp, nil
}

// RunPodSandbox creates and start a hyper Pod.
func (s *FraktiManager) RunPodSandbox(ctx context.Context, req *kubeapi.RunPodSandboxRequest) (*kubeapi.RunPodSandboxResponse, error) {
	glog.V(3).Infof("RunPodSandbox with request %s", req.String())

	podID, err := s.runtimeService.RunPodSandbox(req.Config)
	if err != nil {
		glog.Errorf("RunPodSandbox from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.RunPodSandboxResponse{PodSandboxId: podID}, nil
}

// StopPodSandbox stops the sandbox.
func (s *FraktiManager) StopPodSandbox(ctx context.Context, req *kubeapi.StopPodSandboxRequest) (*kubeapi.StopPodSandboxResponse, error) {
	glog.V(3).Infof("StopPodSandbox with request %s", req.String())

	err := s.runtimeService.StopPodSandbox(req.PodSandboxId)
	if err != nil {
		glog.Errorf("StopPodSandbox from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox deletes the sandbox.
func (s *FraktiManager) RemovePodSandbox(ctx context.Context, req *kubeapi.RemovePodSandboxRequest) (*kubeapi.RemovePodSandboxResponse, error) {
	glog.V(3).Infof("DeletePodSandbox with request %s", req.String())

	err := s.runtimeService.DeletePodSandbox(req.PodSandboxId)
	if err != nil {
		glog.Errorf("DeletePodSandbox from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.RemovePodSandboxResponse{}, nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (s *FraktiManager) PodSandboxStatus(ctx context.Context, req *kubeapi.PodSandboxStatusRequest) (*kubeapi.PodSandboxStatusResponse, error) {
	glog.V(3).Infof("PodSandboxStatus with request %s", req.String())

	podStatus, err := s.runtimeService.PodSandboxStatus(req.PodSandboxId)
	if err != nil {
		glog.Errorf("PodSandboxStatus from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.PodSandboxStatusResponse{Status: podStatus}, nil
}

// ListPodSandbox returns a list of SandBox.
func (s *FraktiManager) ListPodSandbox(ctx context.Context, req *kubeapi.ListPodSandboxRequest) (*kubeapi.ListPodSandboxResponse, error) {
	glog.V(3).Infof("ListPodSandbox with request %s", req.String())

	items, err := s.runtimeService.ListPodSandbox(req.GetFilter())
	if err != nil {
		glog.Errorf("ListPodSandbox from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.ListPodSandboxResponse{Items: items}, nil
}

// CreateContainer creates a new container in specified PodSandbox
func (s *FraktiManager) CreateContainer(ctx context.Context, req *kubeapi.CreateContainerRequest) (*kubeapi.CreateContainerResponse, error) {
	glog.V(3).Infof("CreateContainer with request %s", req.String())

	containerID, err := s.runtimeService.CreateContainer(req.PodSandboxId, req.Config, req.SandboxConfig)
	if err != nil {
		glog.Errorf("CreateContainer from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.CreateContainerResponse{ContainerId: containerID}, nil
}

// StartContainer starts the container.
func (s *FraktiManager) StartContainer(ctx context.Context, req *kubeapi.StartContainerRequest) (*kubeapi.StartContainerResponse, error) {
	glog.V(3).Infof("StartContainer with request %s", req.String())

	err := s.runtimeService.StartContainer(req.ContainerId)
	if err != nil {
		glog.Errorf("StartContainer from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.StartContainerResponse{}, nil
}

// StopContainer stops a running container with a grace period (i.e. timeout).
func (s *FraktiManager) StopContainer(ctx context.Context, req *kubeapi.StopContainerRequest) (*kubeapi.StopContainerResponse, error) {
	glog.V(3).Infof("StopContainer with request %s", req.String())

	err := s.runtimeService.StopContainer(req.ContainerId, req.Timeout)
	if err != nil {
		glog.Errorf("StopContainer from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.StopContainerResponse{}, nil
}

// RemoveContainer removes the container.
func (s *FraktiManager) RemoveContainer(ctx context.Context, req *kubeapi.RemoveContainerRequest) (*kubeapi.RemoveContainerResponse, error) {
	glog.V(3).Infof("RemoveContainer with request %s", req.String())

	err := s.runtimeService.RemoveContainer(req.ContainerId)
	if err != nil {
		glog.Errorf("RemoveContainer from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.RemoveContainerResponse{}, nil
}

// ListContainers lists all containers by filters.
func (s *FraktiManager) ListContainers(ctx context.Context, req *kubeapi.ListContainersRequest) (*kubeapi.ListContainersResponse, error) {
	glog.V(3).Infof("ListContainers with request %s", req.String())

	containers, err := s.runtimeService.ListContainers(req.GetFilter())
	if err != nil {
		glog.Errorf("ListContainers from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.ListContainersResponse{
		Containers: containers,
	}, nil
}

// ContainerStatus returns the container status.
func (s *FraktiManager) ContainerStatus(ctx context.Context, req *kubeapi.ContainerStatusRequest) (*kubeapi.ContainerStatusResponse, error) {
	glog.V(3).Infof("ContainerStatus with request %s", req.String())

	kubeStatus, err := s.runtimeService.ContainerStatus(req.ContainerId)
	if err != nil {
		glog.Errorf("ContainerStatus from runtime service failed: %v", err)
		return nil, err
	}

	return &kubeapi.ContainerStatusResponse{
		Status: kubeStatus,
	}, nil
}

// ExecSync runs a command in a container synchronously.
func (s *FraktiManager) ExecSync(ctx context.Context, req *kubeapi.ExecSyncRequest) (*kubeapi.ExecSyncResponse, error) {
	glog.V(3).Infof("ExecSync with request %s", req.String())

	stdout, stderr, err := s.runtimeService.ExecSync(req.ContainerId, req.Cmd, time.Duration(req.Timeout)*time.Second)
	var exitCode int32
	if err != nil {
		exitError, ok := err.(utilexec.ExitError)
		if !ok {
			glog.Errorf("ExecSync from runtime service failed: %v", err)
			return nil, err
		}
		exitCode = int32(exitError.ExitStatus())
	}

	return &kubeapi.ExecSyncResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}, nil
}

// Exec prepares a streaming endpoint to execute a command in the container.
func (s *FraktiManager) Exec(ctx context.Context, req *kubeapi.ExecRequest) (*kubeapi.ExecResponse, error) {
	glog.V(3).Infof("Exec with request %s", req.String())

	resp, err := s.runtimeService.Exec(req)
	if err != nil {
		glog.Errorf("Exec from runtime service failed: %v", err)
		return nil, err
	}

	return resp, nil
}

// Attach prepares a streaming endpoint to attach to a running container.
func (s *FraktiManager) Attach(ctx context.Context, req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	glog.V(3).Infof("Attach with request %s", req.String())

	resp, err := s.runtimeService.Attach(req)
	if err != nil {
		glog.Errorf("Attach from runtime service failed: %v", err)
		return nil, err
	}

	return resp, nil
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *FraktiManager) PortForward(ctx context.Context, req *kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (s *FraktiManager) UpdateRuntimeConfig(ctx context.Context, req *kubeapi.UpdateRuntimeConfigRequest) (*kubeapi.UpdateRuntimeConfigResponse, error) {
	err := s.runtimeService.UpdateRuntimeConfig(req.GetRuntimeConfig())
	if err != nil {
		return nil, err
	}
	return &kubeapi.UpdateRuntimeConfigResponse{}, nil
}

// Status returns the status of the runtime.
func (s *FraktiManager) Status(ctx context.Context, req *kubeapi.StatusRequest) (*kubeapi.StatusResponse, error) {
	resp, err := s.runtimeService.Status()
	if err != nil {
		return nil, err
	}

	return &kubeapi.StatusResponse{
		Status: resp,
	}, nil
}

// ListImages lists existing images.
func (s *FraktiManager) ListImages(ctx context.Context, req *kubeapi.ListImagesRequest) (*kubeapi.ListImagesResponse, error) {
	glog.V(3).Infof("ListImages with request %s", req.String())

	images, err := s.imageService.ListImages(req.GetFilter())
	if err != nil {
		glog.Errorf("ListImages from image service failed: %v", err)
		return nil, err
	}

	return &kubeapi.ListImagesResponse{
		Images: images,
	}, nil
}

// ImageStatus returns the status of the image.
func (s *FraktiManager) ImageStatus(ctx context.Context, req *kubeapi.ImageStatusRequest) (*kubeapi.ImageStatusResponse, error) {
	glog.V(3).Infof("ImageStatus with request %s", req.String())

	status, err := s.imageService.ImageStatus(req.Image)
	if err != nil {
		glog.Infof("ImageStatus from image service failed: %v", err)
		return nil, err
	}
	return &kubeapi.ImageStatusResponse{Image: status}, nil
}

// PullImage pulls a image with authentication config.
func (s *FraktiManager) PullImage(ctx context.Context, req *kubeapi.PullImageRequest) (*kubeapi.PullImageResponse, error) {
	glog.V(3).Infof("PullImage with request %s", req.String())

	imageRef, err := s.imageService.PullImage(req.Image, req.Auth)
	if err != nil {
		glog.Errorf("PullImage from image service failed: %v", err)
		return nil, err
	}

	return &kubeapi.PullImageResponse{
		ImageRef: imageRef,
	}, nil
}

// RemoveImage removes the image.
func (s *FraktiManager) RemoveImage(ctx context.Context, req *kubeapi.RemoveImageRequest) (*kubeapi.RemoveImageResponse, error) {
	glog.V(3).Infof("RemoveImage with request %s", req.String())

	err := s.imageService.RemoveImage(req.Image)
	if err != nil {
		glog.Errorf("RemoveImage from image service failed: %v", err)
		return nil, err
	}

	return &kubeapi.RemoveImageResponse{}, nil
}
