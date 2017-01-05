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
	"net"

	"github.com/golang/glog"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/vishvananda/netlink"
)

type NetNsInfos struct {
	Interfaces []*InterfaceInfo
}

type InterfaceInfo struct {
	Ip      string
	Gateway string
	Mac     string
}

func getNetNsInfos(netns ns.NetNS) *NetNsInfos {
	result := &NetNsInfos{}

	var infos []*InterfaceInfo
	netns.Do(func(_ ns.NetNS) error {
		infos = collectInterfaceInfos()
		return nil
	})
	if len(infos) != 0 {
		result.Interfaces = infos
	}

	return result
}

func collectInterfaceInfos() []*InterfaceInfo {
	infos := []*InterfaceInfo{}

	links, err := netlink.LinkList()
	if err != nil {
		return infos
	}

	for _, link := range links {
		if link.Type() == "lo" {
			// only omit "lo" here
			continue
		}

		name := link.Attrs().Name

		mac := link.Attrs().HardwareAddr.String()

		// TODO: IPv6
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			glog.Errorf("Get address list of link %s failed: %v", name, err)
			continue
		}

		gateway := ""
		// TODO: IPv6
		routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
		if err != nil {
			glog.Errorf("Get route list of link %s failed: %v", name, err)
			continue
		}
		for _, route := range routes {
			// routes lost, retrieve them in the future
			if route.Gw != nil && route.Dst == nil {
				// only need default gateway right now
				gateway = route.Gw.String()
				break
			}
		}

		for _, addr := range addrs {
			info := &InterfaceInfo{
				Ip:      addr.IPNet.String(),
				Gateway: gateway,
				Mac:     mac,
			}
			infos = append(infos, info)
		}

		// set link down, tap device take over it
		netlink.LinkSetDown(link)
	}

	return infos
}

func GetBridgeNameByIp(ip string) (string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return "", err
	}

	for _, link := range links {
		if link.Type() != "bridge" {
			continue
		}

		// TODO: IPv6
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			bridgeIp := addr.IPNet.String()
			if isSameNetwork(bridgeIp, ip) {
				return link.Attrs().Name, nil
			}
		}
	}

	return "", fmt.Errorf("cannot find bridge which ip %s belong to", ip)
}

// compare IP1 with IP2, return true if they belong to same network
func isSameNetwork(ip1 string, ip2 string) bool {
	_, ipnet1, err := net.ParseCIDR(ip1)
	if err != nil {
		return false
	}

	_, ipnet2, err := net.ParseCIDR(ip2)
	if err != nil {
		return false
	}

	if ipnet1.String() == ipnet2.String() {
		return true
	}

	return false
}
