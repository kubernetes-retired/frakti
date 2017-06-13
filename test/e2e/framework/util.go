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

package framework

import (
	"fmt"
	"net"
	"sync"
	"time"

	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/remote"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
)

var (
	//lock for uuid
	uuidLock sync.Mutex

	// lastUUID record last generated uuid from NewUUID()
	lastUUID uuid.UUID

	//default network for cni configure
	DefaultNet string = "10.30.0.0/16"
)

func LoadDefaultClient() (*FraktiClient, error) {
	rService, err := remote.NewRemoteRuntimeService(TestContext.RuntimeServiceAddr, TestContext.RuntimeServiceTimeout)
	if err != nil {
		return nil, err
	}

	iService, err := remote.NewRemoteImageService(TestContext.ImageServiceAddr, TestContext.ImageServiceTimeout)
	if err != nil {
		return nil, err
	}

	return &FraktiClient{
		FraktiRuntimeService: rService,
		FraktiImageService:   iService,
	}, nil
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	Fail(nowStamp()+": "+msg, 1)
}

func Skipf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	Skip(nowStamp() + ": " + msg)
}

func SkipUnlessAtLeast(value int, minValue int, message string) {
	if value < minValue {
		Skipf(message)
	}
}

func ExpectNoError(err error, explain ...interface{}) {
	if err != nil {
		Logf("Unexpected error occurred: %v", err)
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

// podReady returns whether podsandbox state is ready.
func PodReady(status *kubeapi.PodSandboxStatus) bool {
	if status.State == kubeapi.PodSandboxState_SANDBOX_READY {
		return true
	}
	return false
}

//podFound returns whether podsandbox is found
func PodFound(podsandboxs []*kubeapi.PodSandbox, podId string) bool {
	if len(podsandboxs) == 1 && podsandboxs[0].Id == podId {
		return true
	}
	return false
}
func CniWork(podNetworkStatus *kubeapi.PodSandboxNetworkStatus) bool {
	if podNetworkStatus.Ip != "" {
		podIPMask := podNetworkStatus.Ip + "/16"
		_, podNet, err := net.ParseCIDR(podIPMask)
		if err != nil {
			return false
		}
		_, defaultNet, _ := net.ParseCIDR(DefaultNet)
		if string(podNet.IP) == string(defaultNet.IP) {
			return true
		}
		return false
	}
	return false
}

func NewUUID() string {
	uuidLock.Lock()
	defer uuidLock.Unlock()
	result := uuid.NewUUID()
	for uuid.Equal(lastUUID, result) == true {
		result = uuid.NewUUID()
	}
	lastUUID = result
	return result.String()
}

func ClearAllImages(client internalapi.ImageManagerService) {
	imageList, err := client.ListImages(nil)
	ExpectNoError(err, "Failed to get image list: %v", err)
	for _, image := range imageList {
		if len(image.RepoTags) == 0 {
			for _, rd := range image.RepoDigests {
				repoDigest := rd
				err = client.RemoveImage(&kubeapi.ImageSpec{
					Image: repoDigest,
				})
				ExpectNoError(err, "Failed to remove image: %v", err)
			}
			continue
		}
		for _, rt := range image.RepoTags {
			repoTag := rt
			err = client.RemoveImage(&kubeapi.ImageSpec{
				Image: repoTag,
			})
			ExpectNoError(err, "Failed to remove image: %v", err)
		}
	}
	imageList, err = client.ListImages(nil)
	ExpectNoError(err, "Failed to get image list: %v", err)
	Expect(len(imageList)).To(Equal(0), "should have cleaned all images")
}
