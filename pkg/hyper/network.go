/*
Copyright 2016 The Kubernetes Authors.

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

package hyper

import (
	"github.com/golang/glog"

	"github.com/containernetworking/cni/pkg/ns"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/vishvananda/netlink"
)

// The network information needed to create HyperContainer
// network device from CNI Result
type NetworkInfo struct {
	IfName     string
	Mac        string
	Ip         string
	Gateway    string
	BridgeName string
}

func convertCNIResult2NetworkInfo(result cnitypes.Result) *NetworkInfo {
	ret := &NetworkInfo{}

	r, err := current.GetResult(result)
	if err != nil {
		glog.Errorf("Convert CNI Result failed: %v", err)
		return nil
	}

	// only handle bridge plugin now
	// TODO: support other plugins in the future
	for _, iface := range r.Interfaces {
		if iface.Sandbox != "" {
			// interface information in net ns
			ret.IfName = iface.Name
			ret.Mac = iface.Mac
			continue
		}
		l, err := netlink.LinkByName(iface.Name)
		if err != nil {
			continue
		}
		if _, ok := l.(*netlink.Bridge); ok {
			// find the bridge name
			ret.BridgeName = iface.Name
		}
	}
	ret.Ip = r.IPs[0].Address.String()
	ret.Gateway = r.IPs[0].Gateway.String()

	return ret
}

func setDownLinksInNs(netns ns.NetNS) error {
	err := netns.Do(func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			netlink.LinkSetDown(link)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
