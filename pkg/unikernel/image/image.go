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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"k8s.io/frakti/pkg/unikernel/metadata"
	metaimage "k8s.io/frakti/pkg/unikernel/metadata/image"
	"k8s.io/frakti/pkg/util/downloader"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	// UnikernelImagePrefix is the prefix of unikernel runtime image name.
	UnikernelImagePrefix = "unikernel/"
	// DefaultImageSuffix is the default image tag kubelet try to add to image.
	DefaultImageSuffix = ":latest"
)

type ImageManager struct {
	imageRoot   string
	storageRoot string
	downloader  downloader.Downloader
	metaStore   *metaimage.Store
}

func NewImageManager(downloadProtocol, unikernelRoot string) (*ImageManager, error) {
	manager := &ImageManager{
		imageRoot:  filepath.Join(unikernelRoot, "image"),
		downloader: downloader.NewBasicDownloader(downloadProtocol),
		metaStore:  metaimage.NewStore(),
	}
	manager.storageRoot = filepath.Join(manager.imageRoot, "storage")

	// Init image root dir and storage dir
	if err := os.MkdirAll(filepath.Join(manager.storageRoot), 0755); err != nil {
		return nil, fmt.Errorf("failed to create image root dir: %v", err)
	}

	return manager, nil
}

// PullImage download image from internet
func (im *ImageManager) PullImage(imageName string) (imageRef string, err error) {
	// TODO(Crazykev): Need implements more something like image manifest
	// to check image version before try to pull it

	// Standard image reference
	if strings.HasSuffix(imageName, DefaultImageSuffix) {
		imageName = imageName[0 : len(imageName)-len(DefaultImageSuffix)]
	}
	location := imageName
	if strings.HasPrefix(imageName, UnikernelImagePrefix) {
		location = imageName[len(UnikernelImagePrefix):]
	} else {
		glog.Warningf("Got PullImage request without unikernel image prefix in unikernel runtime.")
	}
	// Check image exist before try to pull it.
	exist := true
	existingImage, err := im.metaStore.Get(imageName)
	if err != nil {
		if !metadata.IsNotExistError(err) {
			return "", err
		}
		exist = false
	}
	imagePath, digest, err := im.downloadImageFile(location)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(imagePath)
	// Same digest means image is not updated in repository.
	// FIXME(Crazykev): What if downloaded image is damaged?
	if exist && digest == existingImage.Digest {
		return imageName, nil
	}

	// Get image size
	fInfo, err := os.Stat(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat download image tar %q: %v", imagePath, err)
	}
	size := fInfo.Size()

	imageStorage, imageManifest, err := im.prepareStorageForImage(imagePath, digest)
	if err != nil {
		return "", err
	}

	if exist {
		// Update image base storage
		err = im.metaStore.Update(imageName, func(origImage metaimage.Image) (metaimage.Image, error) {
			// Remove original storage
			err := os.RemoveAll(filepath.Join(im.storageRoot, origImage.Digest.String()))
			if err != nil {
				return origImage, fmt.Errorf("remove storage dir failed: %v", err)
			}
			delete(origImage.Copies, origImage.Digest.String())
			// Add new one
			if _, ok := origImage.Copies[digest.String()]; ok {
				return origImage, fmt.Errorf("image copy %q for image %q already exist", digest.String(), origImage.ID)
			}
			origImage.Copies[digest.String()] = *imageStorage
			origImage.Digest = digest
			return origImage, nil
		})
		if err != nil {
			return "", fmt.Errorf("update image metadata failed: %v", err)
		}
	} else {
		// Add image matedata
		newImage := metaimage.Image{
			ID:        imageName,
			RepoTags:  []string{imageName},
			Size:      size,
			ImageType: imageManifest.UnikernelType,
			Digest:    digest,
			Copies:    make(map[string]metaimage.Storage, 1),
		}
		newImage.Copies[newImage.Digest.String()] = *imageStorage
		glog.V(5).Infof("Adding image metadata %+v to image store", newImage)
		im.metaStore.Add(newImage)
	}
	return imageName, nil
}

