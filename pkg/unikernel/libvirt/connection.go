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
	"errors"
	"github.com/golang/glog"
	libvirtgo "github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

// ErrDomainNotFound err means can't find specificed domain
// returned by domain loop up methods
var ErrDomainNotFound = errors.New("domain not found")

type LibvirtConnect struct {
	conn *libvirtgo.Connect
}

func NewLibvirtConnect(uri string) (*LibvirtConnect, error) {
	conn, err := libvirtgo.NewConnect(uri)
	if err != nil {
		return nil, err
	}
	return &LibvirtConnect{conn: conn}, nil
}

func (lc *LibvirtConnect) DefineDomain(dxml *libvirtxml.Domain) (*libvirtgo.Domain, error) {
	xml, err := dxml.Marshal()
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Defining domain:%q", xml)
	domain, err := lc.conn.DomainDefineXML(xml)
	if err != nil {
		return nil, err
	}
	return domain, nil
}

func (lc *LibvirtConnect) ListDomains() ([]libvirtgo.Domain, error) {
	domains, err := lc.conn.ListAllDomains(0)
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func (lc *LibvirtConnect) LookupDomainByName(name string) (*libvirtgo.Domain, error) {
	domain, err := lc.conn.LookupDomainByName(name)
	if err != nil {
		libvirtErr, ok := err.(libvirtgo.Error)
		if ok && libvirtErr.Code == libvirtgo.ERR_NO_DOMAIN {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return domain, nil
}

func (lc *LibvirtConnect) LookupDomainByUUIDString(uuid string) (*libvirtgo.Domain, error) {
	domain, err := lc.conn.LookupDomainByUUIDString(uuid)
	if err != nil {
		libvirtErr, ok := err.(libvirtgo.Error)
		if ok && libvirtErr.Code == libvirtgo.ERR_NO_DOMAIN {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return domain, nil
}
