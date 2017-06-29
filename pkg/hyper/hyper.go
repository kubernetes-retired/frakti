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
	"time"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/hyper/ocicni"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	hyperRuntimeName    = "hyper"
	minimumHyperVersion = "0.8.1"
	secondToNano        = 1e9

	// timeout in second for interacting with hyperd's gRPC API.
	hyperConnectionTimeout = 300 * time.Second
)

// Runtime is the HyperContainer implementation of kubelet runtime API
type Runtime struct {
	client            *Client
	streamingServer   streaming.Server
	netPlugin         ocicni.CNIPlugin
	checkpointHandler CheckpointHandler

	defaultCPUNum   int32
	defaultMemoryMB int32
}

// NewHyperRuntime creates a new Runtime
func NewHyperRuntime(hyperEndpoint string, streamingConfig *streaming.Config, cniNetDir, cniPluginDir, rootDir string, defaultCPUNum, defaultMemoryMB int32) (*Runtime, streaming.Server, error) {
	hyperClient, err := NewClient(hyperEndpoint, hyperConnectionTimeout)
	if err != nil {
		glog.Fatalf("Initialize hyper client failed: %v", err)
		return nil, nil, err
	}

	streamingRuntime := &streamingRuntime{client: hyperClient}
	var streamingServer streaming.Server
	if streamingConfig != nil {
		var err error
		streamingServer, err = streaming.NewServer(*streamingConfig, streamingRuntime)
		if err != nil {
			return nil, nil, err
		}
	}

	netPlugin, err := ocicni.InitCNI(cniNetDir, cniPluginDir)
	if err != nil {
		return nil, nil, err
	}
	persistentCheckpointHandler, err := NewPersistentCheckpointHandler(rootDir)
	if err != nil {
		return nil, nil, err
	}

	rt := &Runtime{
		client:            hyperClient,
		streamingServer:   streamingServer,
		netPlugin:         netPlugin,
		checkpointHandler: persistentCheckpointHandler,
		defaultCPUNum:     defaultCPUNum,
		defaultMemoryMB:   defaultMemoryMB,
	}

	return rt, streamingServer, nil
}

// ServiceName method is used to log out with service's name
func (h *Runtime) ServiceName() string {
	return "hyper runtime service"
}

// Version returns the runtime name, runtime version and runtime API version
func (h *Runtime) Version(kubeApiVersion string) (*kubeapi.VersionResponse, error) {
	version, apiVersion, err := h.client.GetVersion()
	if err != nil {
		glog.Errorf("Get hyper version failed: %v", err)
		return nil, err
	}

	return &kubeapi.VersionResponse{
		Version:           kubeApiVersion,
		RuntimeName:       hyperRuntimeName,
		RuntimeVersion:    version,
		RuntimeApiVersion: apiVersion,
	}, nil
}

// Status returns the status of the runtime.
func (h *Runtime) Status() (*kubeapi.RuntimeStatus, error) {
	runtimeReady := &kubeapi.RuntimeCondition{
		Type:   kubeapi.RuntimeReady,
		Status: true,
	}
	var netReady bool
	if err := h.netPlugin.Status(); err == nil {
		netReady = true
	}
	networkReady := &kubeapi.RuntimeCondition{
		Type:   kubeapi.NetworkReady,
		Status: netReady,
	}
	conditions := []*kubeapi.RuntimeCondition{runtimeReady, networkReady}
	if _, _, err := h.client.GetVersion(); err != nil {
		runtimeReady.Status = false
		runtimeReady.Reason = "HyperDaemonNotReady"
		runtimeReady.Message = fmt.Sprintf("hyper: failed to get hyper version: %v", err)
	}

	return &kubeapi.RuntimeStatus{Conditions: conditions}, nil
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (h *Runtime) UpdateRuntimeConfig(runtimeConfig *kubeapi.RuntimeConfig) error {
	return nil
}
