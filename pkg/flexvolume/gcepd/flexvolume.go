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

package gcepd

import (
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/frakti/pkg/flexvolume"
	"k8s.io/klog"
)

type FlexVolumeDriver struct {
	uuid string
	name string

	// Options from flexvolume
	volId   string
	project string
	zone    string
	fsType  string
	// NOTE: GCE disk type is default to PD, we may want to support more in the future.
}

// NewFlexVolumeDriver returns a flex volume driver
func NewFlexVolumeDriver(uuid string, name string) *FlexVolumeDriver {
	return &FlexVolumeDriver{
		uuid: uuid,
		name: name,
	}
}

// Invocation: <driver executable> init
func (d *FlexVolumeDriver) Init() (map[string]interface{}, error) {
	cred := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if len(cred) == 0 {
		return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS environment variable is not set")
	}
	if _, err := os.Stat(cred); err != nil {
		return nil, fmt.Errorf("cannot stat %s: %s", cred, err)
	}
	// "{\"status\": \"Success\", \"capabilities\": {\"attach\": false}}"
	return map[string]interface{}{
		"capabilities": map[string]bool{
			"attach": false,
		},
	}, nil
}

// initFlexVolumeDriverForMount parse user provided jsonOptions to initialize FlexVolumeDriver.
func (d *FlexVolumeDriver) initFlexVolumeDriverForMount(jsonOptions string) error {
	var volOptions map[string]interface{}
	json.Unmarshal([]byte(jsonOptions), &volOptions)

	if len(volOptions[flexvolume.VolIdKey].(string)) == 0 || len(volOptions[flexvolume.SystemFsTypeKey].(string)) == 0 || len(volOptions[flexvolume.ZoneKey].(string)) == 0 || len(volOptions[flexvolume.ProjectKey].(string)) == 0 {
		return fmt.Errorf("jsonOptions is not set by user properly: %#v", jsonOptions)
	}

	d.volId = volOptions[flexvolume.VolIdKey].(string)
	d.fsType = volOptions[flexvolume.SystemFsTypeKey].(string)
	d.zone = volOptions[flexvolume.ZoneKey].(string)
	d.project = volOptions[flexvolume.ProjectKey].(string)

	return nil
}

// initFlexVolumeDriverForUnMount use targetMountDir to initialize FlexVolumeDriver from tag file.
func (d *FlexVolumeDriver) initFlexVolumeDriverForUnMount(targetMountDir string) error {
	// Use the tag file to store volId since flexvolume will execute fresh new binary every time.
	var optsData flexvolume.FlexVolumeOptsData
	err := flexvolume.ReadJsonOptsFile(targetMountDir, &optsData)
	if err != nil {
		return err
	}

	d.volId = optsData.GCEPDData.VolumeID
	d.zone = optsData.GCEPDData.Zone
	d.project = optsData.GCEPDData.Project

	return nil
}

// Invocation: <driver executable> attach <json options> <node name>
func (d *FlexVolumeDriver) Attach(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return nil, nil
}

// Invocation: <driver executable> detach <mount device> <node name>
func (d *FlexVolumeDriver) Detach(mountDev, nodeName string) (map[string]interface{}, error) {
	return nil, nil
}

// Invocation: <driver executable> waitforattach <mount device> <json options>
func (d *FlexVolumeDriver) WaitForAttach(mountDev, jsonOptions string) (map[string]interface{}, error) {
	return map[string]interface{}{"device": mountDev}, nil
}

// Invocation: <driver executable> isattached <json options> <node name>
func (d *FlexVolumeDriver) IsAttached(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{"attached": true}, nil
}

// Invocation: <driver executable> mount <mount dir> <json options>
// mount persist meta data generated from jsonOptions into a tag file in target dir.
func (d *FlexVolumeDriver) Mount(targetMountDir, jsonOptions string) (map[string]interface{}, error) {
	if err := d.initFlexVolumeDriverForMount(jsonOptions); err != nil {
		return nil, err
	}

	// Step 1: Attach GCE PD to the instance.
	if err := attachDisk(d.project, d.zone, d.volId); err != nil {
		return nil, err
	}

	// Step 2: Format the device.
	if err := formatDisk(d.volId, d.fsType); err != nil {
		detachDiskLogError(d)
		return nil, err
	}

	// Step 3: Create a json file and write metadata into the it.
	optsData := &flexvolume.FlexVolumeOptsData{
		GCEPDData: d.generateOptionsData(),
	}
	if err := flexvolume.WriteJsonOptsFile(targetMountDir, optsData); err != nil {
		os.Remove(targetMountDir)
		detachDiskLogError(d)
		return nil, err
	}

	klog.V(5).Infof("[Mount] GCE PD tag file is created in: %s with data: %s", targetMountDir, optsData)

	return nil, nil
}

// generateOptionsData generates metadata for given GCE PD volume.
func (d *FlexVolumeDriver) generateOptionsData() *flexvolume.GCEPDOptsData {
	return &flexvolume.GCEPDOptsData{
		VolumeID:   d.volId,
		Zone:       d.zone,
		Project:    d.project,
		DevicePath: getDevPathByVolID(d.volId),
		FsType:     d.fsType,
	}
}

// detachDiskLogError is a wrapper to detach first before log error
func detachDiskLogError(d *FlexVolumeDriver) {
	err := detachDisk(d.project, d.zone, d.volId)
	if err != nil {
		klog.Warningf("Failed to detach disk: %v (%v)", d, err)
	}
}

// Invocation: <driver executable> unmount <mount dir>
func (d *FlexVolumeDriver) Unmount(targetMountDir string) (map[string]interface{}, error) {
	// check the target directory
	if _, err := os.Stat(targetMountDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("volume directory: %v does not exist", targetMountDir)
	}

	//  initialize FlexVolumeDriver manager by reading cinderConfig from metadata file
	if err := d.initFlexVolumeDriverForUnMount(targetMountDir); err != nil {
		return nil, err
	}

	if err := detachDisk(d.project, d.zone, d.volId); err != nil {
		return nil, err
	}

	// NOTE: the targetDir will be cleaned by flexvolume,
	// we just need to clean up the metadata file.
	if err := flexvolume.CleanUpMetadataFile(targetMountDir); err != nil {
		return nil, err
	}

	klog.V(5).Infof("[Unmount] GCE PD is detached: %s, and volume folder been cleaned: %s", d.volId, targetMountDir)

	return nil, nil
}
