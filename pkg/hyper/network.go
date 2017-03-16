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
	"fmt"
	"github.com/golang/glog"
	"net"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/vishvananda/netlink"
)

// The network information needed to create HyperContainer
// network device from CNI Result
type NetworkInfo struct {
	IfName     string
	Mac        net.HardwareAddr
	Ip         *net.IPNet
	Gateway    string
	BridgeName string
}

func networkInfoFromNs(netns ns.NetNS) *NetworkInfo {
	var result *NetworkInfo

	netns.Do(func(_ ns.NetNS) error {
		result = collectInterfaceInfo()
		return nil
	})

	if result == nil {
		return nil
	}

	br, err := getBridgeNameByIpCompare(result.Ip)
	if err != nil {
		return nil
	}

	result.BridgeName = br

	return result
}

func collectInterfaceInfo() *NetworkInfo {
	links, err := netlink.LinkList()
	if err != nil {
		glog.Errorf("Get link list failed: %v", err)
		return nil
	}

	var result *NetworkInfo
	for _, link := range links {
		if link.Type() != "veth" {
			glog.Infof("Get interface information in container ns, skip non-veth device %s", link.Attrs().Name)
			continue
		}

		name := link.Attrs().Name
		mac := link.Attrs().HardwareAddr

		// TODO: IPv6
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			glog.Errorf("Get address list of link %s failed: %v", name, err)
			continue
		}
		var ip *net.IPNet
		if addrs != nil {
			ip = addrs[0].IPNet
		}

		gateway := ""
		routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
		if err != nil {
			glog.Errorf("Get route list of link %s failed: %v", name, err)
			continue
		}
		for _, route := range routes {
			// Routes lost, retrieve them in the future
			if route.Gw != nil && route.Dst == nil {
				// Only need default gateway right now
				gateway = route.Gw.String()
				break
			}
		}

		result = &NetworkInfo{
			IfName:  name,
			Mac:     mac,
			Ip:      ip,
			Gateway: gateway,
		}

		break
	}

	return result
}

func getBridgeNameByIpCompare(ip *net.IPNet) (string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return "", err
	}

	for _, link := range links {
		if link.Type() != "bridge" {
			glog.Infof("Get bridge name in host net ns, skip non-bridge device %s", link.Attrs().Name)
			continue
		}

		// TODO: IPv6
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if addr.IPNet.Contains(ip.IP) {
				return link.Attrs().Name, nil
			}
		}
	}

	return "", fmt.Errorf("cannot find bridge which ip %s belong to", ip.String())
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
