/*
Copyright 2017 The Kubernetes Authors.

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
	"testing"

	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func TestStartContainer(t *testing.T) {
	clientfake := newFakeClientInterface()
	client := &Client{
		client: clientfake,
	}
	r := &Runtime{client: client}
	tests := []struct {
		containerID     string
		containerStatus string
	}{
		{
			"87654321",
			"running",
		},
		{
			"12345678",
			"failed",
		},
	}

	for _, test := range tests {
		containerStatus := types.ContainerStatus{
			Phase: test.containerStatus,
		}
		clientfake.containerInfo = types.ContainerInfo{
			Status: &containerStatus,
		}

		err := r.StartContainer(test.containerID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		clientfake.CleanCalls()
	}
}

func TestStopContainer(t *testing.T) {
	var timeOut int64 = 10
	clientfake := newFakeClientInterface()
	client := &Client{
		client: clientfake,
	}
	r := &Runtime{client: client}
	tests := []struct {
		containerID     string
		containerStatus string
	}{
		{
			"87654321",
			"failed",
		},
		{
			"12345678",
			"failed",
		},
	}

	for _, test := range tests {
		containerStatus := types.ContainerStatus{
			Phase: test.containerStatus,
		}
		clientfake.containerInfo = types.ContainerInfo{
			Status: &containerStatus,
		}

		err := r.StopContainer(test.containerID, timeOut)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		clientfake.CleanCalls()
	}
}

func TestRemoveContainer(t *testing.T) {
	clientfake := newFakeClientInterface()
	client := &Client{
		client: clientfake,
	}
	r := &Runtime{client: client}
	tests := []struct {
		containerID     string
		containerStatus string
	}{
		{
			"87654321",
			"running",
		},
		{
			"12345678",
			"failed",
		},
	}

	for _, test := range tests {
		containerStatus := types.ContainerStatus{
			Phase: test.containerStatus,
		}
		clientfake.containerInfo = types.ContainerInfo{
			Status: &containerStatus,
		}

		err := r.RemoveContainer(test.containerID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		clientfake.CleanCalls()
	}
}

func TestListContainer(t *testing.T) {
	clientfake := newFakeClientInterface()
	client := &Client{
		client: clientfake,
	}
	r := &Runtime{client: client}

	containerStateValue := kubeapi.ContainerStateValue{
		State: kubeapi.ContainerState_CONTAINER_RUNNING,
	}

	tests := []struct {
		image      string
		labels     map[string]string
		containers []*types.ContainerListResult
		expected   []*kubeapi.Container
		fliter     *kubeapi.ContainerFilter
	}{
		{
			"image12345678",
			map[string]string{
				"testLabels":          "testAnnotations",
				fraktiAnnotationLabel: "{\"test\":\"true\"}",
			},
			[]*types.ContainerListResult{
				{
					ContainerID:   "12345678", //fliter
					ContainerName: "k8s_container1_pod1_podnamespace_pod87654321_test",
					PodID:         "pod87654321",
					Status:        "running",
				},
				{
					ContainerID:   "87654321",
					ContainerName: "k8s_container2_pod2_podnamespace_pod12345678_test",
					PodID:         "pod12345678", //fliter
					Status:        "running",
				},
				{
					ContainerID:   "87654321",
					ContainerName: "k8s_container3_pod1_podnamespace_pod87654321_test",
					PodID:         "pod87654321",
					Status:        "pending", //fliter
				},
				{
					ContainerID:   "87654321",
					ContainerName: "k8s_container4_pod1_podnamespace_pod87654321_test",
					PodID:         "pod87654321",
					Status:        "running",
				},
			},
			[]*kubeapi.Container{
				{
					Id:           "87654321",
					PodSandboxId: "pod87654321",
					ImageRef:     "image12345678",
					State:        kubeapi.ContainerState_CONTAINER_RUNNING,
					Labels: map[string]string{
						"testLabels": "testAnnotations",
					},
					Annotations: map[string]string{
						"test": "true",
					},
				},
			},

			&kubeapi.ContainerFilter{
				Id:            "87654321",
				State:         &containerStateValue,
				PodSandboxId:  "pod87654321",
				LabelSelector: nil,
			},
		},
	}

	for _, test := range tests {
		container := types.Container{
			Labels:  test.labels,
			ImageID: test.image,
		}
		clientfake.containerInfo = types.ContainerInfo{
			Container: &container,
		}
		clientfake.containerList = test.containers
		containers, err := r.ListContainers(test.fliter)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(containers) != 1 {
			t.Errorf("Expected: %q, but got %q", "In theory, only the last test case can be passed", "To many test case can be passed")
			break
		}
		if containers[0].Id != test.expected[0].Id {
			t.Errorf("Id expected: %q, but got %q", test.expected[0].Id, containers[0].Id)
		}
		if containers[0].PodSandboxId != test.expected[0].PodSandboxId {
			t.Errorf("PodSandboxId expected: %q, but got %q", test.expected[0].PodSandboxId, containers[0].PodSandboxId)
		}
		if containers[0].ImageRef != test.expected[0].ImageRef {
			t.Errorf("ImageRef expected: %q, but got %q", test.expected[0].ImageRef, containers[0].ImageRef)
		}
		if containers[0].State != test.expected[0].State {
			t.Errorf("State expected: %q, but got %q", test.expected[0].State, containers[0].State)
		}
		_, exist := containers[0].Labels["testLabels"]
		if !exist || len(containers[0].Labels) != 1 {
			t.Errorf("State expected: %q, but got %q", test.expected[0].Labels, containers[0].Labels)
		}
		_, exist = containers[0].Annotations["test"]
		if !exist || len(containers[0].Annotations) != 1 {
			t.Errorf("State expected: %q, but got %q", test.expected[0].Annotations, containers[0].Annotations)
		}

		clientfake.CleanCalls()
	}
}

func TestContainerStatus(t *testing.T) {
	clientfake := newFakeClientInterface()
	client := &Client{
		client: clientfake,
	}
	r := &Runtime{client: client}
	tests := []struct {
		containerID     string
		containerName   string
		containerStatus string
		startedAt       string
		finishedAt      string
		imageID         string
		podID           string
		labels          map[string]string
		valumeMount     []*types.VolumeMount
		expected        kubeapi.ContainerStatus
	}{
		{
			"container87654321",
			"k8s_container1.5_pod1_podnamespace_pod87654321_test",
			"running",
			"2017-08-20T08:01:00+00:00",
			"",
			"image12345678",
			"pod87654321",
			map[string]string{
				"testLabels":             "testAnnotations",
				fraktiAnnotationLabel:    "{\"test\":\"true\"}",
				containerLogPathLabelKey: "/var/log",
			},
			[]*types.VolumeMount{
				{
					Name:      "mount1",
					MountPath: "/var/mount1",
					ReadOnly:  true,
				},
				{
					Name:      "mount2",
					MountPath: "/var/mount2",
					ReadOnly:  false,
				},
			},
			kubeapi.ContainerStatus{
				Id:        "container87654321",
				State:     kubeapi.ContainerState_CONTAINER_RUNNING,
				StartedAt: 1503216060000000000,
				ImageRef:  "image12345678",
				Labels: map[string]string{
					"testLabels": "testAnnotations",
				},
				Annotations: map[string]string{
					"test": "true",
				},
				Mounts: []*kubeapi.Mount{
					{
						ContainerPath:  "mount1",
						HostPath:       "/var/mount1",
						Readonly:       true,
						SelinuxRelabel: true,
					},
					{
						ContainerPath:  "mount2",
						HostPath:       "/var/mount2",
						Readonly:       false,
						SelinuxRelabel: true,
					},
				},
				LogPath: "/var/log",
			},
		},
	}
	for _, test := range tests {
		container := types.Container{
			Labels:       test.labels,
			ImageID:      test.imageID,
			VolumeMounts: test.valumeMount,
			Name:         test.containerName,
			ContainerID:  test.containerID,
		}
		runningStatus := types.RunningStatus{
			StartedAt: test.startedAt,
		}
		containerStatus := types.ContainerStatus{
			ContainerID: test.containerID,
			Phase:       test.containerStatus,
			Waiting:     nil,
			Running:     &runningStatus,
		}
		podVolume := []*types.PodVolume{
			{
				Name:   "mount1",
				Source: "/var/mount1",
			},
			{
				Name:   "mount2",
				Source: "/var/mount2",
			},
		}
		podSpec := types.PodSpec{
			Volumes: podVolume,
		}
		clientfake.containerInfo = types.ContainerInfo{
			Container: &container,
			Status:    &containerStatus,
			PodID:     test.podID,
		}
		clientfake.podInfo = types.PodInfo{
			Spec: &podSpec,
		}
		containerStatusReturn, err := r.ContainerStatus(test.containerID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if containerStatusReturn.Id != test.expected.Id {
			t.Errorf("Id expected: %q, but got %q", test.expected.Id, containerStatusReturn.Id)
		}
		if containerStatusReturn.State != test.expected.State {
			t.Errorf("State expected: %q, but got %q", test.expected.State, containerStatusReturn.State)
		}
		if containerStatusReturn.StartedAt != test.expected.StartedAt {
			t.Errorf("StartedAt expected: %q, but got %q", test.expected.StartedAt, containerStatusReturn.StartedAt)
		}
		if containerStatusReturn.ImageRef != test.expected.ImageRef {
			t.Errorf("ImageRef expected: %q, but got %q", test.expected.ImageRef, containerStatusReturn.ImageRef)
		}
		_, exist := containerStatusReturn.Labels["testLabels"]
		if !exist || len(containerStatusReturn.Labels) != 1 {
			t.Errorf("Labels expected: %q, but got %q", test.expected.Labels, containerStatusReturn.Labels)
		}
		_, exist = containerStatusReturn.Annotations["test"]
		if !exist || len(containerStatusReturn.Annotations) != 1 {
			t.Errorf("Annotations expected: %q, but got %q", test.expected.Annotations, containerStatusReturn.Annotations)
		}
		if len(containerStatusReturn.Mounts) != 2 {
			t.Errorf("Mounts expected: %q, but got %q", test.expected.Mounts, containerStatusReturn.Mounts)

		} else if containerStatusReturn.Mounts[0].HostPath != test.expected.Mounts[0].HostPath {
			t.Errorf("MountsHostPath expected: %q, but got %q", test.expected.Mounts[0].HostPath, containerStatusReturn.Mounts[0].HostPath)
		}

		clientfake.CleanCalls()

	}
}
