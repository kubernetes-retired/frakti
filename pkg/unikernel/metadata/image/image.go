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

package image

import (
	"sync"

	godigest "github.com/opencontainers/go-digest"
	"k8s.io/frakti/pkg/unikernel/metadata/store"
)

type ImageFormat int

const (
	UNKNOWN ImageFormat = iota
	QCOW2
	RAW
	KERNEL_INITRD
	LINUXKIT_YML
)

// Storage is the storage metadata of a image copy.
type Storage struct {
	// UUID is the unique identifier, either generated or binded sandbox's UUID
	UUID string
	// Format is the format of this image
	Format ImageFormat
	// ImageFile is the location of qcow2/raw/kernel/linuxkit_yaml format image file.
	ImageFile string
	// Initrd is the location of image's initrd part
	Initrd string
	// Cmdline is the content of image's cmdline
	Cmdline string
}

// Image contains all resources associated with the image.
type Image struct {
	// Id of the image. Normally the digest of image config.
	ID string
	// Other names by which this image is known.
	RepoTags []string
	// Digests by which this image is known.
	RepoDigests []string
	// Size is the compressed size of the image.
	Size int64
	// ImageType is the unikernel type of this image.
	ImageType string
	// Digest is the image tar file's digest, only used for check whether two image is same one.
	Digest godigest.Digest
	// Copies are all image file copy for the image, indexed by image digest or sandbox uuid.
	Copies map[string]Storage
}

// UpdateFunc is function used to update the image. If there
// is an error, the update will be rolled back.
type UpdateFunc func(Image) (Image, error)

// Store stores all images.
type Store struct {
	lock   sync.RWMutex
	images map[string]Image
}

// LoadStore loads images from disk.
// TODO(Crazykev): Implement LoadStore.
func LoadStore() *Store { return nil }

// NewStore creates an image store.
func NewStore() *Store {
	return &Store{images: make(map[string]Image)}
}

// Add an image into the store.
func (s *Store) Add(img Image) {
	s.lock.Lock()
	defer s.lock.Unlock()
	i, ok := s.images[img.ID]
	if !ok {
		// If the image doesn't exist, add it.
		s.images[img.ID] = img
		return
	}
	// Or else, merge the repo tags/digests.
	i.RepoTags = mergeStringSlices(i.RepoTags, img.RepoTags)
	i.RepoDigests = mergeStringSlices(i.RepoDigests, img.RepoDigests)
	s.images[img.ID] = i
}

// Get returns the image with specified id. Returns store.ErrNotExist if the
// image doesn't exist.
func (s *Store) Get(id string) (Image, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if i, ok := s.images[id]; ok {
		return i, nil
	}
	return Image{}, store.ErrNotExist
}

// Update updates image content
func (s *Store) Update(id string, u UpdateFunc) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if i, ok := s.images[id]; ok {
		newImage, err := u(i)
		if err != nil {
			return err
		}
		s.images[id] = newImage
		return nil
	}
	return store.ErrNotExist
}

// List lists all images.
func (s *Store) List() []Image {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var images []Image
	for _, sb := range s.images {
		images = append(images, sb)
	}
	return images
}

// Delete deletes the image with specified id.
func (s *Store) Delete(id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.images, id)
	// TODO(Crazykev): Delete metadata on disk
	return nil
}

// mergeStringSlices merges 2 string slices into one and remove duplicated elements.
func mergeStringSlices(a []string, b []string) []string {
	set := map[string]struct{}{}
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		set[s] = struct{}{}
	}
	var ss []string
	for s := range set {
		ss = append(ss, s)
	}
	return ss
}
