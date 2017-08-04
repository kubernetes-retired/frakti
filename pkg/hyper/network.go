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
	"strings"

	"github.com/golang/glog"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/vishvananda/netlink"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/kubelet/network/hostport"
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

func (h *Runtime) GetPodPortMappings(podID string) ([]*hostport.PortMapping, error) {
	// TODO: get portmappings from docker labels for backward compatibility
	checkpoint, err := h.checkpointHandler.GetCheckpoint(podID)
	if err != nil {
		return nil, err

	}

	portMappings := make([]*hostport.PortMapping, 0, len(checkpoint.Data.PortMappings))
	for _, pm := range checkpoint.Data.PortMappings {
		proto := toAPIProtocol(*pm.Protocol)
		portMappings = append(portMappings, &hostport.PortMapping{
			HostPort:      *pm.HostPort,
			ContainerPort: *pm.ContainerPort,
			Protocol:      proto,
		})

	}
	return portMappings, nil

}

func toAPIProtocol(protocol Protocol) v1.Protocol {
	switch protocol {
	case ProtocolTCP:
		return v1.ProtocolTCP
	case ProtocolUDP:
		return v1.ProtocolUDP

	}
	glog.Warningf("Unknown protocol %q: defaulting to TCP", protocol)
	return v1.ProtocolTCP

}

func findContainerLinkInNs(netns ns.NetNS, cniResult *current.Result) (string, netlink.Link, error) {
	var err error
	var ifName string
	var containerLink netlink.Link

	if err := netns.Do(func(_ ns.NetNS) error {
		for _, iface := range cniResult.Interfaces {
			if iface.Sandbox != "" {
				ifName = iface.Name
				containerLink, err = netlink.LinkByName(ifName)
				if err != nil {
					return fmt.Errorf("Could not find link of container by name %q: %v", ifName, err)
				}
				break
			}
		}

		return nil
	}); err != nil {
		return "", nil, err
	}

	return ifName, containerLink, err
}

func GenRandomHwAddr() (net.HardwareAddr, error) {
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

func delNetConfigInNs(netns ns.NetNS, cniResult *current.Result) error {
	ifName, containerLink, err := findContainerLinkInNs(netns, cniResult)
	if err != nil {
		return err
	}

	if err := netns.Do(func(_ ns.NetNS) error {
		// delete all routes
		for _, r := range cniResult.Routes {
			route := &netlink.Route{
				LinkIndex: containerLink.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       &r.Dst,
				Gw:        r.GW,
			}

			if err := netlink.RouteDel(route); err != nil {
				glog.Errorf("Could not delete route associated with %q: %v", ifName, err)
				return err
			}
		}

		// delete all ip configs
		for _, ipc := range cniResult.IPs {
			intIdx := ipc.Interface

			if cniResult.Interfaces[intIdx].Name != ifName {
				return fmt.Errorf("Failed to del IP addr %v from %q: invalid interface index", ipc, ifName)
			}

			addr := &netlink.Addr{IPNet: &ipc.Address, Label: ""}

			if err := netlink.AddrDel(containerLink, addr); err != nil {
				glog.Errorf("Could not delete IP address of %q: %v", ifName, err)
				return err
			}
		}

		// change hardware address of container link to avoid collision
		hwAddr, err := GenRandomHwAddr()
		if err != nil {
			glog.Errorf("Failed to generate hardware address for container link: %v", err)
			return err
		}

		if err := netlink.LinkSetHardwareAddr(containerLink, hwAddr); err != nil {
			glog.Errorf("Failed to change hardware address of container link: %v", err)
			return nil
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// RandomVethName returns string "veth" with random prefix (hashed from entropy)
func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func makeVethPair(name, peer string) (netlink.Link, error) {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Flags: net.FlagUp,
		},
		PeerName: peer,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	return veth, nil
}

func peerExists(name string) bool {
	if _, err := netlink.LinkByName(name); err != nil {
		return false
	}
	return true
}

func makeVeth(name string) (peerName string, veth netlink.Link, err error) {
	for i := 0; i < 10; i++ {
		peerName, err = RandomVethName()
		if err != nil {
			return
		}

		veth, err = makeVethPair(name, peerName)
		switch {
		case err == nil:
			return

		case os.IsExist(err):
			if peerExists(peerName) {
				continue
			}
			err = fmt.Errorf("container veth name provided (%v) already exists", name)
			return

		default:
			err = fmt.Errorf("failed to make veth pair: %v", err)
			return
		}
	}

	// should really never be hit
	err = fmt.Errorf("failed to find a unique veth name")
	return
}

func setupVeth(contVethName string, hostNS ns.NetNS) (netlink.Link, netlink.Link, error) {
	hostVethName, contVeth, err := makeVeth(contVethName)
	if err != nil {
		return nil, nil, err
	}

	if err = netlink.LinkSetUp(contVeth); err != nil {
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

func RandomBridgeName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bridge name: %v", err)
	}

	return fmt.Sprintf("br%x", entropy), nil
}

func bridgeByName(name string) (*netlink.Bridge, error) {
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

	br, err = bridgeByName(brName)
	if err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return nil, err
	}

	return br, nil
}

func setupRelayBridgeInNs(netns ns.NetNS, cniResult *current.Result) (netlink.Link, error) {
	var hostVeth netlink.Link

	_, containerLink, err := findContainerLinkInNs(netns, cniResult)
	if err != nil {
		return "", err
	}

	if err := netns.Do(func(hostNS ns.NetNS) error {
		var err error
		var containerVeth netlink.Link

		// setup bridge in ns
		brName, err := RandomBridgeName()
		if err != nil {
			glog.Errorf("Failed to generate bridge name in ns: %v", err)
			return err
		}

		br, err := setupBridge(brName)
		if err != nil {
			glog.Errorf("Failed to setup bridge in ns: %v", err)
			return err
		}

		// create the veth pair in the container and move host end to host netns
		vethName, err := RandomVethName()
		if err != nil {
			glog.Errorf("Failed to generate veth name in ns: %v", err)
			return err
		}

		hostVeth, containerVeth, err = setupVeth(vethName, hostNS)
		if err != nil {
			glog.Errorf("Failed to create veth pair in ns: %v", err)
			return err
		}

		// connect both new created veth and the old one to the bridge in ns
		if err := netlink.LinkSetMaster(containerVeth, br); err != nil {
			glog.Errorf("Failed to connect new created veth to the bridge in ns: %v", err)
			return err
		}

		if err := netlink.LinkSetMaster(containerLink, br); err != nil {
			glog.Errorf("Failed to connect link of container to the bridge in ns: %v", err)
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return hostVeth, nil
}

func setupRelayBridgeInHost(hostVeth netlink.Link) (string, error) {
	// setup bridge in host
	brName, err := RandomBridgeName()
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

	return brName, nil
}

func buildNetworkInfo(bridgeName string, cniResult *current.Result) *NetworkInfo {
	ret := &NetworkInfo{}

	ret.BridgeName = bridgeName
	ret.IfName = strings.Replace(bridgeName, "br", "tap", 1)

	for _, iface := range cniResult.Interfaces {
		if iface.Sandbox != "" {
			// interface information in net ns
			ret.Mac = iface.Mac
			break
		}
	}

	ret.Ip = cniResult.IPs[0].Address.String()
	ret.Gateway = cniResult.IPs[0].Gateway.String()

	return ret
}
