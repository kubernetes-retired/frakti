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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v0.beta"

	"k8s.io/apimachinery/pkg/util/wait"
	utilnode "k8s.io/frakti/pkg/util/node"
	"k8s.io/utils/exec"
)

const (
	diskPollInterval = 100 * time.Millisecond
	diskCheckTimeout = 5 * time.Second
)

func attachDisk(project string, zone string, volId string) error {
	ctx := context.Background()

	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		return err
	}

	computeService, err := compute.New(c)
	if err != nil {
		return err
	}

	rb := &compute.AttachedDisk{
		// Use default disk type, which is PD.
		Boot:       false,
		Source:     buildDiskURL(project, zone, volId),
		DeviceName: volId,
	}

	// Use hostname as instance name, which should be correct.
	nodeName := utilnode.GetHostname("")

	if _, err := computeService.Instances.AttachDisk(project, zone, nodeName, rb).Context(ctx).Do(); err != nil {
		return err
	}

	// Check if device appears on host.
	if err := waitForDiskAttach(volId); err != nil {
		return fmt.Errorf("failed to wait for disk appear on host: %v", err)
	}

	// TODO(harry): Check status of resp?
	glog.V(5).Infof("[Attach Device] GCE PD: %s is attached to node: %s", volId, nodeName)

	return nil
}

func detachDisk(project string, zone string, volId string) error {
	ctx := context.Background()

	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		return err
	}

	computeService, err := compute.New(c)
	if err != nil {
		return err
	}

	// Use hostname as instance name, which should be correct.
	nodeName := utilnode.GetHostname("")

	if _, err := computeService.Instances.DetachDisk(project, zone, nodeName, volId).Context(ctx).Do(); err != nil {
		return err
	}

	// Check if device appears on host.
	if err := waitForDiskDetach(volId); err != nil {
		return fmt.Errorf("failed to wait for disk disappear on host: %v", err)
	}

	glog.V(5).Infof("[Detach Device] GCE PD device: %s is detached from node: %s", volId, nodeName)

	return nil
}

func waitForDiskAttach(volId string) error {
	devicePath := getDevPathByVolID(volId)
	return wait.Poll(diskPollInterval, diskCheckTimeout, func() (bool, error) {
		if _, err := os.Stat(devicePath); err == nil {
			return true, nil
		}
		return false, nil
	})
}

func waitForDiskDetach(volId string) error {
	devicePath := getDevPathByVolID(volId)
	return wait.Poll(diskPollInterval, diskCheckTimeout, func() (bool, error) {
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			return true, nil
		}
		return false, nil
	})
}

// getDevPathByVolID returns devicePath on VM for given GCE PD volume ID.
func getDevPathByVolID(volId string) string {
	return "/dev/disk/by-id/google-" + volId
}

// formatDisk check the device status and format it if needed.
func formatDisk(volId, fstype string) error {
	source := getDevPathByVolID(volId)

	existingFormat, err := getDiskFormat(source)
	if err != nil {
		return err
	}

	if existingFormat == "" {
		// Disk is unformatted so format it.
		args := []string{source}
		// Use 'ext4' as the default
		if len(fstype) == 0 {
			fstype = "ext4"
		}

		if fstype == "ext4" || fstype == "ext3" {
			args = []string{"-F", source}
		}
		glog.Infof("Disk %q appears to be unformatted, attempting to format as type: %q with options: %v", source, fstype, args)
		_, err := execRun("mkfs."+fstype, args...)
		if err != nil {
			glog.Errorf("format of disk %q failed: type:(%q) error:(%v)", source, fstype, err)
			return err
		}
	}

	glog.V(5).Infof("[Format Device] GCE PD device: %s is formatted with type: %s", source, fstype)

	return nil
}

func execRun(cmd string, args ...string) ([]byte, error) {
	exe := exec.New()
	return exe.Command(cmd, args...).CombinedOutput()
}

func buildDiskURL(project, zone, volID string) string {
	return fmt.Sprintf(
		"https://www.googleapis.com/compute/v1/projects/%s/zones/%s/disks/%s",
		project, zone, volID,
	)
}

// getDiskFormat uses 'lsblk' to return the format of given disk.
func getDiskFormat(disk string) (string, error) {
	args := []string{"-n", "-o", "FSTYPE", disk}
	glog.V(4).Infof("Attempting to determine if disk %q is formatted using lsblk with args: (%v)", disk, args)
	dataOut, err := execRun("lsblk", args...)
	output := string(dataOut)
	glog.V(4).Infof("Output: %q", output)

	if err != nil {
		glog.Errorf("Could not determine if disk %q is formatted (%v)", disk, err)
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
