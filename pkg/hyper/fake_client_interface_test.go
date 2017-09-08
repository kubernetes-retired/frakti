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
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/frakti/pkg/hyper/types"
)

// fakeClientInterface mocks the types.PublicAPIClient interface for testing purpose.
type fakeClientInterface struct {
	Clock clock.Clock
	sync.Mutex
	called           []string
	err              error
	containerInfoMap map[string]*types.ContainerInfo
	podInfoMap       map[string]*types.PodInfo
	imageInfoList    []*types.ImageInfo
	version          string
	apiVersion       string
	execCmd          map[string]*[]string
}

func newFakeClientInterface(c clock.Clock) *fakeClientInterface {
	return &fakeClientInterface{
		Clock:            c,
		containerInfoMap: make(map[string]*types.ContainerInfo),
		podInfoMap:       make(map[string]*types.PodInfo),
		execCmd:          make(map[string]*[]string),
	}
}

type FakePod struct {
	PodID     string
	PodName   string
	Status    string
	PodVolume []*types.PodVolume
}

func (f *fakeClientInterface) SetFakePod(pods []*FakePod) {
	f.Lock()
	defer f.Unlock()
	for i := range pods {
		p := pods[i]
		podSpec := types.PodSpec{
			Volumes: p.PodVolume,
		}
		podStatus := types.PodStatus{
			Phase: p.Status,
		}
		podInfo := types.PodInfo{
			Spec:    &podSpec,
			Status:  &podStatus,
			PodName: p.PodName,
		}

		f.podInfoMap[p.PodID] = &podInfo
	}
}

func (f *fakeClientInterface) SetVersion(version string, apiVersion string) {
	f.Lock()
	defer f.Unlock()
	f.version = version
	f.apiVersion = apiVersion
}

type FakeContainer struct {
	ID     string
	Name   string
	Status string
	PodID  string
}

func (f *fakeClientInterface) SetFakeContainers(containers []*FakeContainer) {
	f.Lock()
	defer f.Unlock()
	for i := range containers {
		c := containers[i]
		container := types.Container{
			Name:        c.Name,
			ContainerID: c.ID,
		}
		containerStatus := types.ContainerStatus{
			ContainerID: c.ID,
			Phase:       c.Status,
		}
		containerInfo := types.ContainerInfo{
			Container: &container,
			Status:    &containerStatus,
			PodID:     c.PodID,
		}
		f.containerInfoMap[c.ID] = &containerInfo
	}

}

func (f *fakeClientInterface) CleanCalls() {
	f.Lock()
	defer f.Unlock()
	f.called = nil
}

func (f *fakeClientInterface) PodList(ctx context.Context, in *types.PodListRequest, opts ...grpc.CallOption) (*types.PodListResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "PodList")
	podList := []*types.PodListResult{}
	for _, value := range f.podInfoMap {
		pod := types.PodListResult{
			PodID:   value.PodID,
			PodName: value.PodName,
			Status:  value.Status.Phase,
		}
		podList = append(podList, &pod)
	}
	return &types.PodListResponse{PodList: podList}, f.err
}

func (f *fakeClientInterface) PodCreate(ctx context.Context, in *types.PodCreateRequest, opts ...grpc.CallOption) (*types.PodCreateResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodInfo(ctx context.Context, in *types.PodInfoRequest, opts ...grpc.CallOption) (*types.PodInfoResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "PodInfo")
	PodID := in.PodID
	podInfo, ok := f.podInfoMap[PodID]
	if !ok {
		return nil, fmt.Errorf("pod doesn't existed")
	}
	return &types.PodInfoResponse{PodInfo: podInfo}, f.err
}

func (f *fakeClientInterface) PodRemove(ctx context.Context, in *types.PodRemoveRequest, opts ...grpc.CallOption) (*types.PodRemoveResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "PodRemove")
	delete(f.podInfoMap, in.PodID)
	return &types.PodRemoveResponse{}, f.err
}