// PrepareImage prepares image for container or VM
// and returns a location descriptor for image.
func (im *ImageManager) PrepareImage(imageName, sandboxID string) (s *metaimage.Storage, err error) {
	if strings.HasSuffix(imageName, DefaultImageSuffix) {
		imageName = imageName[0 : len(imageName)-len(DefaultImageSuffix)]
	}
	image, err := im.metaStore.Get(imageName)
	if err != nil {
		return nil, err
	}
	baseStorage, ok := image.Copies[image.Digest.String()]
	if !ok {
		return nil, fmt.Errorf("base storage of image %q not exist", imageName)
	}
	return im.cloneStorageForVM(&baseStorage, imageName, sandboxID)
}

// CleanupImageCopy cleanups image copy or other files
// prepared for container when create container.
func (im *ImageManager) CleanupImageCopy(imageName, sandboxID string) error {
	if strings.HasSuffix(imageName, DefaultImageSuffix) {
		imageName = imageName[0 : len(imageName)-len(DefaultImageSuffix)]
	}
	image, err := im.metaStore.Get(imageName)
	if err != nil {
		return err
	}
	if _, ok := image.Copies[sandboxID]; !ok {
		return nil
	}
	// Update image storage metadata
	vmStorageDir := filepath.Join(im.storageRoot, sandboxID)
	im.metaStore.Update(imageName, func(origImage metaimage.Image) (metaimage.Image, error) {
		if _, ok := origImage.Copies[sandboxID]; ok {
			if err = os.RemoveAll(vmStorageDir); err != nil {
				return origImage, fmt.Errorf("remove vm storage dir failed: %v", err)
			}
			delete(origImage.Copies, sandboxID)
		}
		return origImage, nil
	})
	return nil
}

// RemoveImage removes image by imageName
// If image is referenced by other containers, returns error
func (im *ImageManager) RemoveImage(imageName string) error {
	if strings.HasSuffix(imageName, DefaultImageSuffix) {
		imageName = imageName[0 : len(imageName)-len(DefaultImageSuffix)]
	}
	image, err := im.metaStore.Get(imageName)
	if err != nil {
		if metadata.IsNotExistError(err) {
			return nil
		}
		return err
	}
	if len(image.Copies) > 1 {
		return fmt.Errorf("Image(%q) is still in use by %d sandboxes", imageName, len(image.Copies)-1)
	}

	// Clean up image related storage.
	if len(image.Copies) == 0 {
		glog.Warningf("Image(%q) has no storage reference")
	} else {
		if _, ok := image.Copies[image.Digest.String()]; ok {
			os.RemoveAll(filepath.Join(im.imageRoot, "storage", image.Digest.String()))
		} else {
			// FIXME(Crazykev): should we forcibly remove this
			glog.Warningf("The last image storage reference is not base storage")
		}
	}

	// Delete image metadata.
	if err = im.metaStore.Delete(image.ID); err != nil {
		return err
	}

	return nil
}

// ListImages lists all images stores in ImageManager
func (im *ImageManager) ListImages() []metaimage.Image {
	return im.metaStore.List()
}

// GetImageInfo gets image metadata for image ID
func (im *ImageManager) GetImageInfo(imageName string) (*metaimage.Image, error) {
	if strings.HasSuffix(imageName, DefaultImageSuffix) {
		imageName = imageName[0 : len(imageName)-len(DefaultImageSuffix)]
	}
	image, err := im.metaStore.Get(imageName)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetFsUsage get image filesystem usage, including all copies of this image.
// TODO(Crazykev): Implement it.
// FIXME(Crazykev): Need to figure out is one image should only have one FilesystemUsage?
func (im *ImageManager) GetFsUsage(imageID string) ([]*kubeapi.FilesystemUsage, error) {
	return nil, fmt.Errorf("not implemented")
}
