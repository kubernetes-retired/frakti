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
	"strings"

	"github.com/golang/glog"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// ListImages lists existing images.
func (h *Runtime) ListImages(filter *kubeapi.ImageFilter) ([]*kubeapi.Image, error) {
	images, err := h.client.GetImages()
	if err != nil {
		glog.Errorf("Get image list failed: %v", err)
		return nil, err
	}

	var results []*kubeapi.Image
	for _, img := range images {
		if filter != nil {
			filter := filter.Image.GetImage()
			// Use 'latest' tag if not specified explicitly
			if !strings.Contains(filter, ":") {
				filter = filter + ":latest"
			}

			if !inList(filter, img.RepoTags) && !inList(filter, img.RepoDigests) {
				continue
			}
		}

		imageSize := uint64(img.VirtualSize)
		results = append(results, &kubeapi.Image{
			Id:          &img.Id,
			RepoTags:    img.RepoTags,
			RepoDigests: img.RepoDigests,
			Size_:       &imageSize,
		})
	}

	glog.V(4).Infof("Got imageList: %q", results)
	return results, nil
}

// PullImage pulls the image with authentication config.
func (h *Runtime) PullImage(image *kubeapi.ImageSpec, authConfig *kubeapi.AuthConfig) (string, error) {
	repo, tag := parseRepositoryTag(image.GetImage())
	auth := getHyperAuthConfig(authConfig)
	err := h.client.PullImage(repo, tag, auth, nil)
	if err != nil {
		glog.Errorf("Pull image %q failed: %v", image.GetImage(), err)
		return "", err
	}

	imageInfo, err := h.client.GetImageInfo(repo, tag)
	if err != nil {
		glog.Errorf("Get image info for %q failed: %v", image.GetImage(), err)
		return "", err
	}

	return imageInfo.Id, nil
}

// RemoveImage removes the image.
func (h *Runtime) RemoveImage(image *kubeapi.ImageSpec) error {
	repo, tag := parseRepositoryTag(image.GetImage())
	err := h.client.RemoveImage(repo, tag)
	if err != nil {
		glog.Errorf("Remove image %q failed: %v", image.GetImage(), err)
		return err
	}

	return nil
}

// ImageStatus returns the status of the image.
func (h *Runtime) ImageStatus(image *kubeapi.ImageSpec) (*kubeapi.Image, error) {
	repo, tag := parseRepositoryTag(image.GetImage())
	imageInfo, err := h.client.GetImageInfo(repo, tag)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		glog.Errorf("Get image info for %q failed: %v", image.GetImage(), err)
		return nil, err
	}

	imageSize := uint64(imageInfo.VirtualSize)
	return &kubeapi.Image{
		Id:          &imageInfo.Id,
		RepoTags:    imageInfo.RepoTags,
		RepoDigests: imageInfo.RepoDigests,
		Size_:       &imageSize,
	}, nil
}
