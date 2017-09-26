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
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/golang/glog"
	"github.com/vishvananda/netlink"
)

const (
	defaultContainerBridgeName = "br-netns"
)

// cniPortMapping maps to the standard CNI portmapping Capability
// see: https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md
type cniPortMapping struct {
	HostPort      int32  `json:"hostPort"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP"`
}

// The network information needed to create HyperContainer
// network device from CNI Result
type NetworkInfo struct {
	IfName     string
	Mac        string
	Ip         string
	Gateway    string
	BridgeName string
}

type containerInterface struct {
	Name    string
	Mac     net.HardwareAddr
	Addr    *net.IPNet
	Gateway string
	Link    *netlink.Link
}

func generateMacAddress() (net.HardwareAddr, error) {
	const alphanum = "0123456789abcdef"
	var bytes = make([]byte, 8)
	_, err := rand.Read(bytes)

	if err != nil {
		glog.Errorf("get random number faild")
		return nil, err
	}

	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}

	tmp := []string{"52:54", string(bytes[0:2]), string(bytes[2:4]), string(bytes[4:6]), string(bytes[6:8])}
	return net.ParseMAC(strings.Join(tmp, ":"))
}

func scanContainerInterfaces(netns ns.NetNS) ([]*containerInterface, error) {
	results := make([]*containerInterface, 0)

	if err := netns.Do(func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		for _, link := range links {
			linkName := link.Attrs().Name
			if linkName == "lo" || link.Type() == "ipip" {
				continue
			}

			// Only ipv4 is supported now.
			var ip *net.IPNet
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				glog.Errorf("Get address list of link %s failed: %v", linkName, err)
				continue
			}
			if addrs != nil {
				ip = addrs[0].IPNet
			}

			gateway := ""
			routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
			if err != nil {
				glog.Errorf("Get route list of link %s failed: %v", linkName, err)
				continue
			}
			for _, route := range routes {
				if route.Gw != nil && route.Dst == nil {
					gateway = route.Gw.String()
					break
				}
			}

			// Fix gateway for /32 IP addresses.
			maskSize, _ := ip.Mask.Size()
			if maskSize == 32 {
				gateway = ip.IP.String()
			}

			results = append(results, &containerInterface{
				Name:    linkName,
				Mac:     link.Attrs().HardwareAddr,
				Addr:    ip,
				Gateway: gateway,
				Link:    &link,
			})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return results, nil
}

// generateVethPair returns a veth pair.
func generateVethPair() (string, string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	return fmt.Sprintf("veth%x", entropy), fmt.Sprintf("ceth%x", entropy), nil
}

func setupVeth(contVethName, hostVethName string, hostNS ns.NetNS) (netlink.Link, netlink.Link, error) {
	contVeth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  contVethName,
			Flags: net.FlagUp,
		},
		PeerName: hostVethName,
	}
	if err := netlink.LinkAdd(contVeth); err != nil {
		return nil, nil, err
	}

	if err := netlink.LinkSetUp(contVeth); err != nil {
		return nil, nil, fmt.Errorf("failed to set %q up: %v", contVethName, err)
	}

	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup %q: %v", hostVethName, err)
	}

	if err = netlink.LinkSetNsFd(hostVeth, int(hostNS.Fd())); err != nil {
		return nil, nil, fmt.Errorf("failed to move veth to host netns: %v", err)
	}

	err = hostNS.Do(func(_ ns.NetNS) error {
		hostVeth, err = netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q in %q: %v", hostVethName, hostNS.Path(), err)
		}

		if err = netlink.LinkSetUp(hostVeth); err != nil {
			return fmt.Errorf("failed to set %q up: %v", hostVethName, err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return hostVeth, contVeth, nil
}

func generateBridgeName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bridge name: %v", err)
	}

	return fmt.Sprintf("br%x", entropy), nil
}

func getBridgeByName(name string) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("Could not look up %q: %v", name, err)
	}

	br, ok := l.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a bridge", name)
	}

	return br, nil
}

func setupBridge(brName string) (*netlink.Bridge, error) {
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: brName,
			// Let kernel use default txqueuelen; leaving it unset
			// means 0, and a zero-length TX queue messes up FIFO
			// traffic shapers which use TX queue length as the
			// default packet limit
			TxQLen: -1,
		},
	}

	err := netlink.LinkAdd(br)
	if err != nil {
		return nil, fmt.Errorf("could not add %q: %v", brName, err)
	}

	br, err = getBridgeByName(brName)
	if err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return nil, err
	}

	return br, nil
}

