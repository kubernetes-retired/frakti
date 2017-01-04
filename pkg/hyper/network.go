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
	"sort"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/ns"
	"github.com/vishvananda/netlink"
)

const (
	DefaultInterfaceName = "eth0"
	DefaultNetDir        = "/etc/cni/net.d"
	DefaultCNIDir        = "/opt/cni/bin"
)

type InterfaceInfo struct {
	Ip string
}

type CNINetworkPlugin struct {
	sandboxNs map[string]*sandboxNetNS
}

func InitCNI() (*CNINetworkPlugin, error) {
	plugin := &CNINetworkPlugin{}

	plugin.sandboxNs = make(map[string]*sandboxNetNS)

	return plugin, nil
}

func (plugin *CNINetworkPlugin) SetupPodNetwork(podname string, netdir string) error {
	sandboxns := &sandboxNetNS{}

	netns, err := ns.NewNS()
	if err != nil {
		return err
	}
	sandboxns.netns = netns

	if netdir == "" {
		netdir = DefaultNetDir
	}
	sandboxns.netdir = netdir

	plugin.sandboxNs[podname] = sandboxns

	return plugin.sandboxNs[podname].addNetwork()
}

func (plugin *CNINetworkPlugin) GetPodNetworkStatus(podname string) ([]*InterfaceInfo, error) {
	return plugin.sandboxNs[podname].getNetworkStatus()
}

func (plugin *CNINetworkPlugin) TeardownPodNetwork(podname string) error {
	err := plugin.sandboxNs[podname].delNetwork()
	if err != nil {
		return err
	}

	err = plugin.sandboxNs[podname].netns.Close()
	if err != nil {
		return err
	}

	delete(plugin.sandboxNs, podname)

	return nil
}

type sandboxNetNS struct {
	netns  ns.NetNS
	netdir string
}

func (sns *sandboxNetNS) addNetwork() error {
	files, err := libcni.ConfFiles(sns.netdir)
	switch {
	case err != nil:
		return err
	case len(files) == 0:
		return fmt.Errorf("No networks found in %s", sns.netdir)
	}

	sort.Strings(files)

	cninet := &libcni.CNIConfig{
		Path: strings.Split(DefaultCNIDir, ":"),
	}

	for _, confFile := range files {
		netconf, err := libcni.ConfFromFile(confFile)
		if err != nil {
			continue
		}

		rt := &libcni.RuntimeConf{
			ContainerID: "cni",
			NetNS:       sns.netns.Path(),
			IfName:      DefaultInterfaceName,
		}

		_, err = cninet.AddNetwork(netconf, rt)
		if err != nil {
			continue
		}
	}

	return nil
}

func (sns *sandboxNetNS) getNetworkStatus() ([]*InterfaceInfo, error) {
	var infos []*InterfaceInfo
	err := sns.netns.Do(func(_ ns.NetNS) error {
		infos = collectionInterfaceInfo()
		return nil
	})

	return infos, err
}

func (sns *sandboxNetNS) delNetwork() error {
	files, err := libcni.ConfFiles(sns.netdir)
	switch {
	case err != nil:
		return err
	case len(files) == 0:
		return fmt.Errorf("No networks found in %s", sns.netdir)
	}

	sort.Strings(files)

	cninet := &libcni.CNIConfig{
		Path: strings.Split(DefaultCNIDir, ":"),
	}

	for _, confFile := range files {
		netconf, err := libcni.ConfFromFile(confFile)
		if err != nil {
			continue
		}

		rt := &libcni.RuntimeConf{
			ContainerID: "cni",
			NetNS:       sns.netns.Path(),
			IfName:      DefaultInterfaceName,
		}

		err = cninet.DelNetwork(netconf, rt)
		if err != nil {
			continue
		}
	}

	return nil
}

func collectionInterfaceInfo() []*InterfaceInfo {
	infos := []*InterfaceInfo{}

	links, err := netlink.LinkList()
	if err != nil {
		return infos
	}

	for _, link := range links {
		if link.Type() != "veth" {
			// lo is here too
			continue
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return infos
		}

		for _, addr := range addrs {
			info := &InterfaceInfo{
				Ip: addr.IPNet.String(),
			}
			infos = append(infos, info)
		}

		// set link down, tap device take over it
		netlink.LinkSetDown(link)
	}

	return infos
}
func GetBridgeFromIpCompare(ip string) (string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return "", err
	}

	for _, link := range links {
		if link.Type() != "bridge" {
			continue
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if IpCompare(addr.IPNet.String(), ip) {
				fmt.Printf("find bridge %s\n", link.Attrs().Name)
				return link.Attrs().Name, nil
			}
		}
	}

	return "", fmt.Errorf("cannot find bridge which ip %s belong to", ip)
}

// compare ip1 with ip2, return true if they belong to the same network
func IpCompare(ip1 string, ip2 string) bool {
	_, ipnet1, _ := net.ParseCIDR(ip1)
	_, ipnet2, _ := net.ParseCIDR(ip2)

	if ipnet1.String() == ipnet2.String() {
		return true
	}

	return false
}
