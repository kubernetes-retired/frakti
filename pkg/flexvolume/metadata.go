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
	"fmt"
	"os"
	"path/filepath"

	utilmetadata "k8s.io/frakti/pkg/util/metadata"
)

const (
	VolIdKey = "volumeID"

	HyperFlexvolumeDataFile = "hyper-flexvolume.json"

	// Cinder flexvolume
	CinderConfigKey  = "cinderConfig"
	CinderConfigFile = "/etc/kubernetes/cinder.conf"

	// GCE PD flexvolume
	ZoneKey    = "zone"
	ProjectKey = "project"

	// Build-in fsType key of flexvolume
	SystemFsTypeKey = "kubernetes.io/fsType"
)

// CinderVolumeOptsData is the struct of json file
type CinderVolumeOptsData struct {
	// Needed to reconstruct new cinder clients
	ConfigKey string `json:"cinderConfig"`
	VolumeID  string `json:"volumeID"`

	// rbd volume details
	VolumeType string   `json:"volume_type"`
	Name       string   `json:"name"`
	FsType     string   `json:"fsType"`
	Hosts      []string `json:"hosts"`
	Ports      []string `json:"ports"`
}

// GCEPDOptsData is the struct of json file
type GCEPDOptsData struct {
	// Needed for unmount
	VolumeID string `json:"volumeID"`
	Zone     string `json:"zone"`
	Project  string `json:"project"`

	// gce pd volume details
	DevicePath string `json:"devicePath"`
	FsType     string `json:"fsType"`
}

type FlexVolumeOptsData struct {
	CinderData *CinderVolumeOptsData `json:"cinderVolumeOptsData,omitempty"`
	GCEPDData  *GCEPDOptsData        `json:"gCEPDOptsData,omitempty"`
}

func WriteJsonOptsFile(targetDir string, opts interface{}) error {
	return utilmetadata.WriteJson(filepath.Join(targetDir, HyperFlexvolumeDataFile), opts, 0700)
}

func ReadJsonOptsFile(targetDir string, opts interface{}) error {
	return utilmetadata.ReadJson(filepath.Join(targetDir, HyperFlexvolumeDataFile), opts)
}

func CleanUpMetadataFile(targetDir string) error {
	metadataFile := filepath.Join(targetDir, HyperFlexvolumeDataFile)
	if err := os.Remove(metadataFile); err != nil {
		return fmt.Errorf("removing metadata file: %v failed: %v", metadataFile, err)
	}
	return nil
}
