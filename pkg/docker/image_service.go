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
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ListImages lists existing images.
func (p *PrivilegedRuntime) ListImages(filter *kubeapi.ImageFilter) ([]*kubeapi.Image, error) {
	request := &kubeapi.ListImagesRequest{Filter: &kubeapi.ImageFilter{}}
	logrus.Debugf("ListImagesRequest: %v", request)
	resp, err := imageClient.ListImages(context.Background(), request)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("ListImagesResponse: %v", resp)

	return resp.Images, nil
}

// ImageStatus returns the status of the image.
func (p *PrivilegedRuntime) ImageStatus(image *kubeapi.ImageSpec) (*kubeapi.Image, error) {
	request := &kubeapi.ImageStatusRequest{
		Image: &kubeapi.ImageSpec{Image: image.Image},
	}
	logrus.Debugf("ImageStatusRequest: %v", request)
	resp, err := imageClient.ImageStatus(context.Background(), request)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("ImageStatusResponse: %v", resp)

	return resp.Image, nil
}

// PullImage pulls an image with authentication config.
func (p *PrivilegedRuntime) PullImage(image *kubeapi.ImageSpec, auth *kubeapi.AuthConfig) (string, error) {
	request := &kubeapi.PullImageRequest{
		Image: &kubeapi.ImageSpec{
			Image: image.GetImage(),
		},
	}
	if auth != nil {
		request.Auth = auth
	}
	logrus.Debugf("PullImageRequest: %v", request)
	resp, err := imageClient.PullImage(context.Background(), request)
	if err != nil {
		return "", err
	}
	logrus.Debugf("PullImageResponse: %v", resp)
	return resp.ImageRef, nil
}

// RemoveImage removes the image.
func (p *PrivilegedRuntime) RemoveImage(image *kubeapi.ImageSpec) error {
	request := &kubeapi.RemoveImageRequest{Image: &kubeapi.ImageSpec{Image: image.Image}}
	logrus.Debugf("RemoveImageRequest: %v", request)
	resp, err := imageClient.RemoveImage(context.Background(), request)
	if err != nil {
		return err
	}
	logrus.Debugf("RemoveImageResponse: %v", resp)
	return nil
}

// ImageFSInfo returns information of the filesystem that is used to store images.
func (p *PrivilegedRuntime) ImageFsInfo() ([]*kubeapi.FilesystemUsage, error) {
	request := &kubeapi.ImageFsInfoRequest{}
	logrus.Debugf("RemoveImageRequest: %v", request)
	resp, err := imageClient.ImageFsInfo(context.Background(), request)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("RemoveImageResponse: %v", resp)

	return resp.ImageFilesystems, nil
}
