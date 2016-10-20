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
	"reflect"
	"strings"

	internalapi "k8s.io/kubernetes/pkg/kubelet/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Framework supports common operations used by e2e tests; it will keep a client & a namespace for you.
// Eventual goal is to merge this with integration test framework.
type Framework struct {
	BaseName string

	Client *FraktiClient

	// To make sure that this framework cleans up after itself, no matter what,
	// we install a Cleanup action before each test and clear it after.  If we
	// should abort, the AfterSuite hook should run all Cleanup actions.
	cleanupHandle CleanupActionHandle

	// configuration for framework's client
	options FrameworkOptions
}

type FraktiClient struct {
	FraktiRuntimeService internalapi.RuntimeService
	FraktiImageService   internalapi.ImageManagerService
}

type TestDataSummary interface {
	PrintHumanReadable() string
	PrintJSON() string
}

type FrameworkOptions struct {
	ClientQPS   float32
	ClientBurst int
}

// NewFramework makes a new framework and sets up a BeforeEach/AfterEach for
// you (you can write additional before/after each functions).
func NewDefaultFramework(baseName string) *Framework {
	options := FrameworkOptions{
		ClientQPS:   20,
		ClientBurst: 50,
	}
	return NewFramework(baseName, options, nil)
}

func NewFramework(baseName string, options FrameworkOptions, client *FraktiClient) *Framework {
	f := &Framework{
		BaseName: baseName,
		options:  options,
		Client:   client,
	}

	BeforeEach(f.BeforeEach)
	AfterEach(f.AfterEach)

	return f
}

// BeforeEach gets a client
func (f *Framework) BeforeEach() {
	// The fact that we need this feels like a bug in ginkgo.
	// https://github.com/onsi/ginkgo/issues/222
	f.cleanupHandle = AddCleanupAction(f.AfterEach)
	if f.Client == nil {
		c, err := LoadDefaultClient()
		Expect(err).NotTo(HaveOccurred())
		f.Client = c
	}

}

// AfterEach clean resourses and print summaries
func (f *Framework) AfterEach() {
	RemoveCleanupAction(f.cleanupHandle)

	defer func() {
		// Paranoia-- prevent reuse!
		f.Client = nil
	}()

	summaries := make([]TestDataSummary, 0)

	// TODO: add summaries

	outputTypes := strings.Split(TestContext.OutputPrintType, ",")
	for _, printType := range outputTypes {
		switch printType {
		case "hr":
			for i := range summaries {
				Logf(summaries[i].PrintHumanReadable())
			}
		case "json":
			for i := range summaries {
				typeName := reflect.TypeOf(summaries[i]).String()
				Logf("%v JSON\n%v", typeName[strings.LastIndex(typeName, ".")+1:], summaries[i].PrintJSON())
				Logf("Finished")
			}
		default:
			Logf("Unknown output type: %v. Skipping.", printType)
		}
	}
}

func KubeDescribe(text string, body func()) bool {
	return Describe("[k8s.io] "+text, body)
}