func (f *fakeClientInterface) PodStart(ctx context.Context, in *types.PodStartRequest, opts ...grpc.CallOption) (*types.PodStartResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodStop(ctx context.Context, in *types.PodStopRequest, opts ...grpc.CallOption) (*types.PodStopResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodSignal(ctx context.Context, in *types.PodSignalRequest, opts ...grpc.CallOption) (*types.PodSignalResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodPause(ctx context.Context, in *types.PodPauseRequest, opts ...grpc.CallOption) (*types.PodPauseResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodUnpause(ctx context.Context, in *types.PodUnpauseRequest, opts ...grpc.CallOption) (*types.PodUnpauseResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

type fakePublicAPI_ExecVMClient struct {
	grpc.ClientStream
}

func (x *fakePublicAPI_ExecVMClient) Send(m *types.ExecVMRequest) error {
	return nil
}

func (x *fakePublicAPI_ExecVMClient) Recv() (*types.ExecVMResponse, error) {
	m := &types.ExecVMResponse{}
	return m, io.EOF
}

func (x *fakePublicAPI_ExecVMClient) CloseSend() error {
	return nil
}

func (f *fakeClientInterface) ExecVM(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_ExecVMClient, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ExecVM")
	return &fakePublicAPI_ExecVMClient{}, f.err
}

func (f *fakeClientInterface) ContainerList(ctx context.Context, in *types.ContainerListRequest, opts ...grpc.CallOption) (*types.ContainerListResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerList")
	containerList := []*types.ContainerListResult{}
	for _, value := range f.containerInfoMap {
		container := types.ContainerListResult{
			ContainerID:   value.Status.ContainerID,
			ContainerName: value.Container.Name,
			PodID:         value.PodID,
			Status:        value.Status.Phase,
		}
		containerList = append(containerList, &container)
	}
	return &types.ContainerListResponse{ContainerList: containerList}, f.err
}

func (f *fakeClientInterface) ContainerInfo(ctx context.Context, in *types.ContainerInfoRequest, opts ...grpc.CallOption) (*types.ContainerInfoResponse, error) {

	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerInfo")
	containerID := in.Container
	containerInfo, ok := f.containerInfoMap[containerID]
	if !ok {
		return nil, fmt.Errorf("container doesn't existed")
	}
	return &types.ContainerInfoResponse{ContainerInfo: containerInfo}, f.err

}

func (f *fakeClientInterface) ImageList(ctx context.Context, in *types.ImageListRequest, opts ...grpc.CallOption) (*types.ImageListResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ImageList")
	return &types.ImageListResponse{
		ImageList: f.imageInfoList,
	}, f.err
}

func (f *fakeClientInterface) VMList(ctx context.Context, in *types.VMListRequest, opts ...grpc.CallOption) (*types.VMListResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) SetPodLabels(ctx context.Context, in *types.PodLabelsRequest, opts ...grpc.CallOption) (*types.PodLabelsResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) PodStats(ctx context.Context, in *types.PodStatsRequest, opts ...grpc.CallOption) (*types.PodStatsResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ContainerLogs(ctx context.Context, in *types.ContainerLogsRequest, opts ...grpc.CallOption) (types.PublicAPI_ContainerLogsClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ContainerCreate(ctx context.Context, in *types.ContainerCreateRequest, opts ...grpc.CallOption) (*types.ContainerCreateResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerCreate")
	timestamp := f.Clock.Now()

	volumeMounts := []*types.VolumeMount{}
	for i := range in.ContainerSpec.Volumes {
		valumeMount := types.VolumeMount{
			Name:      in.ContainerSpec.Volumes[i].Volume,
			MountPath: in.ContainerSpec.Volumes[i].Path,
		}
		volumeMounts = append(volumeMounts, &valumeMount)
	}
	containerNameSplit := strings.Split(in.ContainerSpec.Name, "_")
	//Create containerID
	containerID := containerNameSplit[len(containerNameSplit)-1]
	container := types.Container{
		Name:         in.ContainerSpec.Name,
		ContainerID:  containerID,
		Labels:       in.ContainerSpec.Labels,
		ImageID:      in.ContainerSpec.Image,
		VolumeMounts: volumeMounts,
	}
	runningStatus := types.RunningStatus{
		StartedAt: dockerTimestampToString(timestamp),
	}
	containerStatus := types.ContainerStatus{
		ContainerID: containerID,
		Phase:       "running",
		Waiting:     nil,
		Running:     &runningStatus,
	}

	containerInfo := types.ContainerInfo{
		Container: &container,
		Status:    &containerStatus,
		PodID:     in.PodID,
	}

	f.containerInfoMap[containerID] = &containerInfo
	return &types.ContainerCreateResponse{ContainerID: containerID}, f.err
}

func (f *fakeClientInterface) ContainerStart(ctx context.Context, in *types.ContainerStartRequest, opts ...grpc.CallOption) (*types.ContainerStartResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerStart")
	containerID := in.ContainerId
	containerInfo, ok := f.containerInfoMap[containerID]
	if !ok {
		return nil, fmt.Errorf("container doesn't existed")
	}
	containerInfo.Status.Phase = "running"
	return &types.ContainerStartResponse{}, f.err
}

func (f *fakeClientInterface) ContainerRename(ctx context.Context, in *types.ContainerRenameRequest, opts ...grpc.CallOption) (*types.ContainerRenameResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ContainerSignal(ctx context.Context, in *types.ContainerSignalRequest, opts ...grpc.CallOption) (*types.ContainerSignalResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ContainerStop(ctx context.Context, in *types.ContainerStopRequest, opts ...grpc.CallOption) (*types.ContainerStopResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerStop")
	containerID := in.ContainerID
	containerInfo, ok := f.containerInfoMap[containerID]
	if !ok {
		return nil, fmt.Errorf("container doesn't existed")
	}
	containerInfo.Status.Phase = "failed"
	startedAt := containerInfo.Status.Running.StartedAt
	timestamp := f.Clock.Now()
	termStatus := types.TermStatus{
		StartedAt:  startedAt,
		FinishedAt: dockerTimestampToString(timestamp),
	}
	containerInfo.Status.Terminated = &termStatus
	return &types.ContainerStopResponse{}, f.err
}

func (f *fakeClientInterface) ContainerRemove(ctx context.Context, in *types.ContainerRemoveRequest, opts ...grpc.CallOption) (*types.ContainerRemoveResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerRemove")
	delete(f.containerInfoMap, in.ContainerId)
	return &types.ContainerRemoveResponse{}, f.err
}

func (f *fakeClientInterface) ExecCreate(ctx context.Context, in *types.ExecCreateRequest, opts ...grpc.CallOption) (*types.ExecCreateResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ExecCreate")
	containerId := in.ContainerID
	f.execCmd[containerId] = &in.Command
	//The container's name is "sidecar" + i,we use i as the execID
	ids := strings.Split(containerId, "*")
	execID := ids[1]
	return &types.ExecCreateResponse{
		ExecID: execID,
	}, f.err
}

type fakePublicAPI_ExecStartClient struct {
	grpc.ClientStream
}

func (x *fakePublicAPI_ExecStartClient) Send(m *types.ExecStartRequest) error {
	return nil
}

func (x *fakePublicAPI_ExecStartClient) Recv() (*types.ExecStartResponse, error) {
	m := &types.ExecStartResponse{}
	return m, io.EOF
}

func (x *fakePublicAPI_ExecStartClient) CloseSend() error {
	return nil
}

func (f *fakeClientInterface) ExecStart(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_ExecStartClient, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ExecStart")
	return &fakePublicAPI_ExecStartClient{}, f.err
}

func (f *fakeClientInterface) ExecSignal(ctx context.Context, in *types.ExecSignalRequest, opts ...grpc.CallOption) (*types.ExecSignalResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

type fakePublicAPI_AttachClient struct {
	grpc.ClientStream
}

func (x *fakePublicAPI_AttachClient) Send(m *types.AttachMessage) error {
	return nil
}

func (x *fakePublicAPI_AttachClient) Recv() (*types.AttachMessage, error) {
	m := &types.AttachMessage{}
	return m, io.EOF
}

func (x *fakePublicAPI_AttachClient) CloseSend() error {
	return nil
}

func (f *fakeClientInterface) Attach(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_AttachClient, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "Attach")
	return &fakePublicAPI_AttachClient{}, f.err
}

func (f *fakeClientInterface) Wait(ctx context.Context, in *types.WaitRequest, opts ...grpc.CallOption) (*types.WaitResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "Wait")
	return &types.WaitResponse{}, f.err
}

func (f *fakeClientInterface) TTYResize(ctx context.Context, in *types.TTYResizeRequest, opts ...grpc.CallOption) (*types.TTYResizeResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "TTYResize")
	return &types.TTYResizeResponse{}, nil
}

func (f *fakeClientInterface) ServiceList(ctx context.Context, in *types.ServiceListRequest, opts ...grpc.CallOption) (*types.ServiceListResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ServiceAdd(ctx context.Context, in *types.ServiceAddRequest, opts ...grpc.CallOption) (*types.ServiceAddResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ServiceDelete(ctx context.Context, in *types.ServiceDelRequest, opts ...grpc.CallOption) (*types.ServiceDelResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ServiceUpdate(ctx context.Context, in *types.ServiceUpdateRequest, opts ...grpc.CallOption) (*types.ServiceUpdateResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

type fakeAPIImagePullClient struct {
	grpc.ClientStream
}

func (x *fakeAPIImagePullClient) Recv() (*types.ImagePullResponse, error) {
	m := &types.ImagePullResponse{}
	//Return "the image data has been read"
	return m, io.EOF
}

func (f *fakeClientInterface) ImagePull(ctx context.Context, in *types.ImagePullRequest, opts ...grpc.CallOption) (types.PublicAPI_ImagePullClient, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ImagePull")
	repoSep := ":"
	id := in.Tag
	if strings.Index(in.Tag, ":") > 0 {
		repoSep = "@"
		str := strings.Split(in.Tag, ":")
		id = str[1]
	}
	repoTags := []string{
		fmt.Sprintf("%s%s%s", in.Image, repoSep, in.Tag),
	}
	imageInfo := &types.ImageInfo{
		Id:          id,
		RepoTags:    repoTags,
		VirtualSize: 0,
	}
	f.imageInfoList = append(f.imageInfoList, imageInfo)
	return &fakeAPIImagePullClient{}, f.err
}

func (f *fakeClientInterface) ImagePush(ctx context.Context, in *types.ImagePushRequest, opts ...grpc.CallOption) (types.PublicAPI_ImagePushClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ImageRemove(ctx context.Context, in *types.ImageRemoveRequest, opts ...grpc.CallOption) (*types.ImageRemoveResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ImageRemove")
	tag := ""
	for i, image := range f.imageInfoList {
		for _, im := range image.RepoTags {
			if im == in.Image {
				tag = f.imageInfoList[i].Id
				//In this test,one imageId has only one tag,while deleting the tag,we also delete the image
				f.imageInfoList = append(f.imageInfoList[:i], f.imageInfoList[i+1:]...)
			}
		}
	}
	imageDelete := &types.ImageDelete{
		Untaged: in.Image,
		Deleted: tag,
	}
	images := []*types.ImageDelete{
		imageDelete,
	}
	return &types.ImageRemoveResponse{Images: images}, f.err
}

func (f *fakeClientInterface) Ping(ctx context.Context, in *types.PingRequest, opts ...grpc.CallOption) (*types.PingResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Info(ctx context.Context, in *types.InfoRequest, opts ...grpc.CallOption) (*types.InfoResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Version(ctx context.Context, in *types.VersionRequest, opts ...grpc.CallOption) (*types.VersionResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "Version")
	return &types.VersionResponse{
		Version:    f.version,
		ApiVersion: f.apiVersion,
	}, f.err
}

// dockerTimestampToString converts the timestamp to string
func dockerTimestampToString(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
