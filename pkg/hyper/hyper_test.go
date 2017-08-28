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
	"testing"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/clock"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func newTestRuntime() (*Runtime, *fakeClientInterface, *clock.FakeClock) {
	fakeClock := clock.NewFakeClock(time.Time{})
	publicClient := newFakeClientInterface(fakeClock)
	client := &Client{
		client: publicClient,
	}
	return &Runtime{
		client: client,
	}, publicClient, fakeClock
}

type fakeCNIPlugin struct {
	sync.Mutex
	name   string
	status error
}

func (f *fakeCNIPlugin) Status() error {
	f.Lock()
	defer f.Unlock()
	return f.status
}

func (f *fakeCNIPlugin) Name() string {
	f.Lock()
	defer f.Unlock()
	return f.name
}

func (f *fakeCNIPlugin) SetUpPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) (cnitypes.Result, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (f *fakeCNIPlugin) TearDownPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) error {
	return fmt.Errorf("Not implemented")
}

func TestVersion(t *testing.T) {
	r, fakeClient, _ := newTestRuntime()
	kubeApiVersion := "kube-v1"
	version, apiVersion := "v1", "api-v1"
	//Set the version
	fakeClient.SetVersion(version, apiVersion)
	expected := &kubeapi.VersionResponse{
		Version:           kubeApiVersion,
		RuntimeName:       hyperRuntimeName,
		RuntimeVersion:    version,
		RuntimeApiVersion: apiVersion,
	}
	//Get the version
	versionEx, err := r.Version(kubeApiVersion)
	assert.NoError(t, err)
	assert.Equal(t, expected, versionEx)
}

func TestStatus(t *testing.T) {
	r, fakeClient, _ := newTestRuntime()
	runtimeStatus := true
	networkStatus := true
	version, apiVersion := "v1", "api-v1"
	//Set the version
	fakeClient.SetVersion(version, apiVersion)
	r.netPlugin = &fakeCNIPlugin{
		status: nil,
	}
	runtimeReady := &kubeapi.RuntimeCondition{
		Type:   kubeapi.RuntimeReady,
		Status: runtimeStatus,
	}
	networkReady := &kubeapi.RuntimeCondition{
		Type:   kubeapi.NetworkReady,
		Status: networkStatus,
	}
	conditions := []*kubeapi.RuntimeCondition{runtimeReady, networkReady}
	expected := &kubeapi.RuntimeStatus{Conditions: conditions}
	//Get the status
	status, err := r.Status()
	assert.NoError(t, err)
	assert.Equal(t, expected, status)
}
