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
	"net/url"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/frakti/pkg/runtime"
	unikernelimage "k8s.io/frakti/pkg/unikernel/image"
	"k8s.io/frakti/pkg/util/alternativeruntime"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	utilexec "k8s.io/utils/exec"
)

const (
	runtimeAPIVersion = "0.1.0"

	// TODO(resouer) move this to well-known labels on k8s upstream?

	// OSContainerAnnotationKey specifying this pod will run by OS container runtime.
	OSContainerAnnotationKey = "runtime.frakti.alpha.kubernetes.io/OSContainer"
	// OSContainerAnnotationTrue specifying this pod will run by OS container runtime.
	OSContainerAnnotationTrue = "true"
	// UnikernelAnnotationKey specifying this pod will run by unikernel runtime.
	UnikernelAnnotationKey = "runtime.frakti.alpha.kubernetes.io/Unikernel"
	// UnikernelAnnotationTrue specifying this pod will run by unikernel runtime.
	UnikernelAnnotationTrue = "true"
)

// FraktiManager serves the kubelet runtime gRPC api which will be
// consumed by kubelet
type FraktiManager struct {
	// The grpc server.
	server *grpc.Server
	// The streaming server.
	streamingServer streaming.Server

	hyperRuntimeService runtime.RuntimeService
	hyperImageService   runtime.ImageManagerService

	privilegedRuntimeService runtime.RuntimeService
	privilegedImageService   runtime.ImageManagerService

	unikernelRuntimeService runtime.RuntimeService
	unikernelImageService   runtime.ImageManagerService

	// The pod sets need to be processed by privileged runtime
	cachedAlternativeRuntimeItems *alternativeruntime.AlternativeRuntimeSets
}

// NewFraktiManager creates a new FraktiManager
func NewFraktiManager(
	hyperRuntimeService runtime.RuntimeService,
	hyperImageService runtime.ImageManagerService,
	streamingServer streaming.Server,
	privilegedRuntimeService runtime.RuntimeService,
	privilegedImageService runtime.ImageManagerService,
	unikernelRuntimeService runtime.RuntimeService,
	unikernelImageService runtime.ImageManagerService,
) (*FraktiManager, error) {
	s := &FraktiManager{
		server:                        grpc.NewServer(),
		hyperRuntimeService:           hyperRuntimeService,
		hyperImageService:             hyperImageService,
		streamingServer:               streamingServer,
		privilegedRuntimeService:      privilegedRuntimeService,
		privilegedImageService:        privilegedImageService,
		unikernelRuntimeService:       unikernelRuntimeService,
		unikernelImageService:         unikernelImageService,
		cachedAlternativeRuntimeItems: alternativeruntime.NewAlternativeRuntimeSets(),
	}
	// NOTE: Check the real value of interface, see https://golang.org/doc/faq#nil_error
	if privilegedRuntimeService == nil || reflect.ValueOf(privilegedRuntimeService).IsNil() {
		s.privilegedRuntimeService = nil
		s.privilegedImageService = nil
	}
	if unikernelRuntimeService == nil || reflect.ValueOf(unikernelRuntimeService).IsNil() {
		s.unikernelRuntimeService = nil
		s.unikernelImageService = nil
	}
	for _, runtimeService := range []runtime.RuntimeService{s.privilegedRuntimeService, s.unikernelRuntimeService} {
		if runtimeService != nil {
			runtimeName := runtimeService.ServiceName()
			sandboxes, err := runtimeService.ListPodSandbox(nil)
			if err != nil {
				glog.Errorf("Failed to initialize frakti manager: ListPodSandbox from %s service failed: %v", runtimeName, err)
				return nil, err
			}
			containers, err := runtimeService.ListContainers(nil)
			if err != nil {
				glog.Errorf("Failed to initialize frakti manager: ListContainers from %s service failed: %v", runtimeName, err)
				return nil, err
			}
			for _, sandbox := range sandboxes {
				s.cachedAlternativeRuntimeItems.Add(sandbox.Id, runtimeName)
			}
			for _, container := range containers {
				s.cachedAlternativeRuntimeItems.Add(container.Id, runtimeName)
			}
			glog.Infof("Restored %s managed sandboxes and containers to cache", runtimeName)
		}
	}
	s.registerServer()

	return s, nil
}

