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
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/golang/glog"
	"golang.org/x/sys/unix"

	"k8s.io/api/core/v1"
	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// RunPodSandbox creates and starts a pod-level sandbox.
func (h *Runtime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	userpod, err := h.buildUserPod(config)
	if err != nil {
		glog.Errorf("Build UserPod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	netns, err := ns.NewNS()
	if err != nil {
		glog.Errorf("Create Network Namespace sandbox %q failed: %v", config.String(), err)
		return "", err
	}
	netNsPath := netns.Path()
	defer func() {
		if err != nil {
			unix.Unmount(netNsPath, unix.MNT_DETACH)
			os.Remove(netNsPath)
		}
	}()

	// Persist network namespace in pod label
	if userpod.Labels == nil {
		userpod.Labels = make(map[string]string)
	}
	userpod.Labels["NETNS"] = netNsPath

	// Setup the network
	portMappings := config.GetPortMappings()
	portMappingsParam := make([]cniPortMapping, 0, len(portMappings))
	for _, p := range portMappings {
		if p.HostPort == 0 {
			continue
		}

		protocol := kubeapi.Protocol_name[int32(p.Protocol)]
		portMappingsParam = append(portMappingsParam, cniPortMapping{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      strings.ToLower(protocol),
			HostIP:        p.HostIp,
		})
	}
	capabilities := map[string]interface{}{
		"portMappings": portMappingsParam,
	}
	podId := userpod.Id
	sandboxID := podId
	// workaroud for weave network plugin because it creates a veth pair based on a truncated sandboxID.
	if h.netPlugin.Name() == "weave" {
		sandboxID = getMD5Hash(podId)
	}
	_, err = h.netPlugin.SetUpPod(netNsPath, sandboxID, config.GetMetadata(), config.GetAnnotations(), capabilities)
	if err != nil {
		glog.Errorf("Setup network for sandbox %q by cni plugin failed: %v", config.String(), err)
		return "", err
	}
	defer func() {
		if err != nil {
			// tear down sandbox's network
			teardownError := h.netPlugin.TearDownPod(netNsPath, sandboxID, config.GetMetadata(), config.GetAnnotations(), capabilities)
			if teardownError != nil {
				glog.Errorf("Tear down network for pod %s failed: %v", podId, teardownError)
			}
		}
	}()

	containerInterfaces, err := scanContainerInterfaces(netns)
	if err != nil {
		glog.Errorf("Get CNI result for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	glog.V(3).Infof("Get container interfaces in netns %q: %#v", netNsPath, containerInterfaces)

	hostVeth, err := setupRelayBridgeInNs(netns, containerInterfaces)
	if err != nil {
		glog.Errorf("Set up relay bridge in ns for sandbox %q failed: %v", config.String(), err)
		return "", err
	}
	defer func() {
		if err != nil {
			if tearError := teardownRelayBridgeInNetns(netNsPath, toContainerInterfaceInfos(containerInterfaces)); tearError != nil {
				glog.Errorf("Tear down bridge inside netns %q failed: %v", netNsPath, err)
			}
		}
	}()

	bridgeName, err := setupRelayBridgeInHost(hostVeth)
	if err != nil {
		glog.Errorf("Set up relay bridge in host for sandbox %q failed: %v", config.String(), err)
		return "", err
	}
	userpod.Labels["HOSTBRIDGE"] = bridgeName
	defer func() {
		if err != nil {
			if tearError := teardownRelayBridgeInHost(bridgeName); tearError != nil {
				glog.Errorf("Destroy pod %s host relay bridge failed: %v", podId, err)
			}
		}
	}()

	// Add network configuration of sandbox net ns to userpod
	networkInfo := buildNetworkInfo(bridgeName, containerInterfaces)
	addNetworkInterfaceForPod(userpod, networkInfo)

	podID, err := h.client.CreatePod(userpod)
	if err != nil {
		glog.Errorf("Create pod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}
	defer func() {
		if err != nil {
			if removeError := h.client.RemovePod(podID); removeError != nil {
				glog.Warningf("Remove pod %q failed: %v", removeError)
			}
		}
	}()

	// Create sandbox checkpoint
	err = h.checkpointHandler.CreateCheckpoint(podID, constructPodSandboxCheckpoint(config, netNsPath, bridgeName, containerInterfaces))
	if err != nil {
		return podID, err
	}
	defer func() {
		if err != nil {
			if removeCheckpointError := h.checkpointHandler.RemoveCheckpoint(podID); removeCheckpointError != nil {
				glog.Errorf("Remove checkpoint of pod %s failed: %v", podID, removeCheckpointError)
			}
		}
	}()

	err = h.client.StartPod(podID)
	if err != nil {
		glog.Errorf("Start pod %q failed: %v", podID, err)
		return "", err
	}

	return podID, nil
}

// TODO: only bridge plugin now, support other plugins in the future
func addNetworkInterfaceForPod(userpod *types.UserPod, info *NetworkInfo) {
	ifaces := append([]*types.UserInterface{}, &types.UserInterface{
		Ifname:  info.IfName,
		Bridge:  info.BridgeName,
		Ip:      info.Ip,
		Mac:     info.Mac,
		Gateway: info.Gateway,
	})
	userpod.Interfaces = ifaces
}

// buildUserPod builds hyperd's UserPod based kubelet PodSandboxConfig.
// TODO: support pod-level portmapping (depends on hyperd).
func (h *Runtime) buildUserPod(config *kubeapi.PodSandboxConfig) (*types.UserPod, error) {
	var (
		cpuNumber, memoryinMegabytes int32
		err                          error
	)
	var cgroupParent string
	if linuxConfig := config.GetLinux(); linuxConfig != nil {
		cgroupParent = linuxConfig.CgroupParent
	}

	if len(cgroupParent) != 0 && !strings.Contains(cgroupParent, string(v1.PodQOSBestEffort)) {
		cpuNumber, err = h.getCpuLimitFromCgroup(cgroupParent)
		if err != nil {
			return nil, err
		}
		memoryinMegabytes, err = h.getMemeoryLimitFromCgroup(cgroupParent)
		if err != nil {
			return nil, err
		}
		glog.V(5).Infof("Calculated cpu and memory limit: %v, %v based on cgroup parent %s ", cpuNumber, memoryinMegabytes, cgroupParent)
	} else {
		// If pod level QoS is disabled, or this pod is a BE, use default value instead.
		// NOTE: thus actually changes BE to guaranteed. But generally, HyperContainer should not be used for BE workload,
		// and we now allow multiple runtime in one node.
		cpuNumber = h.defaultCPUNum
		memoryinMegabytes = h.defaultMemoryMB
	}

	spec := &types.UserPod{
		Id:       buildSandboxName(config),
		Hostname: config.Hostname,
		Labels:   buildLabelsWithAnnotations(config.Labels, config.Annotations),
		Resource: &types.UserResource{
			Vcpu:   cpuNumber,
			Memory: memoryinMegabytes,
		},
	}

	// Setup dns options.
	if config.DnsConfig != nil {
		spec.Dns = config.DnsConfig.Servers
		spec.DnsOptions = config.DnsConfig.Options
		spec.DnsSearch = config.DnsConfig.Searches
	}

	return spec, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (h *Runtime) StopPodSandbox(podSandboxID string) error {
	// Get the pod's net ns info first
	var netNsPath string
	var hostBridge string

	// Get sandbox status.
	status, statusErr := h.PodSandboxStatus(podSandboxID)
	if statusErr == nil {
		labels := status.GetLabels()
		netNsPath, _ = labels["NETNS"]
		hostBridge, _ = labels["HOSTBRIDGE"]
	}

	checkpoint, err := h.checkpointHandler.GetCheckpoint(podSandboxID)
	if err != nil {
		glog.Warningf("Failed to get checkpoint for sandbox %q: %v", podSandboxID, err)
	} else {
		netNsPath = checkpoint.NetNsPath
		hostBridge = checkpoint.HostBridge
	}

	// Get portMappings from checkpoint.
	portMappingsParam := make([]cniPortMapping, 0)
	if checkpoint != nil {
		for _, p := range checkpoint.Data.PortMappings {
			if p.HostPort == nil || *p.HostPort == 0 {
				continue
			}

			portMappingsParam = append(portMappingsParam, cniPortMapping{
				HostPort:      *p.HostPort,
				ContainerPort: *p.ContainerPort,
				Protocol:      strings.ToLower(string(*p.Protocol)),
			})
		}
	}
	capabilities := map[string]interface{}{
		"portMappings": portMappingsParam,
	}

	// 1: stop the sandbox.
	code, cause, err := h.client.StopPod(podSandboxID)
	if err != nil && !isPodNotFoundError(err, podSandboxID) {
		return fmt.Errorf("error of stopping sandbox %q, code: %d, cause: %q, error: %v", podSandboxID, code, cause, err)
	}

	// 2: teardown relay bridge inside netns.
	if checkpoint != nil {
		err = teardownRelayBridgeInNetns(netNsPath, checkpoint.Data.Interfaces)
		if err != nil {
			return fmt.Errorf("error of teardown relay bridge inside netns %q: %v", netNsPath, err)
		}
	}

	// 3: tear down the host relay bridge.
	err = teardownRelayBridgeInHost(hostBridge)
	if err != nil {
		return fmt.Errorf("error of teardown relay bridge for sandbox %q: %v", podSandboxID, err)
	}

	// 4: tear down the cni network.
	sandboxID := podSandboxID
	// workaroud for weave network plugin because it creates a veth pair based on a truncated sandboxID.
	if h.netPlugin.Name() == "weave" {
		sandboxID = getMD5Hash(podSandboxID)
	}
	err = h.netPlugin.TearDownPod(netNsPath, sandboxID, status.GetMetadata(), status.GetAnnotations(), capabilities)
	if err != nil {
		return fmt.Errorf("error of teardown network for sandbox %q: %v", podSandboxID, err)
	}

	// 5: umount and remove the netns.
	unix.Unmount(netNsPath, unix.MNT_DETACH)
	os.Remove(netNsPath)

	// 6: remove the checkpoint.
	err = h.checkpointHandler.RemoveCheckpoint(podSandboxID)
	if err != nil {
		return fmt.Errorf("error of removing checkpoint for sandbox %q: %v", podSandboxID, err)
	}

	return nil
}

// RemovePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (h *Runtime) RemovePodSandbox(podSandboxID string) error {
	err := h.client.RemovePod(podSandboxID)
	if err != nil {
		glog.Errorf("Remove pod %s failed: %v", podSandboxID, err)
		return err
	}

	if err := h.checkpointHandler.RemoveCheckpoint(podSandboxID); err != nil {
		glog.Errorf("Remove checkpoint of pod %s failed: %v", podSandboxID, err)
		return err
	}

	return nil
}

// PodSandboxStatus returns the Status of the PodSandbox.
func (h *Runtime) PodSandboxStatus(podSandboxID string) (*kubeapi.PodSandboxStatus, error) {
	info, err := h.client.GetPodInfo(podSandboxID)
	if err != nil {
		glog.Errorf("GetPodInfo for %s failed: %v", podSandboxID, err)
		return nil, err
	}

	state := toPodSandboxState(info.Status.Phase)
	podIP := ""
	if len(info.Status.PodIP) > 0 {
		// Need to do split here since newer hyperd (after 0.8.1) returns 10.244.1.195/24
		podIP = strings.Split(info.Status.PodIP[0], "/")[0]
	}

	podName, podNamespace, podUID, attempt, err := parseSandboxName(info.PodName)
	if err != nil {
		glog.Errorf("ParseSandboxName for %s failed: %v", info.PodName, err)
		return nil, err
	}

	podSandboxMetadata := &kubeapi.PodSandboxMetadata{
		Name:      podName,
		Uid:       podUID,
		Namespace: podNamespace,
		Attempt:   attempt,
	}

	annotations := getAnnotationsFromLabels(info.Spec.Labels)
	kubeletLabels := getKubeletLabels(info.Spec.Labels)
	createdAtNano := info.CreatedAt * secondToNano
	podStatus := &kubeapi.PodSandboxStatus{
		Id:          podSandboxID,
		Metadata:    podSandboxMetadata,
		State:       state,
		Network:     &kubeapi.PodSandboxNetworkStatus{Ip: podIP},
		CreatedAt:   createdAtNano,
		Labels:      kubeletLabels,
		Annotations: annotations,
	}

	return podStatus, nil
}

// ListPodSandbox returns a list of Sandbox.
func (h *Runtime) ListPodSandbox(filter *kubeapi.PodSandboxFilter) ([]*kubeapi.PodSandbox, error) {
	pods, err := h.client.GetPodList()
	if err != nil {
		glog.Errorf("GetPodList failed: %v", err)
		return nil, err
	}

	// using map as set
	sandboxIDs := make(map[string]bool)
	items := make([]*kubeapi.PodSandbox, 0, len(pods))
	for _, pod := range pods {
		state := toPodSandboxState(pod.Status)

		if filter != nil {
			if filter.Id != "" && pod.PodID != filter.Id {
				continue
			}

			if filter.State != nil && state != filter.GetState().State {
				continue
			}

			if filter.LabelSelector != nil && !inMap(filter.LabelSelector, pod.Labels) {
				continue
			}
		}
		converted, err := podResultToKubeAPISandbox(pod)
		if err != nil {
			continue
		}
		sandboxIDs[converted.Id] = true
		items = append(items, converted)
	}

	// Include sandbox that could only be found with its checkpoint if no filter is applied
	// These PodSandbox will only include PodSandboxID, Name, Namespace.
	// These PodSandbox will be in PodSandboxState_SANDBOX_NOTREADY state.
	if filter == nil {
		checkpoints := h.checkpointHandler.ListCheckpoints()
		for _, id := range checkpoints {
			if _, ok := sandboxIDs[id]; !ok {
				checkpoint, err := h.checkpointHandler.GetCheckpoint(id)
				if err != nil {
					glog.Errorf("Failed to get checkpoint for sandbox %q: %v", id, err)
					continue
				}
				items = append(items, checkpointToKubeAPISandbox(id, checkpoint))
			}
		}
	}

	sortByCreatedAt(items)

	return items, nil
}

func constructPodSandboxCheckpoint(config *kubeapi.PodSandboxConfig, netnspath, hostBridge string, interfaces []*containerInterface) *PodSandboxCheckpoint {
	checkpoint := NewPodSandboxCheckpoint(config.GetMetadata().Namespace, config.GetMetadata().Name)
	checkpoint.NetNsPath = netnspath
	checkpoint.HostBridge = hostBridge
	checkpoint.Data.Interfaces = toContainerInterfaceInfos(interfaces)
	for _, pm := range config.GetPortMappings() {
		proto := toCheckpointProtocol(pm.Protocol)
		checkpoint.Data.PortMappings = append(checkpoint.Data.PortMappings, &PortMapping{
			HostPort:      &pm.HostPort,
			ContainerPort: &pm.ContainerPort,
			Protocol:      &proto,
		})
	}

	return checkpoint
}

func toContainerInterfaceInfos(interfaces []*containerInterface) []*ContainerInterfaceInfo {
	result := make([]*ContainerInterfaceInfo, len(interfaces))
	for i := range interfaces {
		result[i] = &ContainerInterfaceInfo{
			Name: interfaces[i].Name,
			Addr: interfaces[i].Addr,
		}
	}

	return result
}

func toCheckpointProtocol(protocol kubeapi.Protocol) Protocol {
	switch protocol {
	case kubeapi.Protocol_TCP:
		return ProtocolTCP
	case kubeapi.Protocol_UDP:
		return ProtocolUDP
	}
	glog.Warningf("Unknown protocol %q: defaulting to TCP", protocol)
	return ProtocolTCP
}

func podResultToKubeAPISandbox(pod *types.PodListResult) (*kubeapi.PodSandbox, error) {
	state := toPodSandboxState(pod.Status)
	podName, podNamespace, podUID, attempt, err := parseSandboxName(pod.PodName)
	if err != nil {
		glog.V(3).Infof("ParseSandboxName for %q failed (%v), assuming it is not managed by frakti", pod.PodName, err)
		return nil, err
	}

	podSandboxMetadata := &kubeapi.PodSandboxMetadata{
		Name:      podName,
		Uid:       podUID,
		Namespace: podNamespace,
		Attempt:   attempt,
	}

	createdAtNano := pod.CreatedAt * secondToNano
	return &kubeapi.PodSandbox{
		Id:        pod.PodID,
		Metadata:  podSandboxMetadata,
		Labels:    pod.Labels,
		State:     state,
		CreatedAt: createdAtNano,
	}, nil

}

func checkpointToKubeAPISandbox(id string, checkpoint *PodSandboxCheckpoint) *kubeapi.PodSandbox {
	state := kubeapi.PodSandboxState_SANDBOX_NOTREADY
	return &kubeapi.PodSandbox{
		Id: id,
		Metadata: &kubeapi.PodSandboxMetadata{
			Name:      checkpoint.Name,
			Namespace: checkpoint.Namespace,
		},
		State: state,
	}
}
