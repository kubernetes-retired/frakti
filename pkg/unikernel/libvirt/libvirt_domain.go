// +build with_libvirt

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
	"github.com/golang/glog"
	libvirtgo "github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

// LibvirtConnect is a wrapper for libvirt connection
type LibvirtConnect struct {
	conn *libvirtgo.Connect
}

// NewLibvirtConnect create LibvirtConnect by libvirt uri
func NewLibvirtConnect(uri string) (*LibvirtConnect, error) {
	conn, err := libvirtgo.NewConnect(uri)
	if err != nil {
		return nil, err
	}
	return &LibvirtConnect{conn: conn}, nil
}

// DefineDomain define domain with domain setting xml
func (lc *LibvirtConnect) DefineDomain(dxml *libvirtxml.Domain) (*LibvirtDomain, error) {
	xml, err := dxml.Marshal()
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Defining domain:%q", xml)
	domain, err := lc.conn.DomainDefineXML(xml)
	if err != nil {
		return nil, err
	}
	return &LibvirtDomain{domain}, nil
}

// ListDomains get all domains managed by libvirt, including those not managed by unikernel runtime.
func (lc *LibvirtConnect) ListDomains() ([]*LibvirtDomain, error) {
	domains, err := lc.conn.ListAllDomains(0)
	if err != nil {
		return nil, err
	}
	ldomains := make([]*LibvirtDomain, len(domains))
	for n, d := range domains {
		current := d
		ldomains[n] = &LibvirtDomain{&current}
	}
	return ldomains, nil
}

// GetDomainByName get domain from libvirt by domian name
func (lc *LibvirtConnect) GetDomainByName(name string) (*LibvirtDomain, error) {
	domain, err := lc.conn.LookupDomainByName(name)
	if err != nil {
		libvirtErr, ok := err.(libvirtgo.Error)
		if ok && libvirtErr.Code == libvirtgo.ERR_NO_DOMAIN {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return &LibvirtDomain{domain}, nil
}

// GetDomainByUUIDString get domain from libvirt by domain UUID
func (lc *LibvirtConnect) GetDomainByUUIDString(uuid string) (*LibvirtDomain, error) {
	domain, err := lc.conn.LookupDomainByUUIDString(uuid)
	if err != nil {
		libvirtErr, ok := err.(libvirtgo.Error)
		if ok && libvirtErr.Code == libvirtgo.ERR_NO_DOMAIN {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return &LibvirtDomain{domain}, nil
}

// LibvirtDomain is a wrapper for libvirt-go Domain
type LibvirtDomain struct {
	domain *libvirtgo.Domain
}

// Create creates libvirt domain
func (ld *LibvirtDomain) Create() error {
	return ld.domain.Create()
}

// Destroy destroy libvirt domain
func (ld *LibvirtDomain) Destroy() error {
	return ld.domain.Destroy()
}

// Shutdown shutdown libvirt domain
func (ld *LibvirtDomain) Shutdown() error {
	return ld.domain.Shutdown()
}

// Undefine undefines libvirt domain
func (ld *LibvirtDomain) Undefine() error {
	return ld.domain.Undefine()
}

// GetUUIDString return uuid of libvirt domain
func (ld *LibvirtDomain) GetUUIDString() (string, error) {
	return ld.domain.GetUUIDString()
}

// GetName gets name of libvirt domain
func (ld *LibvirtDomain) GetName() (string, error) {
	return ld.domain.GetName()
}

// GetState get domain state
func (ld *LibvirtDomain) GetState() (DomainState, error) {
	state, _, err := ld.domain.GetState()
	if err != nil {
		return DOMAIN_NOSTATE, err
	}
	switch state {
	case libvirtgo.DOMAIN_NOSTATE:
		return DOMAIN_NOSTATE, nil
	case libvirtgo.DOMAIN_RUNNING:
		return DOMAIN_RUNNING, nil
	case libvirtgo.DOMAIN_BLOCKED:
		return DOMAIN_BLOCKED, nil
	case libvirtgo.DOMAIN_PAUSED:
		return DOMAIN_PAUSED, nil
	case libvirtgo.DOMAIN_SHUTDOWN:
		return DOMAIN_SHUTDOWN, nil
	case libvirtgo.DOMAIN_CRASHED:
		return DOMAIN_CRASHED, nil
	case libvirtgo.DOMAIN_PMSUSPENDED:
		return DOMAIN_PMSUSPENDED, nil
	case libvirtgo.DOMAIN_SHUTOFF:
		return DOMAIN_SHUTOFF, nil
	default:
		return DOMAIN_NOSTATE, fmt.Errorf("unknow domain state %v", state)
	}
}
