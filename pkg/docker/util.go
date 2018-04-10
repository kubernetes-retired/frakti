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

package docker

import (
	"fmt"
	grpc "google.golang.org/grpc"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
	"time"
)

const (
	fraktiDockerShim = "unix:///var/run/dockershim.sock"
	defaultTimeout   = 10 * time.Second
)

var runtimeClient kubeapi.RuntimeServiceClient
var imageClient kubeapi.ImageServiceClient
var conn *grpc.ClientConn

func getRuntimeClient() error {
	// Set up a connection to the server.
	var err error
	conn, err = getRuntimeClientConnection()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	runtimeClient = kubeapi.NewRuntimeServiceClient(conn)
	return nil
}

func getImageClient() error {
	// Set up a connection to the server.
	var err error
	conn, err = getImageClientConnection()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	imageClient = kubeapi.NewImageServiceClient(conn)
	return nil
}

func closeConnection() error {
	if conn == nil {
		return nil
	}

	return conn.Close()
}

func getRuntimeClientConnection() (*grpc.ClientConn, error) {

	addr, dialer, err := util.GetAddressAndDialer(fraktiDockerShim)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(defaultTimeout), grpc.WithDialer(dialer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

func getImageClientConnection() (*grpc.ClientConn, error) {

	addr, dialer, err := util.GetAddressAndDialer(fraktiDockerShim)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(defaultTimeout), grpc.WithDialer(dialer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}
