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

package alternativeruntime

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// internal represent of privileged runtime name
	PrivilegedRuntimeName = "privileged runtime"
	// internal represent of unikernel runtime name
	UnikernelRuntimeName = "unikernel runtime"
)

type AlternativeRuntimeSets struct {
	// privilegedStore store privileged runtime sandbox/container id set.
	privilegedStore sets.String
	// unikernelStore store unikernel runtime sandbox/container id set.
	unikernelStore sets.String
	// Lock to protect this sets
	sync.RWMutex
}

func NewAlternativeRuntimeSets() *AlternativeRuntimeSets {
	return &AlternativeRuntimeSets{
		privilegedStore: sets.NewString(),
		unikernelStore:  sets.NewString(),
	}
}

// GetRuntime return name of runtime it belongs to, return "" if none matched.
func (f *AlternativeRuntimeSets) GetRuntime(id string) string {
	f.RLock()
	defer f.RUnlock()
	if f.privilegedStore.Has(id) {
		return PrivilegedRuntimeName
	} else if f.unikernelStore.Has(id) {
		return UnikernelRuntimeName
	}
	return ""
}

// Has return if id exist in specific runtime store.
func (f *AlternativeRuntimeSets) Has(id string, runtimeType string) bool {
	f.RLock()
	defer f.RUnlock()
	switch runtimeType {
	case PrivilegedRuntimeName:
		return f.privilegedStore.Has(id)
	case UnikernelRuntimeName:
		return f.unikernelStore.Has(id)
	}
	return false
}

// Remove remove id record from specific runtime store.
func (f *AlternativeRuntimeSets) Remove(id string, runtimeType string) {
	f.Lock()
	defer f.Unlock()
	// Do nothing if none is matched
	switch runtimeType {
	case PrivilegedRuntimeName:
		f.privilegedStore.Delete(id)
	case UnikernelRuntimeName:
		f.unikernelStore.Delete(id)
	}
}

// Add add id record to specific runtime store.
func (f *AlternativeRuntimeSets) Add(id string, runtimeType string) {
	f.Lock()
	defer f.Unlock()
	// Do nothing if none is matched
	switch runtimeType {
	case PrivilegedRuntimeName:
		f.privilegedStore.Insert(id)
	case UnikernelRuntimeName:
		f.unikernelStore.Insert(id)
	}
}

// IsNotEmpty return if specific runtime store is empty.
func (f *AlternativeRuntimeSets) IsNotEmpty(runtimeType string) bool {
	f.RLock()
	defer f.RUnlock()

	switch runtimeType {
	case PrivilegedRuntimeName:
		return (f.privilegedStore.Len() != 0)
	case UnikernelRuntimeName:
		return (f.unikernelStore.Len() != 0)
	}
	// Return empty if none is matched
	return true
}
