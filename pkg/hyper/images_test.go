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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func TestPullImage(t *testing.T) {
	r, _, _ := newTestRuntime()
	imageFullName := []string{
		"localhost:5000/foo/bar@sha256:12345",
		"test/foo:54321",
	}
	for i := range imageFullName {
		imageSpec := &kubeapi.ImageSpec{
			Image: imageFullName[i],
		}
		id, err := r.PullImage(imageSpec, nil)
		assert.NoError(t, err)
		str := strings.Split(imageFullName[i], ":")
		expected := str[len(str)-1]
		assert.Equal(t, expected, id)
	}

}

func TestListImage(t *testing.T) {
	r, fakeClient, _ := newTestRuntime()
	imageFullName := []string{
		"localhost:5000/foo/bar@sha256:12345",
		"test/foo:54321",
	}
	expected := []*kubeapi.Image{}
	for i := range imageFullName {
		imageSpec := &kubeapi.ImageSpec{
			Image: imageFullName[i],
		}
		id, err := r.PullImage(imageSpec, nil)
		assert.NoError(t, err)
		repoTages := []string{}
		repoTages = append(repoTages, imageFullName[i])
		image := kubeapi.Image{
			Id:          id,
			RepoTags:    repoTages,
			RepoDigests: nil,
			Size_:       0,
		}
		expected = append(expected, &image)
	}
	fliter := kubeapi.ImageFilter{}
	//Test list image
	images, err := r.ListImages(&fliter)
	assert.NoError(t, err)
	assert.Equal(t, expected, images)
	//Test remove image
	assert.Len(t, fakeClient.imageInfoList, len(imageFullName))
	image := &kubeapi.ImageSpec{
		Image: "localhost:5000/foo/bar@sha256:12345",
	}
	err = r.RemoveImage(image)
	assert.NoError(t, err)
	assert.Len(t, fakeClient.imageInfoList, len(imageFullName)-1)

}

func TestImageStatus(t *testing.T) {
	r, _, _ := newTestRuntime()
	imageFullName := []string{
		"localhost:5000/foo/bar@sha256:12345",
		"test/foo:54321",
	}
	for i := range imageFullName {
		imageSpec := &kubeapi.ImageSpec{
			Image: imageFullName[i],
		}
		//Pull the image for test
		id, err := r.PullImage(imageSpec, nil)
		assert.NoError(t, err)

		imageSpec = &kubeapi.ImageSpec{
			Image: imageFullName[i],
		}
		//Get the image's status
		image, err := r.ImageStatus(imageSpec)
		assert.NoError(t, err)
		repoTages := []string{}
		repoTages = append(repoTages, imageFullName[i])
		expected := &kubeapi.Image{
			Id:          id,
			RepoTags:    repoTages,
			RepoDigests: nil,
			Size_:       0,
		}
		assert.Equal(t, image, expected)
	}
}
