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

	"github.com/golang/glog"

	"github.com/containernetworking/cni/pkg/ns"
	"golang.org/x/sys/unix"
	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// RunPodSandbox creates and starts a pod-level sandbox.
func (h *Runtime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	userpod, err := buildUserPod(config)
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

	// Persist network namespace in pod label
	if userpod.Labels == nil {
		userpod.Labels = make(map[string]string)
	}
	userpod.Labels["NETNS"] = netNsPath

	// Setup the network
	podId := userpod.Id
	result, err := h.netPlugin.SetUpPod(netNsPath, podId)
	if err != nil {
		glog.Errorf("Setup network for sandbox %q by cni plugin failed: %v", config.String(), err)
	}

	// set down all network interfaces in net ns
	err = setDownLinksInNs(netns)
	if err != nil {
		glog.Errorf("Set down network interfaces in net ns for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	var networkInfo *NetworkInfo
	if result != nil {
		networkInfo = convertCNIResult2NetworkInfo(result)
	}
	if networkInfo != nil {
		// Add network configuration of sandbox net ns to userpod
		addNetworkInterfaceForPod(userpod, networkInfo)
	}

	podID, err := h.client.CreatePod(userpod)
	if err != nil {
		glog.Errorf("Create pod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	// Create sandbox checkpoint
	if err = h.checkpointHandler.CreateCheckpoint(podID, constructPodSandboxCheckpoint(config, netNsPath)); err != nil {
		return podID, err
	}

	err = h.client.StartPod(podID)
	if err != nil {
		glog.Errorf("Start pod %q failed: %v", podID, err)
		if removeError := h.client.RemovePod(podID); removeError != nil {
			glog.Warningf("Remove pod %q failed: %v", removeError)
		}
		// destroy the network namespace
		if tearError := h.netPlugin.TearDownPod(netNsPath, podID); tearError != nil {
			glog.Errorf("Destroy pod %s network namespace failed: %v", podID, tearError)
		}
		unix.Unmount(netNsPath, unix.MNT_DETACH)
		os.Remove(netNsPath)

		// delete sandbox checkpoint
		if removeCheckpointError := h.checkpointHandler.RemoveCheckpoint(podID); removeCheckpointError != nil {
			glog.Errorf("Remove checkpoint of pod %s failed: %v", podID, removeCheckpointError)
		}
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
func buildUserPod(config *kubeapi.PodSandboxConfig) (*types.UserPod, error) {
	var (
		cpuNumber, memoryinMegabytes int32
		err                          error
	)
	var cgroupParent string
	if linuxConfig := config.GetLinux(); linuxConfig != nil {
		cgroupParent = linuxConfig.CgroupParent
	}

	if len(cgroupParent) != 0 && !strings.Contains(cgroupParent, BestEffort) {
		cpuNumber, err = getCpuLimitFromCgroup(cgroupParent)
		if err != nil {
			return nil, err
		}
		memoryinMegabytes, err = getMemeoryLimitFromCgroup(cgroupParent)
		if err != nil {
			return nil, err
		}
		glog.V(5).Infof("Calculated cpu and memory limit: %v, %v based on cgroup parent %s ", cpuNumber, memoryinMegabytes, cgroupParent)
	} else {
		// If pod level QoS is disabled, or this pod is a BE, use default value instead.
		// NOTE: thus actually changes BE to guaranteed. But generally, HyperContainer should not be used for BE workload,
		// it only make sense when we allow multiple runtime in one node.
		cpuNumber = int32(defaultCPUNumber)
		memoryinMegabytes = int32(defaultMemoryinMegabytes)
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

	// Make dns
	if config.DnsConfig != nil {
		// TODO: support DNS search domains in upstream hyperd
		spec.Dns = config.DnsConfig.Servers
	}

	return spec, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (h *Runtime) StopPodSandbox(podSandboxID string) error {
	// Get the pod's net ns info first
	var netNsPath string

	status, statusErr := h.PodSandboxStatus(podSandboxID)
	if statusErr == nil {
		labels := status.GetLabels()
		netNsPath, _ = labels["NETNS"]
	} else {
		checkpoint, err := h.checkpointHandler.GetCheckpoint(podSandboxID)
		if err != nil {
			glog.Errorf("Failed to get checkpoint for sandbox %q: %v", podSandboxID, err)
			return fmt.Errorf("Failed to get sandbox status: %v", statusErr)
		}
		netNsPath = checkpoint.NetNsPath
	}

	code, cause, err := h.client.StopPod(podSandboxID)
	if err != nil {
		glog.Errorf("Stop pod %s failed, code: %d, cause: %s, error: %v", podSandboxID, code, cause, err)
		return err
	}

	// destroy the network namespace
	err = h.netPlugin.TearDownPod(netNsPath, podSandboxID)
	if err != nil {
		glog.Errorf("Destroy pod %s network namespace failed: %v", podSandboxID, err)
	}

	unix.Unmount(netNsPath, unix.MNT_DETACH)
	os.Remove(netNsPath)

	return nil
}

// DeletePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (h *Runtime) DeletePodSandbox(podSandboxID string) error {
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
		podIP = info.Status.PodIP[0]
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

func constructPodSandboxCheckpoint(config *kubeapi.PodSandboxConfig, netnspath string) *PodSandboxCheckpoint {
	checkpoint := NewPodSandboxCheckpoint(config.GetMetadata().Namespace, config.GetMetadata().Name)
	checkpoint.NetNsPath = netnspath
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
