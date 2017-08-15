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

package service

import (
	"fmt"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ListImages lists existing images.
func (u *UnikernelRuntime) ListImages(filter *kubeapi.ImageFilter) ([]*kubeapi.Image, error) {
	return nil, fmt.Errorf("not implemented")
}

// PullImage pulls the image with authentication config.
func (u *UnikernelRuntime) PullImage(image *kubeapi.ImageSpec, authConfig *kubeapi.AuthConfig) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// RemoveImage removes the image.
func (u *UnikernelRuntime) RemoveImage(image *kubeapi.ImageSpec) error {
	return fmt.Errorf("not implemented")
}

// ImageStatus returns the status of the image.
func (u *UnikernelRuntime) ImageStatus(image *kubeapi.ImageSpec) (*kubeapi.Image, error) {
	return nil, fmt.Errorf("not implemented")
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (u *UnikernelRuntime) ImageFsInfo(req *kubeapi.ImageFsInfoRequest) (*kubeapi.ImageFsInfoResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
