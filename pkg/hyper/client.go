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
	"time"

	"github.com/hyperhq/hyperd/types"
	"google.golang.org/grpc"
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

// TODO: implements hyper client
