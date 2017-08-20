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
	"sync"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/frakti/pkg/hyper/types"
)

// fakeClientInterface mocks the types.PublicAPIClient interface for testing purpose.
type fakeClientInterface struct {
	sync.Mutex
	called        []string
	err           error
	containerInfo types.ContainerInfo
	containerList []*types.ContainerListResult
	podInfo       types.PodInfo
}

func newFakeClientInterface() *fakeClientInterface {
	return &fakeClientInterface{}
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

	return &types.PodInfoResponse{PodInfo: &f.podInfo}, f.err
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
	return &types.ContainerListResponse{ContainerList: f.containerList}, f.err
}

func (f *fakeClientInterface) ContainerInfo(ctx context.Context, in *types.ContainerInfoRequest, opts ...grpc.CallOption) (*types.ContainerInfoResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerInfo")
	//the following 'if' is used to TestListContainer
	if len(f.containerList) != 0 {
		for _, container := range f.containerList {
			if container.ContainerID == in.Container {
				containerStatus := types.ContainerStatus{
					Phase: container.Status,
				}
				f.containerInfo.Status = &containerStatus
			}
		}
	}
	return &types.ContainerInfoResponse{ContainerInfo: &f.containerInfo}, f.err
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
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeClientInterface) ContainerStart(ctx context.Context, in *types.ContainerStartRequest, opts ...grpc.CallOption) (*types.ContainerStartResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerStart")
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
	return &types.ContainerStopResponse{}, f.err
}

func (f *fakeClientInterface) ContainerRemove(ctx context.Context, in *types.ContainerRemoveRequest, opts ...grpc.CallOption) (*types.ContainerRemoveResponse, error) {
	f.Lock()
	defer f.Unlock()
	f.called = append(f.called, "ContainerRemove")
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
