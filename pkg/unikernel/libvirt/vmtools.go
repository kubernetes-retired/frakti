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

package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/golang/glog"
	libvirtgo "github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/frakti/pkg/unikernel/metadata"
)

const (
	DefaultMaxMem  = 32768 // size in MiB
	DefaultMaxCPUs = 8
)

type VMTool struct {
	conn *LibvirtConnect
}

func NewVMTool(conn *LibvirtConnect) *VMTool {
	return &VMTool{
		conn: conn,
	}
}

type VMInfo struct {
	UUID  string
	State libvirtgo.DomainState
}

type VMSetting struct {
	domainName string
	domainUUID string
	enableKVM  bool
	vcpuNum    int
	memory     int
	image      string
}

// NOTE(Crazykev): This method may be changed when support multiple container per Pod.
// CreateContainer creates VM which contains container defined in container spec
func (vt *VMTool) CreateContainer(ctrMeta *metadata.ContainerMetadata, sbMeta *metadata.SandboxMetadata) error {
	settings := VMSetting{
		domainName: sbMeta.Name,
		domainUUID: sbMeta.ID,
		vcpuNum:    int(sbMeta.VMConfig.CPUNum),
		memory:     int(sbMeta.VMConfig.Memory),
		image:      ctrMeta.ImageRef,
		enableKVM:  enableKVM(),
	}
	domainxml, err := vt.createDomain(&settings)
	if err != nil {
		return fmt.Errorf("failed to create domain with config(%v): %v", settings, err)
	}

	if _, err = vt.conn.DefineDomain(domainxml); err != nil {
		return err
	}

	domain, err := vt.conn.LookupDomainByUUIDString(settings.domainUUID)
	if err != nil {
		if domain != nil {
			if err1 := domain.Undefine(); err1 != nil {
				glog.Errorf("Failed to undefine failed domain: %v", err1)
			}
		}
		return err
	}

	return nil
}

// StartVM starts VM by domain UUID
func (vt *VMTool) StartVM(domainID string) error {
	// Get domain
	domain, err := vt.conn.LookupDomainByUUIDString(domainID)
	if err != nil {
		return fmt.Errorf("failed to look up domain(%q): %v", domainID, err)
	}

	// Validate domain state
	state, _, err := domain.GetState()
	if err != nil {
		return fmt.Errorf("failed to get domain(%q) state: %v", domainID, err)
	}
	if state != libvirtgo.DOMAIN_SHUTOFF {
		return fmt.Errorf("unexpected domain(%q) state(%v) when try to StartVM", domainID, state)
	}

	// Create domain
	if err = domain.Create(); err != nil {
		return fmt.Errorf("failed to create domain(%q): %v", domainID, err)
	}

	// Check domain state
	err = wait.PollImmediate(200*time.Millisecond, 10*time.Second, domainRunning(domainID, domain))
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("timeout(10s) waiting for VM(%s) running", domainID)
	}
	return err
}

// StopVM stops VM by domain UUID
func (vt *VMTool) StopVM(domainID string, timeout int64) error {
	domain, err := vt.conn.LookupDomainByUUIDString(domainID)
	if err != nil {
		return err
	}

	if timeout == 0 {
		if err = domain.Destroy(); err != nil {
			return fmt.Errorf("failed to destroy domain(%q): %v", domainID, err)
		}
	} else {
		err = wait.PollImmediate(1*time.Second, time.Duration(timeout)*time.Second, domainStopped(domainID, domain))
		if err != nil {
			glog.Warning("Try to destroy VM(%q) due to shutdown VM failed: %v", domainID, err)
			if err = domain.Destroy(); err != nil {
				return fmt.Errorf("failed to destroy domain(%q): %v", domainID, err)
			}
		}
	}

	// TODO(Crazykev): cleanup other resources

	return nil
}

