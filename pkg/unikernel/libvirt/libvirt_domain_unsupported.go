// +build !with_libvirt

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
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

// LibvirtConnect is a wrapper for libvirt connection
type LibvirtConnect struct {
}

// NewLibvirtConnect create LibvirtConnect by libvirt uri
func NewLibvirtConnect(uri string) (*LibvirtConnect, error) {
	return nil, fmt.Errorf("not supported")
}

// DefineDomain define domain with domain setting xml
func (lc *LibvirtConnect) DefineDomain(dxml *libvirtxml.Domain) (*LibvirtDomain, error) {
	return nil, fmt.Errorf("not supported")
}

// ListDomains get all domains managed by libvirt, including those not managed by unikernel runtime.
func (lc *LibvirtConnect) ListDomains() ([]*LibvirtDomain, error) {
	return nil, fmt.Errorf("not supported")
}

// GetDomainByName get domain from libvirt by domian name
func (lc *LibvirtConnect) GetDomainByName(name string) (*LibvirtDomain, error) {
	return nil, fmt.Errorf("not supported")
}

// GetDomainByUUIDString get domain from libvirt by domain UUID
func (lc *LibvirtConnect) GetDomainByUUIDString(uuid string) (*LibvirtDomain, error) {
	return nil, fmt.Errorf("not supported")
}

// LibvirtDomain is a wrapper for libvirt-go Domain
type LibvirtDomain struct {
}

// Create creates libvirt domain
func (ld *LibvirtDomain) Create() error {
	return fmt.Errorf("not supported")
}

// Destroy destroy libvirt domain
func (ld *LibvirtDomain) Destroy() error {
	return fmt.Errorf("not supported")
}

// Shutdown shutdown libvirt domain
func (ld *LibvirtDomain) Shutdown() error {
	return fmt.Errorf("not supported")
}

// Undefine undefines libvirt domain
func (ld *LibvirtDomain) Undefine() error {
	return fmt.Errorf("not supported")
}

// GetUUIDString return uuid of libvirt domain
func (ld *LibvirtDomain) GetUUIDString() (string, error) {
	return "", fmt.Errorf("not supported")
}

// GetName gets name of libvirt domain
func (ld *LibvirtDomain) GetName() (string, error) {
	return "", fmt.Errorf("not supported")
}

// GetState get domain state
func (ld *LibvirtDomain) GetState() (DomainState, error) {
	return DOMAIN_NOSTATE, fmt.Errorf("not supported")
}
