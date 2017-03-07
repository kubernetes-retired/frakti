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

type AlternativeRuntimeSets struct {
	store sets.String
	// Lock to protect this sets
	sync.RWMutex
}

func NewAlternativeRuntimeSets() *AlternativeRuntimeSets {
	return &AlternativeRuntimeSets{store: sets.NewString()}
}

func (f *AlternativeRuntimeSets) Has(id string) bool {
	f.RLock()
	defer f.RUnlock()
	return f.store.Has(id)
}

func (f *AlternativeRuntimeSets) Remove(id string) {
	f.Lock()
	defer f.Unlock()
	f.store.Delete(id)
}

func (f *AlternativeRuntimeSets) Add(id string) {
	f.Lock()
	defer f.Unlock()
	f.store.Insert(id)
}

func (f *AlternativeRuntimeSets) IsNotEmpty() bool {
	f.RLock()
	defer f.RUnlock()
	return (f.store.Len() != 0)
}