// RemoveVM stops VM by domain UUID
func (vt *VMTool) RemoveVM(domainID string) error {
	domain, err := vt.conn.LookupDomainByUUIDString(domainID)
	if err != nil && err != ErrDomainNotFound {
		return err
	}

	if domain == nil {
		// TODO(Crazykev): Cleanup resources
		return nil
	}
	state, _, err := domain.GetState()
	if err != nil {
		return fmt.Errorf("failed to get domain(%q) state: %v", domainID, err)
	}
	if state == libvirtgo.DOMAIN_RUNNING {
		// Try to shutdown VM gracefully before try to undefined it
		err = wait.PollImmediate(1*time.Second, 10*time.Second, domainStopped(domainID, domain))
		if err != nil {
			if err = domain.Destroy(); err != nil {
				glog.Warning("failed to destroy domain(%q): %v", domainID, err)
			}
		}
	}
	// Undefine domain
	if err = domain.Undefine(); err != nil {
		return fmt.Errorf("failed to undefine domain(%q): %v", domainID, err)
	}
	domainUndefined := func() wait.ConditionFunc {
		return func() (bool, error) {
			_, err := vt.conn.LookupDomainByUUIDString(domainID)
			if err != nil {
				if err == ErrDomainNotFound {
					return true, nil
				}
				return false, fmt.Errorf("failed to loop up domain(%q): %v", domainID, err)
			}
			return false, nil
		}
	}
	err = wait.PollImmediate(200*time.Millisecond, 5*time.Second, domainUndefined())
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("Failed to wait domain(%q) undefined state: %v", domainID, err)
	}
	return err
}

// ListVMs list all known VMs managed by libvirt
func (vt *VMTool) ListVMs() (map[string]*VMInfo, error) {
	domains, err := vt.conn.ListDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to list all domains: %v", err)
	}
	vms := make(map[string]*VMInfo, len(domains))
	for _, domain := range domains {
		uuid, err := domain.GetUUIDString()
		if err != nil {
			return nil, fmt.Errorf("failed to get domain's uuid: %v", err)
		}
		state, _, err := domain.GetState()
		if err != nil {
			return nil, fmt.Errorf("failed to get domain(%q) state: %v", uuid, err)
		}
		vms[uuid] = &VMInfo{
			UUID:  uuid,
			State: state,
		}
	}
	return vms, nil
}

// GetVMInfo get VM instance info by domain UUID
func (vt *VMTool) GetVMInfo(domainID string) (*VMInfo, error) {
	domain, err := vt.conn.LookupDomainByUUIDString(domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to loop up domain(%q): %v", domainID, err)
	}
	uuid, err := domain.GetUUIDString()
	if err != nil {
		return nil, fmt.Errorf("failed to get domain's uuid: %v", err)
	}
	state, _, err := domain.GetState()
	if err != nil {
		return nil, fmt.Errorf("failed to get domain(%q) state: %v", uuid, err)
	}
	return &VMInfo{
		UUID:  uuid,
		State: state,
	}, nil
}

func enableKVM() bool {
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return false
	}
	return true
}

func domainStopped(domainID string, domain *libvirtgo.Domain) wait.ConditionFunc {
	return func() (bool, error) {
		// Try to shutdown domain
		errShutdown := domain.Shutdown()

		// Check domain state
		state, _, err := domain.GetState()
		if err != nil {
			return false, fmt.Errorf("failed to get state of domain(%q)", domainID)
		}
		if state == libvirtgo.DOMAIN_SHUTOFF {
			return true, nil
		}

		// Ignore shutdown error if we find out domain already shutdown
		if errShutdown != nil {
			return false, fmt.Errorf("failed to shutdown domain(%q): %v", domainID, errShutdown)
		}

		return false, nil
	}
}

func domainRunning(domainID string, domain *libvirtgo.Domain) wait.ConditionFunc {
	return func() (bool, error) {
		state, _, err := domain.GetState()
		if err != nil {
			return false, fmt.Errorf("failed to get domain(%q) state: %v", domainID, err)
		}
		switch state {
		case libvirtgo.DOMAIN_RUNNING:
			return true, nil
		case libvirtgo.DOMAIN_SHUTDOWN:
			return false, fmt.Errorf("unexpected shutdown for new created domain(%q)", domainID)
		case libvirtgo.DOMAIN_CRASHED:
			return false, fmt.Errorf("unexpected domain(%q) crash on start", domainID)
		default:
			return false, nil
		}
	}
}

