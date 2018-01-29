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
	"github.com/golang/glog"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v0.beta"

	utilnode "k8s.io/frakti/pkg/util/node"
)

func attachDisk(project string, zone string, volId string, size int64) error {
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
		Boot:       false,
		DeviceName: volId,
		InitializeParams: &compute.AttachedDiskInitializeParams{
			DiskSizeGb: size,
			DiskName:   volId,
		},
	}

	// Use hostname as instance name, which should be correct.
	nodeName := utilnode.GetHostname("")

	if _, err := computeService.Instances.AttachDisk(project, zone, nodeName, rb).Context(ctx).Do(); err != nil {
		return err
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

	mountDev := getDevPathByVolID(volId)

	if _, err := computeService.Instances.DetachDisk(project, zone, nodeName, mountDev).Context(ctx).Do(); err != nil {
		return err
	}

	glog.V(5).Infof("[Detach Device] GCE PD device: %s is detached from node: %s", mountDev, nodeName)

	return nil
}

// getDevPathByVolID returns devicePath on VM for given GCE PD volume ID.
func getDevPathByVolID(volId string) string {
	return "/dev/disk/by-id/google-" + volId
}
