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

	"k8s.io/frakti/pkg/unikernel/metadata"
	metaimage "k8s.io/frakti/pkg/unikernel/metadata/image"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ListImages lists existing images.
func (u *UnikernelRuntime) ListImages(filter *kubeapi.ImageFilter) ([]*kubeapi.Image, error) {
	// Deal with filter
	if filter != nil && filter.GetImage().GetImage() != "" {
		meta, err := u.imageManager.GetImageInfo(filter.GetImage().GetImage())
		if err != nil {
			if metadata.IsNotExistError(err) {
				return nil, nil
			}
			return nil, err
		}
		return []*kubeapi.Image{metaImageToCRI(meta)}, nil
	}
	// List all images in store
	imageMetaList := u.imageManager.ListImages()
	imageList := make([]*kubeapi.Image, len(imageMetaList))
	for n, meta := range imageMetaList {
		imageList[n] = metaImageToCRI(&meta)
	}

	return imageList, nil
}

// PullImage pulls the image with authentication config.
func (u *UnikernelRuntime) PullImage(image *kubeapi.ImageSpec, authConfig *kubeapi.AuthConfig) (string, error) {
	// TODO(Crazykev): Deal with auth config
	return u.imageManager.PullImage(image.GetImage())
}

// RemoveImage removes the image.
func (u *UnikernelRuntime) RemoveImage(image *kubeapi.ImageSpec) error {
	return u.imageManager.RemoveImage(image.GetImage())
}

// ImageStatus returns the status of the image.
func (u *UnikernelRuntime) ImageStatus(image *kubeapi.ImageSpec) (*kubeapi.Image, error) {
	meta, err := u.imageManager.GetImageInfo(image.GetImage())
	if err != nil {
		// return empty without error when image not found.
		if metadata.IsNotExistError(err) {
			return nil, nil
		}
		return nil, err
	}
	return metaImageToCRI(meta), nil
}

// ImageFsInfo returns information of the filesystem that is used to store images.
func (u *UnikernelRuntime) ImageFsInfo() ([]*kubeapi.FilesystemUsage, error) {
	return nil, fmt.Errorf("not implemented")
}

// metaImageToCRI returns CRI image based on image metadata
func metaImageToCRI(meta *metaimage.Image) *kubeapi.Image {
	return &kubeapi.Image{
		Id:       meta.ID,
		RepoTags: meta.RepoTags,
		Size_:    uint64(meta.Size),
	}
}