func (vt *VMTool) createDomain(setting *VMSetting) (*libvirtxml.Domain, error) {
	scsiControllerIndex := uint(0)
	// TODO(Crazykev): use a wrapper emulator
	emulator, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return nil, fmt.Errorf("find qemu-system-x86_64 binary failed: %v", err)
	}
	imageDiskDomainIndex := uint(0)
	imageDiskBusIndex := uint(1)
	imageDiskSlotIndex := uint(1)
	domain := &libvirtxml.Domain{
		Type: "kvm",
		Name: setting.domainName,
		UUID: setting.domainUUID,
		Memory: &libvirtxml.DomainMemory{
			Unit:  "MiB",
			Value: uint(setting.memory),
		},
		MaximumMemory: &libvirtxml.DomainMaxMemory{
			Unit:  "MiB",
			Value: DefaultMaxMem,
			Slots: 1,
		},
		VCPU: &libvirtxml.DomainVCPU{
			Placement: "static",
			Current:   strconv.Itoa(setting.vcpuNum),
			Value:     DefaultMaxCPUs,
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Arch:    "x86_64",
				Machine: "pc-i440fx-2.1",
				Type:    "hvm",
			},
			BootDevices: []libvirtxml.DomainBootDevice{
				{Dev: "hd"},
			},
		},
		CPU: &libvirtxml.DomainCPU{
			Mode: "host-passthrough",
			Numa: &libvirtxml.DomainNuma{
				Cell: []libvirtxml.DomainCell{
					{
						ID:     "0",
						CPUs:   fmt.Sprintf("0-%d", DefaultMaxCPUs-1),
						Memory: strconv.Itoa(setting.memory * 1024), // older libvirt always considers unit='KiB'
						Unit:   "KiB",
					},
				},
			},
		},
		Clock: &libvirtxml.DomainClock{
			Offset: "utc",
			Timer: []libvirtxml.DomainTimer{
				{Name: "rtc", Track: "guest", Tickpolicy: "catchup"},
			},
		},
		Devices: &libvirtxml.DomainDeviceList{
			Emulator: emulator,
			Inputs: []libvirtxml.DomainInput{
				{Type: "tablet", Bus: "usb"},
			},
			Graphics: []libvirtxml.DomainGraphic{
				{Type: "vnc", Port: -1},
			},
			Videos: []libvirtxml.DomainVideo{
				{Model: libvirtxml.DomainVideoModel{Type: "cirrus"}},
			},
			Controllers: []libvirtxml.DomainController{
				{Type: "scsi", Index: &scsiControllerIndex, Model: "virtio-scsi"},
			},
			// Use this hard coded disk for test
			Disks: []libvirtxml.DomainDisk{
				{Type: "file", Device: "disk",
					Driver: &libvirtxml.DomainDiskDriver{Name: "qemu", Type: "qcow2"},
					Source: &libvirtxml.DomainDiskSource{File: setting.image},
					Target: &libvirtxml.DomainDiskTarget{Dev: "vda", Bus: "virtio"},
					Address: &libvirtxml.DomainAddress{Type: "pci", Domain: &imageDiskDomainIndex,
						Bus: &imageDiskBusIndex, Slot: &imageDiskSlotIndex},
				},
			},
		},
		OnPoweroff: "destroy",
		OnReboot:   "destroy",
		OnCrash:    "destroy",
	}

	if !setting.enableKVM {
		domain.Type = "qemu"
		domain.CPU.Mode = "host-model"
		domain.CPU.Match = "exact"
		domain.CPU.Model = &libvirtxml.DomainCPUModel{
			Fallback: "allow",
			Value:    "core2duo",
		}
	}
	return domain, nil
}
