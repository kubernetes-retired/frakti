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
