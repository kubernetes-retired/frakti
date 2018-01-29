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
	"errors"
	"fmt"
	"os"

	"github.com/golang/glog"
	"k8s.io/frakti/pkg/util/knownflags"
	utilmetadata "k8s.io/frakti/pkg/util/metadata"
)

type FlexVolumeDriver struct {
	uuid string
	name string

	// Options from flexvolume
	volId   string
	size    int64
	project string
	zone    string
	fsType  string
}

// NewFlexVolumeDriver returns a flex volume driver
func NewFlexVolumeDriver(uuid string, name string) *FlexVolumeDriver {
	return &FlexVolumeDriver{
		uuid: uuid,
		name: name,
	}
}

// Invocation: <driver executable> init
func (d *FlexVolumeDriver) init() (map[string]interface{}, error) {
	// "{\"status\": \"Success\""
	return map[string]interface{}{}, nil
}

// initFlexVolumeDriverForMount parse user provided jsonOptions to initialize FlexVolumeDriver.
func (d *FlexVolumeDriver) initFlexVolumeDriverForMount(jsonOptions string) error {
	var volOptions map[string]interface{}
	json.Unmarshal([]byte(jsonOptions), &volOptions)

	if len(volOptions[knownflags.VolIdKey].(string)) == 0 {
		return fmt.Errorf("jsonOptions is not set by user properly: %#v", jsonOptions)
	}

	d.volId = volOptions[knownflags.VolIdKey].(string)
	d.fsType = volOptions[knownflags.SystemFsTypeKey].(string)
	d.zone = volOptions[knownflags.ZoneKey].(string)
	d.size = volOptions[knownflags.SizeKey].(int64)
	d.project = volOptions[knownflags.ProjectKey].(string)

	return nil
}

// initFlexVolumeDriverForUnMount use targetMountDir to initialize FlexVolumeDriver from tag file.
func (d *FlexVolumeDriver) initFlexVolumeDriverForUnMount(targetMountDir string) error {
	// Use the tag file to store volId since flexvolume will execute fresh new binary every time.
	optsData, err := utilmetadata.ReadJsonOptsFile(targetMountDir)
	if err != nil {
		return err
	}

	d.volId = optsData[knownflags.VolIdKey].(string)
	d.zone = optsData[knownflags.ZoneKey].(string)
	d.project = optsData[knownflags.ProjectKey].(string)

	return nil
}

// Invocation: <driver executable> attach <json options> <node name>
func (d *FlexVolumeDriver) attach(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{"device": getDevPathByVolID(d.volId)}, nil
}

// Invocation: <driver executable> detach <mount device> <node name>
func (d *FlexVolumeDriver) detach(mountDev, nodeName string) (map[string]interface{}, error) {
	return nil, nil
}

// Invocation: <driver executable> waitforattach <mount device> <json options>
func (d *FlexVolumeDriver) waitForAttach(mountDev, jsonOptions string) (map[string]interface{}, error) {
	return nil, nil
}

// Invocation: <driver executable> isattached <json options> <node name>
func (d *FlexVolumeDriver) isAttached(jsonOptions, nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{"attached": true}, nil
}

// Invocation: <driver executable> mount <mount dir> <json options>
// mount persist meta data generated from jsonOptions into a tag file in target dir.
func (d *FlexVolumeDriver) mount(targetMountDir, jsonOptions string) (map[string]interface{}, error) {
	if err := d.initFlexVolumeDriverForMount(jsonOptions); err != nil {
		return nil, err
	}

	// Step 1: Attach GCE PD to the instance.
	if err := attachDisk(d.project, d.zone, d.volId, d.size); err != nil {
		return nil, err
	}

	// Step 2: Create a json file and write metadata into the it.
	optsData := d.generateOptionsData()
	if err := utilmetadata.WriteJsonOptsFile(targetMountDir, optsData); err != nil {
		os.Remove(targetMountDir)
		detachDiskLogError(d)
		return nil, err
	}

	glog.V(5).Infof("[Mount] GCE PD tag file is created in: %s with data: %s", targetMountDir, optsData)

	return nil, nil
}

// generateOptionsData generates metadata for given GCE PD volume.
func (d *FlexVolumeDriver) generateOptionsData() map[string]interface{} {
	optsData := map[string]interface{}{}

	optsData[knownflags.VolIdKey] = d.volId
	optsData[knownflags.FsTypeKey] = d.fsType
	optsData[knownflags.ZoneKey] = d.zone
	optsData[knownflags.ProjectKey] = d.project

	optsData[knownflags.DivcePathKey] = getDevPathByVolID(d.volId)

	return optsData
}

// detachDiskLogError is a wrapper to detach first before log error
func detachDiskLogError(d *FlexVolumeDriver) {
	err := detachDisk(d.project, d.zone, d.volId)
	if err != nil {
		glog.Warningf("Failed to detach disk: %v (%v)", d, err)
	}
}

// Invocation: <driver executable> unmount <mount dir>
func (d *FlexVolumeDriver) unmount(targetMountDir string) (map[string]interface{}, error) {
	// check the target directory
	if _, err := os.Stat(targetMountDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("volume directory: %v does not exists", targetMountDir)
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
	if err := utilmetadata.CleanUpMetadataFile(targetMountDir); err != nil {
		return nil, err
	}

	return nil, nil
}

type driverOp func(*FlexVolumeDriver, []string) (map[string]interface{}, error)

type cmdInfo struct {
	numArgs int
	run     driverOp
}

var commands = map[string]cmdInfo{
	"init": {
		0, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.init()
		},
	},
	"attach": {
		2, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.attach(args[0], args[1])
		},
	},
	"detach": {
		2, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.detach(args[0], args[1])
		},
	},
	"waitforattach": {
		2, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.waitForAttach(args[0], args[1])
		},
	},
	"isattached": {
		2, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.isAttached(args[0], args[1])
		},
	},
	"mount": {
		2, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.mount(args[0], args[1])
		},
	},
	"unmount": {
		1, func(d *FlexVolumeDriver, args []string) (map[string]interface{}, error) {
			return d.unmount(args[0])
		},
	},
}

func (d *FlexVolumeDriver) doRun(args []string) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments passed to flexvolume driver")
	}
	nArgs := len(args) - 1
	op := args[0]
	if cmdInfo, found := commands[op]; found {
		if cmdInfo.numArgs == nArgs {
			return cmdInfo.run(d, args[1:])
		} else {
			return nil, fmt.Errorf("unexpected number of args %d (expected %d) for operation %q", nArgs, cmdInfo.numArgs, op)
		}
	} else {
		return map[string]interface{}{
			"status": "Not supported",
		}, nil
	}
}

func (d *FlexVolumeDriver) Run(args []string) string {
	r := formatResult(d.doRun(args))
	return r
}

func formatResult(fields map[string]interface{}, err error) string {
	var data map[string]interface{}
	if err != nil {
		data = map[string]interface{}{
			"status":  "Failure",
			"message": err.Error(),
		}
	} else {
		data = map[string]interface{}{
			"status": "Success",
		}
		for k, v := range fields {
			data[k] = v
		}
	}
	s, err := json.Marshal(data)
	if err != nil {
		panic("error marshalling the data")
	}
	return string(s) + "\n"
}
