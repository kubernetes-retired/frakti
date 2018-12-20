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
	"fmt"
	"os/exec"
	"strings"

	"github.com/Unknwon/goconfig"
	"k8s.io/klog"
)

const (
	rbdBin = "rbd"
)

type rbdDevice struct {
	rbdBinPath string
	volId      string
	pool       string
	fstype     string
	mappedDev  string
}

func newRbdDevice(volId, pool, fstype string) (*rbdDevice, error) {
	rbdPath, err := exec.LookPath(rbdBin)
	if err != nil {
		return nil, fmt.Errorf("cannot find %s: %v", rbdBin, err)
	}

	return &rbdDevice{
		rbdBinPath: rbdPath,
		volId:      volId,
		pool:       pool,
		fstype:     fstype,
	}, nil
}

func (r *rbdDevice) mapDevice() (string, error) {
	klog.V(4).Infof("map device %s", r.volId)
	if len(r.mappedDev) != 0 {
		return r.mappedDev, nil
	}

	mappedDeviceByte, err := exec.Command(r.rbdBinPath, "map", r.volId, "-p", r.pool).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("fail to map rbd: %v", err)
	}

	r.mappedDev = strings.TrimSpace(string(mappedDeviceByte))
	klog.V(4).Infof("volume %s mapped as %s", r.volId, r.mappedDev)

	return r.mappedDev, nil
}

func (r *rbdDevice) unmapDevice() error {
	klog.V(4).Infof("unmap device %s", r.mappedDev)
	if len(r.mappedDev) == 0 {
		return nil
	}

	_, err := exec.Command(r.rbdBinPath, "unmap", r.mappedDev).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to unmap rbd: %v", err)
	}
	r.mappedDev = ""

	return nil
}

// formatDisk check the device status and format it if needed.
func formatDisk(volId, pool, fstype string) error {
	klog.V(4).Infof("format volume %s pool %s as %s", volId, pool, fstype)

	r, err := newRbdDevice(volId, pool, fstype)
	if err != nil {
		return err
	}

	// Map rbd locally
	device, err := r.mapDevice()
	if err != nil {
		return err
	}
	defer r.unmapDevice()

	// Check existing format
	existingFormat, err := getDiskFormat(device)
	if err != nil {
		return err
	}

	// Format disk
	if len(existingFormat) == 0 {
		// Disk is unformatted so format it.
		args := []string{device}
		if fstype == "ext4" || fstype == "ext3" {
			args = []string{"-F", device}
		}
		klog.V(4).Infof("Disk %q appears to be unformatted, attempting to format as type: %q with options: %v", device, fstype, args)
		_, err := exec.Command("mkfs."+fstype, args...).CombinedOutput()
		if err != nil {
			klog.Errorf("format of disk %q failed: type:(%q) error:(%v)", device, fstype, err)
			return err
		}
		klog.V(5).Infof("[Format Device] rbd device %s is formatted with type: %s", device, fstype)
	}

	return nil
}

// getDiskFormat uses 'lsblk' to return the format of given disk.
func getDiskFormat(disk string) (string, error) {
	args := []string{"-n", "-o", "FSTYPE", disk}
	klog.V(4).Infof("Attempting to determine if disk %q is formatted using lsblk with args: (%v)", disk, args)
	dataOut, err := exec.Command("lsblk", args...).CombinedOutput()
	output := string(dataOut)
	klog.V(4).Infof("Output: %q", output)

	if err != nil {
		klog.Errorf("Could not determine if disk %q is formatted (%v)", disk, err)
		return "", err
	}

	// Split lsblk output into lines. Unformatted devices should contain only
	// "\n". Beware of "\n\n", that's a device with one empty partition.
	output = strings.TrimSuffix(output, "\n") // Avoid last empty line
	lines := strings.Split(output, "\n")
	if lines[0] != "" {
		// The device is formatted
		return lines[0], nil
	}

	if len(lines) == 1 {
		// The device is unformatted and has no dependent devices
		return "", nil
	}

	// The device has dependent devices, most probably partitions (LVM, LUKS
	// and MD RAID are reported as FSTYPE and caught above).
	return "unknown data, probably partitions", nil
}

// readCephConfig parses the ceph config file and the kerying file
func readCephConfig(configFile, keyringFile string) (string, string, []string, error) {
	rbdcfg, err := goconfig.LoadConfigFile(configFile)
	if err != nil {
		klog.Errorf("Read config file (%s) failed, %s", configFile, err)
		return "", "", []string{}, err
	}

	monitors, err := rbdcfg.GetValue("global", "mon_host")
	if err != nil {
		return "", "", []string{}, err
	}

	keycfg, err := goconfig.LoadConfigFile(keyringFile)
	if err != nil {
		klog.Errorf("Read keyring config file (%s) failed, %s", keyringFile, err)
		return "", "", []string{}, err
	}

	keysecs := keycfg.GetSectionList()
	if len(keysecs) != 1 {
		return "", "", []string{}, fmt.Errorf("keyring config format error: no user section")
	}

	user := "admin"
	if users := strings.Split(keysecs[0], "."); len(users) == 2 {
		user = users[1]
	}

	keyring, err := keycfg.GetValue(keysecs[0], "key")
	if err != nil {
		return "", "", []string{}, err
	}

	return user, keyring, strings.Split(monitors, ","), nil
}