func setupRelayBridgeInNs(netns ns.NetNS, containerInterfaces []*containerInterface) (netlink.Link, error) {
	var hostVeth netlink.Link

	if err := netns.Do(func(hostNS ns.NetNS) error {
		var err error
		var containerVeth netlink.Link

		br, err := setupBridge(defaultContainerBridgeName)
		if err != nil {
			glog.Errorf("Failed to setup bridge in ns: %v", err)
			return err
		}

		// create the veth pair in the container and move host end to host netns
		vethName, pairName, err := generateVethPair()
		if err != nil {
			glog.Errorf("Failed to generate veth name in ns: %v", err)
			return err
		}

		hostVeth, containerVeth, err = setupVeth(vethName, pairName, hostNS)
		if err != nil {
			glog.Errorf("Failed to create veth pair in ns: %v", err)
			return err
		}

		// connect both new created veth and the old one to the bridge in ns
		if err := netlink.LinkSetMaster(containerVeth, br); err != nil {
			glog.Errorf("Failed to connect new created veth to the bridge in ns: %v", err)
			return err
		}

		for _, iface := range containerInterfaces {
			// remove addr on the iface, which will be set inside hypercontainer.
			if err := netlink.AddrDel(*iface.Link, &netlink.Addr{
				IPNet: iface.Addr,
			}); err != nil {
				return fmt.Errorf("error of removing addr on iface: %v", err)
			}

			// set a new mac address, whose old one will be used in hypercontainer.
			mac, err := generateMacAddress()
			if err != nil {
				return fmt.Errorf("error of generating mac address: %v", err)
			}
			if err := netlink.LinkSetHardwareAddr(*iface.Link, mac); err != nil {
				return fmt.Errorf("error of setting mac on iface: %v", err)
			}

			if err := netlink.LinkSetMaster(*iface.Link, br); err != nil {
				return fmt.Errorf("error of setting iface master: %v", err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return hostVeth, nil
}

func teardownRelayBridgeInNetns(netnsPath string, interfaces []*ContainerInterfaceInfo) error {
	netns, err := ns.GetNS(netnsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("error of getting netns %q: %v", netnsPath, err)
	}

	if err = netns.Do(func(hostNS ns.NetNS) error {
		br, err := getBridgeByName(defaultContainerBridgeName)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("error of getting bridge: %v", err)
		}

		// remove the bridge.
		if br != nil {
			if err := netlink.LinkSetDown(br); err != nil {
				return err
			}

			if err := netlink.LinkDel(br); err != nil {
				return err
			}
		}

		// set back address for interfaces.
		// This is required for some network plugins, e.g. bridge.
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}
		for _, link := range links {
			linkName := link.Attrs().Name

			for _, it := range interfaces {
				if it.Name != linkName {
					continue
				}

				addr, err := netlink.ParseAddr(it.Addr.String())
				if err != nil {
					glog.Warningf("Parsing addr %q failed: %v", it.Addr.String(), err)
					continue
				}
				if err := netlink.AddrAdd(link, addr); err != nil {
					glog.Warningf("Adding addr %q failed: %v", it.Addr.String(), err)
				}
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func setupRelayBridgeInHost(hostVeth netlink.Link) (string, error) {
	// setup bridge in host
	brName, err := generateBridgeName()
	if err != nil {
		glog.Errorf("Failed to generate bridge name in host: %v", err)
		return "", err
	}

	br, err := setupBridge(brName)
	if err != nil {
		glog.Errorf("Failed to setup bridge in host: %v", err)
		return "", err
	}

	// connect veth to bridge in host
	if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
		glog.Errorf("Failed to connect veth to bridge in host: %v", err)
		return "", err
	}

	// bypass netfilter for the bridge.
	// This is because bridge-nf-call-iptables=1 is required for kubernetes.
	if err := disableBridgeTracking(brName, true); err != nil {
		return "", fmt.Errorf("disableBridgeTracking failed: %v", err)
	}

	return brName, nil
}

func teardownRelayBridgeInHost(bridgeName string) error {
	br, err := getBridgeByName(bridgeName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}

		return err
	}

	if err := netlink.LinkSetDown(br); err != nil {
		return err
	}

	if err := netlink.LinkDel(br); err != nil {
		return err
	}

	if err := disableBridgeTracking(bridgeName, false); err != nil {
		return err
	}

	return nil
}

func buildNetworkInfo(bridgeName string, interfaces []*containerInterface) *NetworkInfo {
	return &NetworkInfo{
		BridgeName: bridgeName,
		IfName:     strings.Replace(bridgeName, "br", "tap", 1),
		Mac:        interfaces[0].Mac.String(),
		Ip:         interfaces[0].Addr.String(),
		Gateway:    interfaces[0].Gateway,
	}
}

func disableBridgeTracking(brName string, disable bool) error {
	iptablesPath, err := exec.LookPath("iptables")
	if err != nil {
		return err
	}

	if disable {
		_, err = exec.Command(iptablesPath, "-t", "raw", "-I", "PREROUTING", "-i", brName, "-j", "NOTRACK").CombinedOutput()
	} else {
		_, err = exec.Command(iptablesPath, "-t", "raw", "-D", "PREROUTING", "-i", brName, "-j", "NOTRACK").CombinedOutput()
	}

	if err != nil {
		return err
	}

	return nil
}
