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

package rbd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"k8s.io/frakti/pkg/flexvolume"
	"k8s.io/klog"
)

type FlexVolumeDriver struct {
	uuid string
	name string

	// Options from flexvolume
	volId  string
	pool   string
	fsType string
}

// NewFlexVolumeDriver returns a flex volume driver
func NewFlexVolumeDriver(uuid string, name string) *FlexVolumeDriver {
	return &FlexVolumeDriver{
		uuid: uuid,
		name: name,
	}
}

// Invocation: <driver executable> init
// Check config file existence.
func (d *FlexVolumeDriver) Init() (map[string]interface{}, error) {
	if _, err := os.Stat(flexvolume.CephRBDConfigFile); err != nil {
		return nil, fmt.Errorf("cannot stat %s: %s", flexvolume.CephRBDConfigFile, err)
	}
	if _, err := os.Stat(flexvolume.CephRBDKeyringFile); err != nil {
		return nil, fmt.Errorf("cannot stat %s: %s", flexvolume.CephRBDKeyringFile, err)
	}
	if _, err := exec.LookPath(rbdBin); err != nil {
		return nil, fmt.Errorf("rbd command not found")
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

	if volOptions[flexvolume.VolIdKey] == nil || volOptions[flexvolume.SystemFsTypeKey] == nil ||
		len(volOptions[flexvolume.VolIdKey].(string)) == 0 || len(volOptions[flexvolume.SystemFsTypeKey].(string)) == 0 {
		return fmt.Errorf("jsonOptions is not set by user properly: %#v", jsonOptions)
	}

	d.volId = volOptions[flexvolume.VolIdKey].(string)
	d.fsType = volOptions[flexvolume.SystemFsTypeKey].(string)

	if volOptions[flexvolume.PoolKey] != nil && len(volOptions[flexvolume.PoolKey].(string)) != 0 {
		d.pool = volOptions[flexvolume.PoolKey].(string)
	} else {
		d.pool = flexvolume.DefaultCephRBDPool
	}

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

	// Step 1: Format the device.
	if err := formatDisk(d.volId, d.pool, d.fsType); err != nil {
		return nil, err
	}

	// Step 2: Create a json file and write metadata into the it.
	data, err := d.generateOptionsData()
	if err != nil {
		return nil, err
	}

	optsData := &flexvolume.FlexVolumeOptsData{
		CephRBDData: data,
	}

	if err := flexvolume.WriteJsonOptsFile(targetMountDir, optsData); err != nil {
		os.Remove(targetMountDir)
		return nil, err
	}

	klog.V(5).Infof("[Mount] Ceph RBD tag file is created in: %s with data: %s", targetMountDir, optsData)

	return nil, nil
}

// generateOptionsData generates metadata for given ceph rbd volume.
func (d *FlexVolumeDriver) generateOptionsData() (*flexvolume.CephRBDOptsData, error) {
	user, keyring, monitors, err := readCephConfig(flexvolume.CephRBDConfigFile, flexvolume.CephRBDKeyringFile)
	if err != nil {
		return nil, err
	}

	return &flexvolume.CephRBDOptsData{
		VolumeID: d.volId,
		Pool:     d.pool,
		FsType:   d.fsType,
		User:     user,
		Keyring:  keyring,
		Monitors: monitors,
	}, nil
}

// Invocation: <driver executable> unmount <mount dir>
func (d *FlexVolumeDriver) Unmount(targetMountDir string) (map[string]interface{}, error) {
	// check the target directory
	if _, err := os.Stat(targetMountDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("volume directory: %v does not exist", targetMountDir)
	}

	// NOTE: the targetDir will be cleaned by flexvolume,
	// we just need to clean up the metadata file.
	if err := flexvolume.CleanUpMetadataFile(targetMountDir); err != nil {
		return nil, err
	}

	klog.V(5).Infof("[Unmount] Ceph RBD umounted: %s, and volume folder been cleaned: %s", d.volId, targetMountDir)

	return nil, nil
}
