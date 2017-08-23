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

package indexset

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// IndexSet is a very simple multi-thread safety index set to store ids
type IndexSet struct {
	indexs sets.String
	// Lock to protect this sets
	sync.RWMutex
}

func NewIndexSet(item ...string) *IndexSet {
	return &IndexSet{indexs: sets.NewString(item...)}
}

// Add adds id record to set, return error if already exist.
func (idx *IndexSet) Add(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if idx.indexs.Has(id) {
		return fmt.Errorf("id already exist: '%s'", id)
	}
	idx.indexs.Insert(id)
	return nil
}

// Delete delete id record from set, error if doesn't exist.
func (idx *IndexSet) Delete(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if idx.indexs.Has(id) {
		idx.indexs.Delete(id)
		return nil
	}
	return fmt.Errorf("no such id: '%s'", id)
}

// Has return if id exist in set.
func (idx *IndexSet) Has(id string) bool {
	idx.RLock()
	defer idx.RUnlock()
	return idx.indexs.Has(id)
}
