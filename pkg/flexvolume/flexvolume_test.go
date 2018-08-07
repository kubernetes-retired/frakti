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

package flexvolume

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockFlexVolume struct{}

func (m *mockFlexVolume) Init() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) Attach(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) Detach(mountDev, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) WaitForAttach(mountDev, jsonOptions string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) IsAttached(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) Mount(targetMountDir, jsonOptions string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (m *mockFlexVolume) Unmount(targetMountDir string) (map[string]interface{}, error) {
	return map[string]interface{}{}, errors.New("mock unmount failure")
}

func TestNewFlexVolume(t *testing.T) {
	assert := assert.New(t)

	notsupported := "Not supported"
	failure := "Failure"
	success := "Success"

	m := &mockFlexVolume{}
	f := NewFlexVolume(m)

	_, err := f.doRun([]string{})
	assert.Error(err)

	result := f.Run(strings.Split("foo bar foo", " "))
	assert.Contains(result, notsupported)

	result = f.Run(strings.Split("init", " "))
	assert.Contains(result, success)

	result = f.Run(strings.Split("init foobar", " "))
	assert.Contains(result, failure)

	result = f.Run(strings.Split("unmount foobar", " "))
	assert.Contains(result, failure)
}
