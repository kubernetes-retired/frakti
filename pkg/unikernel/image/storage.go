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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	godigest "github.com/opencontainers/go-digest"
	metaimage "k8s.io/frakti/pkg/unikernel/metadata/image"
)

type ImageManifest struct {
	UnikernelType string
	Format        string
	ImageFile     string
	Kernel        string
	Initrd        string
	Cmdline       string
}

func (im *ImageManager) prepareStorageForImage(imagePath string, digest godigest.Digest) (*metaimage.Storage, *ImageManifest, error) {
	// Prepare temp storage dir
	desDir, err := ioutil.TempDir(im.storageRoot, "_temp")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir failed: %v", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(desDir)
		}
	}()
	// storage
	err = archive.NewDefaultArchiver().UntarPath(imagePath, desDir)
	if err != nil {
		return nil, nil, fmt.Errorf("untar image tar file(%q) failed: %v", imagePath, err)
	}
	maniContent, err := ioutil.ReadFile(filepath.Join(desDir, "manifest"))
	if err != nil {
		return nil, nil, fmt.Errorf("read image manifest failed: %v", err)
	}
	imageManifest := ImageManifest{}
	if err = json.Unmarshal(maniContent, &imageManifest); err != nil {
		return nil, nil, fmt.Errorf("unmarshal image manifest failed: %v", err)
	}
	if err = valideManifest(&imageManifest); err != nil {
		return nil, nil, err
	}
	imageDirPath := filepath.Join(im.storageRoot, digest.String())
	// TODO(Crazykev): Need to check imageDirPath is exist
	if err = os.Rename(desDir, imageDirPath); err != nil {
		return nil, nil, fmt.Errorf("rename temp dir to image dir failed: %v", err)
	}
	return im.manifestToStorage(&imageManifest, digest.String()), &imageManifest, nil
}

func (im *ImageManager) manifestToStorage(mani *ImageManifest, digestID string) *metaimage.Storage {
	// NOTE: validation of image manifest should be gurranteed by `valideManifest`
	meta := &metaimage.Storage{UUID: digestID}
	imageDir := filepath.Join(im.storageRoot, digestID)
	switch mani.Format {
	case "qcow2":
		meta.Format = metaimage.QCOW2
	case "raw":
		meta.Format = metaimage.RAW
	case "linuxkit":
		meta.Format = metaimage.LINUXKIT_YML
	case "kernel+initrd":
		meta.Initrd = filepath.Join(imageDir, filepath.Clean(mani.Initrd))
		fallthrough
	case "kernel":
		meta.Format = metaimage.KERNEL_INITRD
		meta.Cmdline = mani.Cmdline
	default:
		meta.Format = metaimage.UNKNOWN
	}
	meta.ImageFile = filepath.Join(imageDir, filepath.Clean(mani.ImageFile))
	return meta
}

// valideManifest check if all fields are validate
// all images are gurranteed
// TODO(Crazykev): Implement this
func valideManifest(mani *ImageManifest) error {
	return nil
}

// cloneStorageForVM prepares a copy of image storage for every VM.
func (im *ImageManager) cloneStorageForVM(baseStorage *metaimage.Storage, imageName, sandboxID string) (s *metaimage.Storage, err error) {
	tmpDir, err := ioutil.TempDir(im.storageRoot, "_temp_vm")
	if err != nil {
		return nil, fmt.Errorf("create temp dir failed: %v", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()
	targetDir := filepath.Join(im.storageRoot, sandboxID)
	vmStorage := metaimage.Storage{UUID: sandboxID}
	switch baseStorage.Format {
	case metaimage.LINUXKIT_YML:
		return nil, fmt.Errorf("linuxkit image live build not implemented yet")
	case metaimage.KERNEL_INITRD:
		if baseStorage.Initrd != "" {
			err = copyFile(baseStorage.Initrd, filepath.Join(tmpDir, filepath.Base(baseStorage.Initrd)))
			if err != nil {
				return nil, fmt.Errorf("copy image files failed: %v", err)
			}
			vmStorage.Initrd = filepath.Join(targetDir, filepath.Base(baseStorage.Initrd))
		}
		fallthrough
	case metaimage.RAW:
		fallthrough
	case metaimage.QCOW2:
		err = copyFile(baseStorage.ImageFile, filepath.Join(tmpDir, filepath.Base(baseStorage.ImageFile)))
		if err != nil {
			return nil, fmt.Errorf("copy image files failed: %v", err)
		}
		vmStorage.Format = baseStorage.Format
		vmStorage.ImageFile = filepath.Join(targetDir, filepath.Base(baseStorage.ImageFile))
	default:
		return nil, fmt.Errorf("unknown storage type %d", baseStorage.Format)
	}
	// Update image storage
	im.metaStore.Update(imageName, func(origImage metaimage.Image) (metaimage.Image, error) {
		if _, ok := origImage.Copies[sandboxID]; ok {
			return origImage, fmt.Errorf("image %q copy for sandbox %q already exist", origImage.ID, sandboxID)
		}
		if err = os.Rename(tmpDir, targetDir); err != nil {
			return origImage, fmt.Errorf("rename image storage for sandbox %q failed: %v", sandboxID, err)
		}
		origImage.Copies[sandboxID] = vmStorage
		return origImage, nil
	})

	return &vmStorage, nil
}

// copyFile copies file from src to des.
func copyFile(srcFile, destFile string) error {
	file, err := os.Open(srcFile)
	if err != nil {
		return err

	}
	defer file.Close()
	dest, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer dest.Close()
	io.Copy(dest, file)
	return nil
}

// downloadImageFile download image file with image location, returns downloaded image path and caculated digest
func (im *ImageManager) downloadImageFile(imageName string) (string, godigest.Digest, error) {
	readcloser, err := im.downloader.DownloadStream(imageName)
	if err != nil {
		return "", "", fmt.Errorf("download image(%q) failed: %v", imageName, err)
	}
	defer readcloser.Close()
	tmpFile, err := ioutil.TempFile("", "frakti_")
	if err != nil {
		return "", "", err
	}
	defer tmpFile.Close()
	digester := godigest.Canonical.Digester()
	_, err = io.Copy(tmpFile, io.TeeReader(readcloser, digester.Hash()))
	if err != nil {
		return "", "", fmt.Errorf("coping image failed")
	}
	return tmpFile.Name(), digester.Digest(), nil
}
