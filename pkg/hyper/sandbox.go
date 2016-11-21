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

	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	// default resources while the resource limit of kubelet pod is not specified.
	defaultCPUNumber         = 1
	defaultMemoryinMegabytes = 64
)

// RunPodSandbox creates and starts a pod-level sandbox.
func (h *Runtime) RunPodSandbox(config *kubeapi.PodSandboxConfig) (string, error) {
	userpod, err := buildUserPod(config)
	if err != nil {
		glog.Errorf("Build UserPod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	podID, err := h.client.CreatePod(userpod)
	if err != nil {
		glog.Errorf("Create pod for sandbox %q failed: %v", config.String(), err)
		return "", err
	}

	err = h.client.StartPod(podID)
	if err != nil {
		glog.Errorf("Start pod %q failed: %v", podID, err)
		if removeError := h.client.RemovePod(podID); removeError != nil {
			glog.Warningf("Remove pod %q failed: %v", removeError)
		}
		return "", err
	}

	return podID, nil
}

// buildUserPod builds hyperd's UserPod based kubelet PodSandboxConfig.
// TODO: support pod-level portmapping (depends on hyperd).
// TODO: support resource limits via pod-level cgroups, ref https://github.com/kubernetes/kubernetes/issues/27097.
func buildUserPod(config *kubeapi.PodSandboxConfig) (*types.UserPod, error) {
	spec := &types.UserPod{
		Id:       buildSandboxName(config),
		Hostname: config.GetHostname(),
		Labels:   buildLabelsWithAnnotations(config.Labels, config.Annotations),
		Resource: &types.UserResource{
			Vcpu:   int32(defaultCPUNumber),
			Memory: int32(defaultMemoryinMegabytes),
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
	code, cause, err := h.client.StopPod(podSandboxID)
	if err != nil {
		glog.Errorf("Stop pod %s failed, code: %d, cause: %s, error: %v", podSandboxID, code, cause, err)
		return err
	}

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
		Name:      &podName,
		Uid:       &podUID,
		Namespace: &podNamespace,
		Attempt:   &attempt,
	}

	annotations := getAnnotationsFromLabels(info.Spec.Labels)
	kubeletLabels := getKubeletLabels(info.Spec.Labels)
	createdAtNano := info.CreatedAt * secondToNano
	podStatus := &kubeapi.PodSandboxStatus{
		Id:          &podSandboxID,
		Metadata:    podSandboxMetadata,
		State:       &state,
		Network:     &kubeapi.PodSandboxNetworkStatus{Ip: &podIP},
		CreatedAt:   &createdAtNano,
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

	items := make([]*kubeapi.PodSandbox, 0, len(pods))
	for _, pod := range pods {
		state := toPodSandboxState(pod.Status)

		podName, podNamespace, podUID, attempt, err := parseSandboxName(pod.PodName)
		if err != nil {
			glog.V(3).Infof("ParseSandboxName for %q failed (%v), assuming it is not managed by frakti", pod.PodName, err)
			continue
		}

		if filter != nil {
			if filter.Id != nil && pod.PodID != filter.GetId() {
				continue
			}

			if filter.State != nil && state != filter.GetState() {
				continue
			}

			if filter.LabelSelector != nil && !inMap(filter.LabelSelector, pod.Labels) {
				continue
			}
		}

		podSandboxMetadata := &kubeapi.PodSandboxMetadata{
			Name:      &podName,
			Uid:       &podUID,
			Namespace: &podNamespace,
			Attempt:   &attempt,
		}

		createdAtNano := pod.CreatedAt * secondToNano
		items = append(items, &kubeapi.PodSandbox{
			Id:        &pod.PodID,
			Metadata:  podSandboxMetadata,
			Labels:    pod.Labels,
			State:     &state,
			CreatedAt: &createdAtNano,
		})
	}

	sortByCreatedAt(items)

	return items, nil
}
