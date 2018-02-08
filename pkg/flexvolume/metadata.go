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

const (
	VolIdKey  = "volumeID"
	FsTypeKey = "fsType"

	HyperFlexvolumeDataFile = "hyper-flexvolume.json"

	// Cinder flexvolume
	CinderConfigKey  = "cinderConfig"
	CinderConfigFile = "/etc/kubernetes/cinder.conf"

	// GCE PD flexvolume
	ZoneKey      = "zone"
	ProjectKey   = "project"
	DivcePathKey = "devicePath"

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
	DevicePath   string `json:"devicePath"`
	SystemFsType string `json:"kubernetes.io/fsType"`
}
