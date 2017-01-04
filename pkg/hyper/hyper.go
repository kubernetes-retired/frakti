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
	"github.com/golang/protobuf/proto"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

const (
	hyperRuntimeName    = "hyper"
	minimumHyperVersion = "0.6.0"
	secondToNano        = 1e9

	// timeout in second for interacting with hyperd's gRPC API.
	hyperConnectionTimeout = 300 * time.Second
)

// Runtime is the HyperContainer implementation of kubelet runtime API
type Runtime struct {
	client          *Client
	streamingServer streaming.Server
	networkPlugin   *CNINetworkPlugin
}

// NewHyperRuntime creates a new Runtime
func NewHyperRuntime(hyperEndpoint string, streamingConfig *streaming.Config) (*Runtime, streaming.Server, error) {
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

	networkPlugin, err := InitCNI()
	if err != nil {
		return nil, nil, err
	}

	return &Runtime{client: hyperClient, streamingServer: streamingServer, networkPlugin: networkPlugin}, streamingServer, nil
}

// Version returns the runtime name, runtime version and runtime API version
func (h *Runtime) Version() (string, string, string, error) {
	version, apiVersion, err := h.client.GetVersion()
	if err != nil {
		glog.Errorf("Get hyper version failed: %v", err)
		return "", "", "", err
	}

	return hyperRuntimeName, version, apiVersion, nil
}

// Status returns the status of the runtime.
func (h *Runtime) Status() (*kubeapi.RuntimeStatus, error) {
	runtimeReady := &kubeapi.RuntimeCondition{
		Type:   proto.String(kubeapi.RuntimeReady),
		Status: proto.Bool(true),
	}
	// Always set networkReady for now.
	// TODO: get real network status when network plugin is enabled.
	networkReady := &kubeapi.RuntimeCondition{
		Type:   proto.String(kubeapi.NetworkReady),
		Status: proto.Bool(true),
	}
	conditions := []*kubeapi.RuntimeCondition{runtimeReady, networkReady}
	if _, _, err := h.client.GetVersion(); err != nil {
		runtimeReady.Status = proto.Bool(false)
		runtimeReady.Reason = proto.String("HyperDaemonNotReady")
		runtimeReady.Message = proto.String(fmt.Sprintf("hyper: failed to get hyper version: %v", err))
	}

	return &kubeapi.RuntimeStatus{Conditions: conditions}, nil
}

// UpdateRuntimeConfig updates runtime configuration if specified
func (h *Runtime) UpdateRuntimeConfig(runtimeConfig *kubeapi.RuntimeConfig) error {
	return nil
}
