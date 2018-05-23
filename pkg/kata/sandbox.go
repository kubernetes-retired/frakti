/*
Copyright 2018 The Kubernetes Authors.

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

package kata

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/runtime"
)

// CreateSandbox creates a kata-runtime sandbox
func (r *Runtime) CreateSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("not implemented")
}

// StartSandbox starts a kata-runtime sandbox
func (r *Runtime) StartSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("not implemented")
}

// StopSandbox stops a kata-runtime sandbox
func (r *Runtime) StopSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("not implemented")
}

// DeleteSandbox deletes a kata-runtime sandbox
func (r *Runtime) DeleteSandbox(ctx context.Context, id string, opts runtime.CreateOpts) error {
	return fmt.Errorf("not implemented")
}