// getRuntimeService returns corresponding runtime service based on given sandbox or container ID
func (s *FraktiManager) getRuntimeService(id string) (runtime.RuntimeService, runtime.ImageManagerService) {
	runtimeName := s.cachedAlternativeRuntimeItems.GetRuntime(id)
	switch runtimeName {
	case alternativeruntime.PrivilegedRuntimeName:
		return s.privilegedRuntimeService, s.privilegedImageService
	case alternativeruntime.UnikernelRuntimeName:
		return s.unikernelRuntimeService, s.unikernelImageService
	default:
		return s.hyperRuntimeService, s.hyperImageService
	}
}

// getEnabledRuntimeService get all enabled runtime services in FraktiManager
func (s *FraktiManager) getEnabledRuntimeService() []runtime.RuntimeService {
	runtimeServices := []runtime.RuntimeService{}
	if s.hyperRuntimeService != nil {
		runtimeServices = append(runtimeServices, s.hyperRuntimeService)
	}
	if s.privilegedRuntimeService != nil {
		runtimeServices = append(runtimeServices, s.privilegedRuntimeService)
	}
	if s.unikernelRuntimeService != nil {
		runtimeServices = append(runtimeServices, s.unikernelRuntimeService)
	}
	return runtimeServices
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
	// Version API use hyper runtime service
	resp, err := s.hyperRuntimeService.Version(runtimeAPIVersion)
	if err != nil {
		glog.Errorf("Get version from runtime service failed: %v", err)
		return nil, err
	}

	return resp, nil
}

// RunPodSandbox creates and start a hyper Pod.
func (s *FraktiManager) RunPodSandbox(ctx context.Context, req *kubeapi.RunPodSandboxRequest) (*kubeapi.RunPodSandboxResponse, error) {
	glog.V(3).Infof("RunPodSandbox from runtime service with request %s", req.String())

	runtimeService := s.getRuntimeServiceBySandboxConfig(req.GetConfig())
	runtimeName := runtimeService.ServiceName()
	podID, err := runtimeService.RunPodSandbox(req.Config)
	if err != nil {
		glog.Errorf("RunPodSandbox from %s failed: %v", runtimeName, err)
		return nil, err
	}

	if runtimeService != s.hyperRuntimeService {
		s.cachedAlternativeRuntimeItems.Add(podID, runtimeName)
	}
	return &kubeapi.RunPodSandboxResponse{PodSandboxId: podID}, nil
}

