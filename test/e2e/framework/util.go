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
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var (
	//lock for uuid
	uuidLock sync.Mutex

	// lastUUID record last generated uuid from NewUUID()
	lastUUID uuid.UUID
)

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
	if *status.State == kubeapi.PodSandBoxState_READY {
		return true
	}
	return false
}

//podFound returns whether podsandbox is found
func PodFound(podsandboxs []*kubeapi.PodSandbox, podId string) bool {
	if len(podsandboxs) == 1 && podsandboxs[0].GetId() == podId {
		return true
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

func ClearAllImages(client *FraktiClient) {
	imageList, err := client.ListImages(nil)
	ExpectNoError(err, "Failed to get image list: %v", err)
	for _, image := range imageList {
		if len(image.RepoTags) == 0 {
			for _, rd := range image.RepoDigests {
				repoDigest := rd
				err = client.RemoveImage(&kubeapi.ImageSpec{
					Image: &repoDigest,
				})
				ExpectNoError(err, "Failed to remove image: %v", err)
			}
			continue
		}
		for _, rt := range image.RepoTags {
			repoTag := rt
			err = client.RemoveImage(&kubeapi.ImageSpec{
				Image: &repoTag,
			})
			ExpectNoError(err, "Failed to remove image: %v", err)
		}
	}
	imageList, err = client.ListImages(nil)
	ExpectNoError(err, "Failed to get image list: %v", err)
	Expect(len(imageList)).To(Equal(0), "should have cleaned all images")
}
