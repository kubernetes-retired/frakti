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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var defaultCinderVolume = &CinderVolumeOptsData{
	ConfigKey:  "emptyKey",
	VolumeID:   "fooVolume",
	VolumeType: "rbd",
	Name:       "volumeNameBar",
	FsType:     "fstype_empty",
	Hosts:      []string{"host1", "host2"},
	Ports:      []string{"port1", "port2"},
}

var defaultGcePdVolume = &GCEPDOptsData{
	VolumeID:   "volumebar",
	Zone:       "earch",
	Project:    "frakti",
	DevicePath: "Aroad",
	FsType:     "empty",
}

const testDir = "/tmp"

func TestSaveLoadCinderOptsData(t *testing.T) {
	src := defaultCinderVolume

	err := WriteJsonOptsFile(testDir, src)
	assert.Nil(t, err, "write cinder json")

	var dst CinderVolumeOptsData
	err = ReadJsonOptsFile(testDir, &dst)
	assert.Nil(t, err, "read cinder json")

	assert.True(t, reflect.DeepEqual(*src, dst))
}

func TestSaveLoadGCEPDOptsData(t *testing.T) {
	src := defaultGcePdVolume

	err := WriteJsonOptsFile(testDir, src)
	assert.Nil(t, err, "write gcepd json")

	var dst GCEPDOptsData
	err = ReadJsonOptsFile(testDir, &dst)
	assert.Nil(t, err, "read gcepd json")

	assert.True(t, reflect.DeepEqual(*src, dst))
}

func TestSaveLoadFlexVolumeOptsData(t *testing.T) {
	src := &FlexVolumeOptsData{
		CinderData: defaultCinderVolume,
		GCEPDData:  defaultGcePdVolume,
	}

	err := WriteJsonOptsFile(testDir, src)
	assert.Nil(t, err, "write flex volume json")

	var dst FlexVolumeOptsData
	err = ReadJsonOptsFile(testDir, &dst)
	assert.Nil(t, err, "read flex volume json")

	assert.True(t, reflect.DeepEqual(*src, dst))
}
