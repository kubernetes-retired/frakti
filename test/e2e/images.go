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

package e2e

import (
	"fmt"
	"strings"

	"k8s.io/frakti/test/e2e/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// image name for test image api
	testImageName string = "busybox"

	// name-tagged reference for latest test image
	latestTestImageRef string = testImageName + ":latest"

	// Digested reference for latest test image
	latestBusyboxDigestRef string = testImageName + "@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6"
)

func testPublicImage(client internalapi.ImageManagerService, image string) {
	By(fmt.Sprintf("pull image %s", image))
	_, err := client.PullImage(&kubeapi.ImageSpec{
		Image: image,
	}, nil)
	framework.ExpectNoError(err, "Failed to pull image: %v", err)

	if !strings.Contains(image, ":") {
		image = image + ":latest"
	}
	imageSpec := kubeapi.ImageSpec{
		Image: image,
	}
	By("get image list")
	imageList, err := client.ListImages(&kubeapi.ImageFilter{
		Image: &imageSpec,
	})
	framework.ExpectNoError(err, "Failed to get image list: %v", err)
	Expect(len(imageList)).To(Equal(1), "should have one image in list")

	By("remove image")
	err = client.RemoveImage(&imageSpec)
	framework.ExpectNoError(err, "Failed to remove image: %v", err)

	By("check image list empty")
	imageList, err = client.ListImages(&kubeapi.ImageFilter{
		Image: &imageSpec,
	})
	framework.ExpectNoError(err, "Failed to get image list: %v", err)
	Expect(len(imageList)).To(Equal(0), "should have none image in list")
}

// TODO: need some test case with private image

func pullImageList(client internalapi.ImageManagerService, imageList []string) {
	for _, imageRef := range imageList {
		By(fmt.Sprintf("pull image %s", imageRef))
		_, err := client.PullImage(&kubeapi.ImageSpec{
			Image: imageRef,
		}, nil)
		framework.ExpectNoError(err, "Failed to pull image: %v", err)
	}
}

func removeImageList(client internalapi.ImageManagerService, imageList []string) {
	for _, imageRef := range imageList {
		By(fmt.Sprintf("remove image %s", imageRef))
		err := client.RemoveImage(&kubeapi.ImageSpec{
			Image: imageRef,
		})
		framework.ExpectNoError(err, "Failed to remove image: %v", err)
	}
}

var _ = framework.KubeDescribe("Test image", func() {
	f := framework.NewDefaultFramework("image")

	var c internalapi.ImageManagerService

	BeforeEach(func() {
		c = f.Client.FraktiImageService
		// clear all images before each test
		framework.ClearAllImages(c)
	})

	It("public image with tag should be pulled and removed", func() {
		imageName := latestTestImageRef
		testPublicImage(c, imageName)
	})

	It("public image without tag should be pulled and removed", func() {
		imageName := testImageName
		testPublicImage(c, imageName)
	})

	It("public image with digest should be pulled and removed", func() {
		imageName := latestBusyboxDigestRef
		testPublicImage(c, imageName)
	})

	It("image status get image fileds should not be empty", func() {
		imageName := latestTestImageRef
		imageSpec := kubeapi.ImageSpec{
			Image: imageName,
		}

		By(fmt.Sprintf("pull image %s", imageName))
		_, err := c.PullImage(&imageSpec, nil)
		framework.ExpectNoError(err, "Failed to pull image: %v", err)

		defer c.RemoveImage(&imageSpec)

		By("get image status")
		image, err := c.ImageStatus(&imageSpec)
		framework.ExpectNoError(err, "Failed to get image status: %v", err)
		Expect(image.Id).NotTo(BeNil(), "image Id should not be nil")
		Expect(len(image.RepoTags)).NotTo(Equal(0), "should have repoTags in image")
		Expect(image.Size_).NotTo(BeNil(), "image Size should not be nil")
	})

	It("ListImage should get exactly 3 image in the result list", func() {
		// different tags refer to different images
		testImageList := []string{
			"busybox:1",
			"busybox:musl",
			"busybox:glibc",
		}

		pullImageList(c, testImageList)

		defer removeImageList(c, testImageList)

		By("get all image list")
		imageList, err := c.ListImages(nil)
		framework.ExpectNoError(err, "Failed to get image list: %v", err)
		Expect(len(imageList)).To(Equal(3), "should have exactly three images in list")
	})

	It("ListImage should get exactly 3 repoTags in the result image", func() {
		// different tags refer to the same image
		testImageList := []string{
			"busybox:1",
			"busybox:1-uclibc",
			"busybox:uclibc",
		}

		pullImageList(c, testImageList)

		defer removeImageList(c, testImageList)

		By("get all image list")
		imageList, err := c.ListImages(nil)
		framework.ExpectNoError(err, "Failed to get image list: %v", err)

		Expect(len(imageList)).To(Equal(1), "should have only one image in list")
		Expect(len(imageList[0].RepoTags)).To(Equal(3), "should have three repoTags in image")
	})
})