// StopPodSandbox stops the sandbox.
func (s *FraktiManager) StopPodSandbox(ctx context.Context, req *kubeapi.StopPodSandboxRequest) (*kubeapi.StopPodSandboxResponse, error) {
	glog.V(3).Infof("StopPodSandbox from runtime service with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.PodSandboxId)
	err := runtimeService.StopPodSandbox(req.PodSandboxId)
	if err != nil {
		glog.Errorf("StopPodSandbox from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}

	return &kubeapi.StopPodSandboxResponse{}, nil
}

// RemovePodSandbox deletes the sandbox.
func (s *FraktiManager) RemovePodSandbox(ctx context.Context, req *kubeapi.RemovePodSandboxRequest) (*kubeapi.RemovePodSandboxResponse, error) {
	glog.V(3).Infof("RemovePodSandbox from runtime service with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.PodSandboxId)
	err := runtimeService.RemovePodSandbox(req.PodSandboxId)
	if err != nil {
		glog.Errorf("RemovePodSandbox from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}
	if runtimeService != s.hyperRuntimeService {
		s.cachedAlternativeRuntimeItems.Remove(req.PodSandboxId, runtimeService.ServiceName())
	}
	return &kubeapi.RemovePodSandboxResponse{}, nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (s *FraktiManager) PodSandboxStatus(ctx context.Context, req *kubeapi.PodSandboxStatusRequest) (*kubeapi.PodSandboxStatusResponse, error) {
	glog.V(3).Infof("PodSandboxStatus with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.PodSandboxId)
	podStatus, err := runtimeService.PodSandboxStatus(req.PodSandboxId)
	if err != nil {
		glog.Errorf("PodSandboxStatus from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}
	return &kubeapi.PodSandboxStatusResponse{Status: podStatus}, nil
}

// ListPodSandbox returns a list of SandBox.
func (s *FraktiManager) ListPodSandbox(ctx context.Context, req *kubeapi.ListPodSandboxRequest) (*kubeapi.ListPodSandboxResponse, error) {
	glog.V(3).Infof("ListPodSandbox with request %s", req.String())

	var items []*kubeapi.PodSandbox
	for _, runtimeService := range s.getEnabledRuntimeService() {
		podsInRuntime, err := runtimeService.ListPodSandbox(req.GetFilter())
		if err != nil {
			glog.Errorf("ListPodSandbox from  %s runtime service failed: %v", runtimeService.ServiceName(), err)
			return nil, err
		}
		items = append(items, podsInRuntime...)
	}

	return &kubeapi.ListPodSandboxResponse{Items: items}, nil
}

// CreateContainer creates a new container in specified PodSandbox
func (s *FraktiManager) CreateContainer(ctx context.Context, req *kubeapi.CreateContainerRequest) (*kubeapi.CreateContainerResponse, error) {
	glog.V(3).Infof("CreateContainer with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.PodSandboxId)
	containerID, err := runtimeService.CreateContainer(req.PodSandboxId, req.Config, req.SandboxConfig)
	runtimeName := runtimeService.ServiceName()

	if err != nil {
		glog.Errorf("CreateContainer from %s failed: %v", runtimeName, err)
		return nil, err
	}
	if s.cachedAlternativeRuntimeItems.Has(req.PodSandboxId, runtimeName) {
		s.cachedAlternativeRuntimeItems.Add(containerID, runtimeName)
		glog.V(3).Infof("added container: %s to %s container sets", containerID, runtimeName)
	}
	return &kubeapi.CreateContainerResponse{ContainerId: containerID}, nil
}

// StartContainer starts the container.
func (s *FraktiManager) StartContainer(ctx context.Context, req *kubeapi.StartContainerRequest) (*kubeapi.StartContainerResponse, error) {
	glog.V(3).Infof("StartContainer with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	err := runtimeService.StartContainer(req.ContainerId)
	if err != nil {
		glog.Errorf("StartContainer from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}
	return &kubeapi.StartContainerResponse{}, nil
}

// StopContainer stops a running container with a grace period (i.e. timeout).
func (s *FraktiManager) StopContainer(ctx context.Context, req *kubeapi.StopContainerRequest) (*kubeapi.StopContainerResponse, error) {
	glog.V(3).Infof("StopContainer with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	err := runtimeService.StopContainer(req.ContainerId, req.Timeout)
	if err != nil {
		glog.Errorf("StopContainer from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}
	return &kubeapi.StopContainerResponse{}, nil
}

// RemoveContainer removes the container.
func (s *FraktiManager) RemoveContainer(ctx context.Context, req *kubeapi.RemoveContainerRequest) (*kubeapi.RemoveContainerResponse, error) {
	glog.V(3).Infof("RemoveContainer with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	runtimeName := runtimeService.ServiceName()
	err := runtimeService.RemoveContainer(req.ContainerId)
	if err != nil {
		glog.Errorf("RemoveContainer from %s failed: %v", runtimeName, err)
		return nil, err
	}
	if runtimeService != s.hyperRuntimeService {
		s.cachedAlternativeRuntimeItems.Remove(req.ContainerId, runtimeName)
		glog.V(3).Infof("removed container: %s from %s container sets", req.ContainerId, runtimeName)

	}
	return &kubeapi.RemoveContainerResponse{}, nil
}

// ListContainers lists all containers by filters.
func (s *FraktiManager) ListContainers(ctx context.Context, req *kubeapi.ListContainersRequest) (*kubeapi.ListContainersResponse, error) {
	glog.V(3).Infof("ListContainers with request %s", req.String())

	var containers []*kubeapi.Container
	for _, runtimeService := range s.getEnabledRuntimeService() {
		runtimeContainers, err := runtimeService.ListContainers(req.GetFilter())
		if err != nil {
			glog.Errorf("ListContainers from %s runtime service failed: %v", runtimeService.ServiceName(), err)
			return nil, err
		}
		containers = append(containers, runtimeContainers...)
	}

	return &kubeapi.ListContainersResponse{
		Containers: containers,
	}, nil
}

// ContainerStatus returns the container status.
func (s *FraktiManager) ContainerStatus(ctx context.Context, req *kubeapi.ContainerStatusRequest) (*kubeapi.ContainerStatusResponse, error) {
	glog.V(3).Infof("ContainerStatus with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	kubeStatus, err := runtimeService.ContainerStatus(req.ContainerId)
	if err != nil {
		glog.Errorf("ContainerStatus from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}

	return &kubeapi.ContainerStatusResponse{
		Status: kubeStatus,
	}, nil
}

// UpdateContainerResources updates ContainerConfig of the container
func (s *FraktiManager) UpdateContainerResources(
	ctx context.Context,
	req *kubeapi.UpdateContainerResourcesRequest,
) (*kubeapi.UpdateContainerResourcesResponse, error) {
	glog.V(3).Infof("UpdateContainerResources with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	if err := runtimeService.UpdateContainerResources(
		req.GetContainerId(),
		req.GetLinux(),
	); err != nil {
		return nil, err
	}

	return &kubeapi.UpdateContainerResourcesResponse{}, nil
}

// ExecSync runs a command in a container synchronously.
func (s *FraktiManager) ExecSync(ctx context.Context, req *kubeapi.ExecSyncRequest) (*kubeapi.ExecSyncResponse, error) {
	glog.V(3).Infof("ExecSync with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	stdout, stderr, err := runtimeService.ExecSync(req.ContainerId, req.Cmd, time.Duration(req.Timeout)*time.Second)
	var exitCode int32
	if err != nil {
		exitError, ok := err.(utilexec.ExitError)
		if !ok {
			glog.Errorf("ExecSync from %s failed: %v", runtimeService.ServiceName(), err)
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

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	resp, err := runtimeService.Exec(req)

	if err != nil {
		glog.Errorf("Exec from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}

	return resp, nil
}

// Attach prepares a streaming endpoint to attach to a running container.
func (s *FraktiManager) Attach(ctx context.Context, req *kubeapi.AttachRequest) (*kubeapi.AttachResponse, error) {
	glog.V(3).Infof("Attach with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.ContainerId)
	resp, err := runtimeService.Attach(req)
	if err != nil {
		glog.Errorf("Attach from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}

	return resp, nil
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox.
func (s *FraktiManager) PortForward(ctx context.Context, req *kubeapi.PortForwardRequest) (*kubeapi.PortForwardResponse, error) {
	glog.V(3).Infof("PortForward with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.PodSandboxId)
	resp, err := runtimeService.PortForward(req)
	if err != nil {
		glog.Errorf("PortForward from %s failed: %v", runtimeService.ServiceName(), err)
		return nil, err
	}
	return resp, nil
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (s *FraktiManager) UpdateRuntimeConfig(ctx context.Context, req *kubeapi.UpdateRuntimeConfigRequest) (*kubeapi.UpdateRuntimeConfigResponse, error) {
	glog.V(3).Infof("Update hyper runtime configure with request %s", req.String())
	// TODO(resouer) only for hyper runtime update, so we cannot deal with handles podCIDR updates in docker.
	err := s.hyperRuntimeService.UpdateRuntimeConfig(req.GetRuntimeConfig())
	if err != nil {
		return nil, err
	}
	return &kubeapi.UpdateRuntimeConfigResponse{}, nil
}

// Status returns the status of the runtime.
func (s *FraktiManager) Status(ctx context.Context, req *kubeapi.StatusRequest) (*kubeapi.StatusResponse, error) {
	glog.V(3).Infof("Status hyper runtime service with request %s", req.String())
	var (
		resp *kubeapi.RuntimeStatus
		err  error
	)
	resp, err = s.hyperRuntimeService.Status()
	if err != nil {
		return nil, err
	}

	if s.privilegedRuntimeService != nil {
		privilegedResp, err := s.privilegedRuntimeService.Status()
		if err != nil {
			return nil, fmt.Errorf("Status request succeed for hyper, but failed for privileged runtime: %v", err)
		}
		glog.V(3).Infof("Status of privileged runtime service is %v", privilegedResp)
	}

	return &kubeapi.StatusResponse{
		Status: resp,
	}, nil
}

// ListImages lists existing images.
func (s *FraktiManager) ListImages(ctx context.Context, req *kubeapi.ListImagesRequest) (*kubeapi.ListImagesResponse, error) {
	glog.V(3).Infof("ListImages with request %s", req.String())

	errs := []error{}

	// NOTE: The following steps assume 'imageServiceList' and 'imageMapList' have corresponding order
	imageServiceList := []runtime.ImageManagerService{s.hyperImageService, s.privilegedImageService}
	workerNum := 2
	if s.unikernelImageService != nil {
		imageServiceList = append(imageServiceList, s.unikernelImageService)
		workerNum++
	}
	imageMapList := make([]map[string]*kubeapi.Image, workerNum)

	listImageFunc := func(i int) {
		images, err := imageServiceList[i].ListImages(req.GetFilter())
		if err != nil {
			errs = append(errs, fmt.Errorf("ListImage from %s failed: %v", imageServiceList[i].ServiceName(), err))
			return
		}
		imageMapList[i] = make(map[string]*kubeapi.Image, len(images))
		for _, image := range images {
			imageMapList[i][image.Id] = image
		}
	}

	workqueue.Parallelize(workerNum, workerNum, listImageFunc)

	if len(errs) > 0 {
		glog.Error(errs[0])
		return nil, errs[0]
	}

	// NOTE: we show intersection of image list of hyper and privileged runtime
	intersectList := getImageListIntersection(imageMapList[0], imageMapList[1])

	// if there is different in two sides, print the different if log lever is high enough
	if glog.V(5) && len(imageMapList[0]) != len(intersectList) {
		glog.Infof("Image black hole in %s:\n%v", imageServiceList[0].ServiceName(), getImageListDifference(imageMapList[0], imageMapList[1]))
		glog.Infof("Image black hole in %s:\n%v", imageServiceList[1].ServiceName(), getImageListDifference(imageMapList[1], imageMapList[0]))
	}

	// Append unikernl image list at last
	// NOTE: Here we assume all unikernel images never overlap with hyper/docker's image,
	// so we just append unikernel images to the list.
	if s.unikernelImageService != nil {
		for _, v := range imageMapList[2] {
			intersectList = append(intersectList, v)
		}
	}

	return &kubeapi.ListImagesResponse{
		Images: intersectList,
	}, nil
}

// ImageStatus returns the status of the image.
func (s *FraktiManager) ImageStatus(ctx context.Context, req *kubeapi.ImageStatusRequest) (*kubeapi.ImageStatusResponse, error) {
	glog.V(3).Infof("ImageStatus with request %s", req.String())

	var (
		status *kubeapi.Image
		err    error
	)
	if s.unikernelImageService != nil && isUnikernelRuntimeImage(req.GetImage().GetImage()) {
		status, err = s.unikernelImageService.ImageStatus(req.GetImage())
		if err != nil {
			return nil, fmt.Errorf("ImageStatus from unikernel image service failed: %v", err)
		}
	} else {
		// NOTE: we only show image status of hyper runtime and assume privileged runtime image are the same
		status, err = s.hyperImageService.ImageStatus(req.Image)
		if err != nil {
			glog.Errorf("ImageStatus from hyper image service failed: %v", err)
			return nil, err
		}
	}
	return &kubeapi.ImageStatusResponse{Image: status}, nil
}

// PullImage pulls a image with authentication config.
func (s *FraktiManager) PullImage(ctx context.Context, req *kubeapi.PullImageRequest) (*kubeapi.PullImageResponse, error) {
	glog.V(3).Infof("PullImage with request %s", req.String())

	var (
		imageRef string
		err      error
	)
	if s.unikernelImageService != nil && isUnikernelRuntimeImage(req.GetImage().GetImage()) {
		imageRef, err = s.unikernelImageService.PullImage(req.Image, req.Auth)
		if err != nil {
			return nil, fmt.Errorf("PullImage from unikernel image service failed: %v", err)
		}
	} else {
		images := []string{}
		errs := []error{}
		pullImageFunc := func(i int) {
			if i == 0 {
				imageRef, err = s.hyperImageService.PullImage(req.Image, req.Auth)
				if err != nil {
					errs = append(errs, fmt.Errorf("PullImage from hyper image service failed: %v", err))
				}
				images = append(images, imageRef)
			} else {
				imageRef, err = s.privilegedImageService.PullImage(req.Image, req.Auth)
				if err != nil {
					errs = append(errs, fmt.Errorf("PullImage from privileged image service failed: %v", err))
				}
				images = append(images, imageRef)
			}
		}

		workqueue.Parallelize(2, 2, pullImageFunc)

		if len(errs) > 0 || len(images) == 0 {
			glog.Error(errs[0])
			return nil, errs[0]
		}
		imageRef = images[0]
	}

	return &kubeapi.PullImageResponse{
		ImageRef: imageRef,
	}, nil
}

// RemoveImage removes the image.
func (s *FraktiManager) RemoveImage(ctx context.Context, req *kubeapi.RemoveImageRequest) (*kubeapi.RemoveImageResponse, error) {
	glog.V(3).Infof("RemoveImage with request %s", req.String())

	if s.unikernelImageService != nil && isUnikernelRuntimeImage(req.GetImage().GetImage()) {
		err := s.unikernelImageService.RemoveImage(req.GetImage())
		if err != nil {
			return nil, fmt.Errorf("RemoveImage from unikernel image service failed: %v", err)
		}
	} else {
		errs := []error{}

		imageServiceList := []runtime.ImageManagerService{s.hyperImageService, s.privilegedImageService}

		removeImageFunc := func(i int) {
			err := imageServiceList[i].RemoveImage(req.Image)
			if err != nil {
				errs = append(errs, fmt.Errorf("RemoveImage from %s failed: %v", imageServiceList[i].ServiceName(), err))
			}
		}

		workqueue.Parallelize(2, 2, removeImageFunc)

		if len(errs) > 0 {
			glog.Error(errs[0])
			return nil, errs[0]
		}
	}

	return &kubeapi.RemoveImageResponse{}, nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *FraktiManager) ImageFsInfo(ctx context.Context, req *kubeapi.ImageFsInfoRequest) (*kubeapi.ImageFsInfoResponse, error) {
	glog.V(3).Infof("ImageFsInfo with request %s", req.String())
	return nil, fmt.Errorf("not implemented")
}

// ContainerStats returns information of the container filesystem.
func (s *FraktiManager) ContainerStats(ctx context.Context, req *kubeapi.ContainerStatsRequest) (*kubeapi.ContainerStatsResponse, error) {
	glog.V(3).Infof("ContainerStats with request %s", req.String())
	runtimeService, _ := s.getRuntimeService(req.GetContainerId())

	stats, err := runtimeService.ContainerStats(req.GetContainerId())
	if err != nil {
		return nil, err
	}

	return &kubeapi.ContainerStatsResponse{
		Stats: stats,
	}, nil
}

// ListContainerStats returns stats of all running containers
func (s *FraktiManager) ListContainerStats(ctx context.Context, req *kubeapi.ListContainerStatsRequest) (*kubeapi.ListContainerStatsResponse, error) {
	glog.V(3).Infof("ListContainerStats with request %s", req.String())

	runtimeService, _ := s.getRuntimeService(req.GetFilter().GetPodSandboxId())

	statsList, err := runtimeService.ListContainerStats(req.GetFilter())
	if err != nil {
		return nil, err
	}

	return &kubeapi.ListContainerStatsResponse{
		Stats: statsList,
	}, nil
}

// getRuntimeServiceBySandboxConfig returns corresponding runtime service by sandbox config
func (s *FraktiManager) getRuntimeServiceBySandboxConfig(podConfig *kubeapi.PodSandboxConfig) runtime.RuntimeService {
	if isOSContainerRuntimeRequired(podConfig) {
		return s.privilegedRuntimeService
	}
	if s.unikernelRuntimeService != nil && isUnikernelRuntimeRequiredBySandbox(podConfig) {
		return s.unikernelRuntimeService
	}
	return s.hyperRuntimeService
}

// isOSContainerRuntimeRequired check if this pod requires to run with os container runtime.
func isOSContainerRuntimeRequired(podConfig *kubeapi.PodSandboxConfig) bool {
	// user require it
	if annotations := podConfig.GetAnnotations(); annotations != nil {
		if useOSContainer := annotations[OSContainerAnnotationKey]; useOSContainer == OSContainerAnnotationTrue {
			return true
		}
	}

	// privileged container required
	if securityContext := podConfig.GetLinux().GetSecurityContext(); securityContext != nil {
		if !securityContext.Privileged {
			// use host namespace
			if nsOptions := securityContext.GetNamespaceOptions(); nsOptions != nil {
				if nsOptions.HostIpc || nsOptions.HostNetwork || nsOptions.HostPid {
					return true
				}
			}
		} else {
			return true
		}
	}

	return false
}

// isUnikernelRuntimeRequiredBySandbox check if this pod config requires to run with unikernel runtime.
func isUnikernelRuntimeRequiredBySandbox(podConfig *kubeapi.PodSandboxConfig) bool {
	// user required it
	if annotations := podConfig.GetAnnotations(); annotations != nil {
		if useUnikernel := annotations[UnikernelAnnotationKey]; useUnikernel == UnikernelAnnotationTrue {
			return true
		}
	}
	return false
}

// getImageListIntersection return the intersection of images in mapA and mapB
func getImageListIntersection(mapA, mapB map[string]*kubeapi.Image) []*kubeapi.Image {
	var result []*kubeapi.Image
	intersecIDList := sets.StringKeySet(mapA).Intersection(sets.StringKeySet(mapB)).UnsortedList()
	for _, imageID := range intersecIDList {
		kubeImage := &kubeapi.Image{
			Id:          imageID,
			RepoTags:    sets.NewString(mapA[imageID].RepoTags...).Intersection(sets.NewString(mapB[imageID].RepoTags...)).UnsortedList(),
			RepoDigests: sets.NewString(mapA[imageID].RepoDigests...).Intersection(sets.NewString(mapB[imageID].RepoDigests...)).UnsortedList(),
			Size_:       mapA[imageID].Size_,
			Uid:         mapA[imageID].Uid,
			Username:    mapA[imageID].Username,
		}
		result = append(result, kubeImage)
	}
	return result
}

// getImageListIntersection return the difference of images in mapA from mapB
func getImageListDifference(mapA, mapB map[string]*kubeapi.Image) []*kubeapi.Image {
	var diffList []*kubeapi.Image
	diff := sets.StringKeySet(mapA).Difference(sets.StringKeySet(mapB)).UnsortedList()
	for _, i := range diff {
		diffList = append(diffList, mapA[i])
	}
	return diffList
}

// isUnikernelRuntimeImage check image reference and return whether this image is unikernel image
// TODO(Crazykev): Even after this, we may also wrongly consider a docker image as unikernel image. ie. 'unikernel/busybox.com:latest'
func isUnikernelRuntimeImage(imageRef string) bool {
	if strings.HasPrefix(imageRef, unikernelimage.UnikernelImagePrefix) {
		imageRef = imageRef[len(unikernelimage.UnikernelImagePrefix) : len(imageRef)-1]
		// When we specific an unikernel image, kubelet will try to use
		// docker image format to parse image reference, most of time, docker will regard
		// this image reference(repo:tag) lack of image tag, and add default tag ':latest'.
		// Remove this default tag to get what user specified in pod spec.
		defaultImageSuffix := ":latest"
		if strings.HasSuffix(imageRef, defaultImageSuffix) {
			imageRef = imageRef[0 : len(imageRef)-len(defaultImageSuffix)]
		}
		// Try to parse it as url, it's ok there is no scheme here.
		if _, err := url.Parse(imageRef); err == nil {
			return true
		}
	}
	return false
}
