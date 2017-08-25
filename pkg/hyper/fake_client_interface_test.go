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
}

func newFakeClientInterface(c clock.Clock) *fakeClientInterface {
	return &fakeClientInterface{
		Clock:            c,
		containerInfoMap: make(map[string]*types.ContainerInfo),
		podInfoMap:       make(map[string]*types.PodInfo),
	}
}

type FakePod struct {
	PodID     string
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
		podInfo := types.PodInfo{
			Spec: &podSpec,
		}

		f.podInfoMap[p.PodID] = &podInfo
	}
}

func (f *fakeClientInterface) CleanCalls() {
	f.Lock()
	defer f.Unlock()
	f.called = nil
}

func (f *fakeClientInterface) PodList(ctx context.Context, in *types.PodListRequest, opts ...grpc.CallOption) (*types.PodListResponse, error) {
	return nil, fmt.Errorf("Not implemented")
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
	return nil, fmt.Errorf("Not implemented")
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

func (f *fakeClientInterface) ExecVM(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_ExecVMClient, error) {
	return nil, fmt.Errorf("Not implemented")
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
	return nil, fmt.Errorf("Not implemented")
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
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ExecStart(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_ExecStartClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ExecSignal(ctx context.Context, in *types.ExecSignalRequest, opts ...grpc.CallOption) (*types.ExecSignalResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Attach(ctx context.Context, opts ...grpc.CallOption) (types.PublicAPI_AttachClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Wait(ctx context.Context, in *types.WaitRequest, opts ...grpc.CallOption) (*types.WaitResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) TTYResize(ctx context.Context, in *types.TTYResizeRequest, opts ...grpc.CallOption) (*types.TTYResizeResponse, error) {
	return nil, fmt.Errorf("Not implemented")
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

func (f *fakeClientInterface) ImagePull(ctx context.Context, in *types.ImagePullRequest, opts ...grpc.CallOption) (types.PublicAPI_ImagePullClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ImagePush(ctx context.Context, in *types.ImagePushRequest, opts ...grpc.CallOption) (types.PublicAPI_ImagePushClient, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ImageRemove(ctx context.Context, in *types.ImageRemoveRequest, opts ...grpc.CallOption) (*types.ImageRemoveResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Ping(ctx context.Context, in *types.PingRequest, opts ...grpc.CallOption) (*types.PingResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Info(ctx context.Context, in *types.InfoRequest, opts ...grpc.CallOption) (*types.InfoResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) Version(ctx context.Context, in *types.VersionRequest, opts ...grpc.CallOption) (*types.VersionResponse, error) {
	return nil, fmt.Errorf("Not implemented")
}

// dockerTimestampToString converts the timestamp to string
func dockerTimestampToString(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
