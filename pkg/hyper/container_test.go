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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func makeContainerConfig(sConfig *kubeapi.PodSandboxConfig, name, image string, attempt uint32, labels, annotations map[string]string, mounts []*kubeapi.Mount) *kubeapi.ContainerConfig {
	return &kubeapi.ContainerConfig{
		Metadata: &kubeapi.ContainerMetadata{
			Name:    name,
			Attempt: attempt,
		},
		Image:       &kubeapi.ImageSpec{Image: image},
		Mounts:      mounts,
		Labels:      labels,
		Annotations: annotations,
	}
}

func TestCreateContainer(t *testing.T) {
	r, fakeClient, fakeClock := newTestRuntime()
	podName, namespace := "foo", "bar"
	containerName, image := "sidecar", "logger"
	//'hostPath' has to be existed ,select current file
	containerPath, hostPath := "/var/log/", "container_test.go"
	sandbox := "sandboxid"
	labelKey, labelValue := "abc.xyz", "label"
	annotationKey, annotationValue := "foo.bar", "annotation"
	configs := []*kubeapi.ContainerConfig{}
	sConfigs := []*kubeapi.PodSandboxConfig{}
	//Initialize to create three test cases
	for i := 0; i < 3; i++ {
		s := makeSandboxConfig(fmt.Sprintf("%s%d", podName, i),
			fmt.Sprintf("%s%d", namespace, i), fmt.Sprintf("%d", i), 0)

		labels := map[string]string{labelKey: fmt.Sprintf("%s%d", labelValue, i)}
		annotations := map[string]string{annotationKey: fmt.Sprintf("%s%d", annotationValue, i)}
		mounts := []*kubeapi.Mount{
			{
				ContainerPath: fmt.Sprintf("%s%d%s", containerPath, i, ".go"),
				HostPath:      hostPath,
			},
		}
		c := makeContainerConfig(s, fmt.Sprintf("%s%d", containerName, i),
			fmt.Sprintf("%s:v%d", image, i), uint32(i), labels, annotations, mounts)
		sConfigs = append(sConfigs, s)
		configs = append(configs, c)
	}

	createdAt := dockerTimestampToString(fakeClock.Now())
	for i := range configs {
		sandboxID := fmt.Sprintf("%s%d", sandbox, i)
		containerID, err := r.CreateContainer(sandboxID, configs[i], sConfigs[i])
		assert.NoError(t, err)

		volumeMounts := []*types.VolumeMount{
			{
				//We don't know the name until it's created
				Name:      fakeClient.containerInfoMap[containerID].Container.VolumeMounts[0].Name,
				MountPath: fmt.Sprintf("%s%d%s", containerPath, i, ".go"),
			},
		}
		labels := map[string]string{
			labelKey: fmt.Sprintf("%s%d", labelValue, i),
			"io.kubernetes.container.logpath":  "",
			"io.kubernetes.frakti.annotations": "{\"foo.bar\":\"" + fmt.Sprintf("%s%d", annotationValue, i) + "\"}",
		}
		container := types.Container{
			//We don't know the name until it's created
			Name:         fakeClient.containerInfoMap[containerID].Container.Name,
			ContainerID:  containerID,
			Labels:       labels,
			ImageID:      image + fmt.Sprintf(":v%d", i),
			VolumeMounts: volumeMounts,
		}
		runningStatus := types.RunningStatus{
			StartedAt: createdAt,
		}
		containerStatus := types.ContainerStatus{
			ContainerID: containerID,
			Phase:       "running",
			Waiting:     nil,
			Running:     &runningStatus,
		}
		expected := types.ContainerInfo{
			Container: &container,
			Status:    &containerStatus,
			PodID:     sandboxID,
		}
		assert.Equal(t, expected, *fakeClient.containerInfoMap[containerID])
	}

}

func TestListContainer(t *testing.T) {
	r, fakeClient, _ := newTestRuntime()
	podName, namespace := "foo", "bar"
	containerName, image := "sidecar", "logger"
	//'hostPath' has to be existed ,select current file
	containerPath, hostPath := "/var/log/", "container_test.go"
	sandbox := "sandboxid"
	labelKey, labelValue := "abc.xyz", "label"
	annotationKey, annotationValue := "foo.bar", "annotation"
	configs := []*kubeapi.ContainerConfig{}
	sConfigs := []*kubeapi.PodSandboxConfig{}
	//Initialize to create three test cases
	for i := 0; i < 3; i++ {
		s := makeSandboxConfig(fmt.Sprintf("%s%d", podName, i),
			fmt.Sprintf("%s%d", namespace, i), fmt.Sprintf("%d", i), 0)

		labels := map[string]string{labelKey: fmt.Sprintf("%s%d", labelValue, i)}
		annotations := map[string]string{annotationKey: fmt.Sprintf("%s%d", annotationValue, i)}
		mounts := []*kubeapi.Mount{
			{
				ContainerPath: fmt.Sprintf("%s%d%s", containerPath, i, ".go"),
				HostPath:      hostPath,
			},
		}
		c := makeContainerConfig(s, fmt.Sprintf("%s%d", containerName, i),
			fmt.Sprintf("%s:v%d", image, i), uint32(i), labels, annotations, mounts)
		sConfigs = append(sConfigs, s)
		configs = append(configs, c)
	}
	containerIDs := []string{}
	for i := range configs {
		sandboxID := fmt.Sprintf("%s%d", sandbox, i)
		containerID, err := r.CreateContainer(sandboxID, configs[i], sConfigs[i])
		assert.NoError(t, err)
		containerIDs = append(containerIDs, containerID)
	}
	//Filter the running containers
	containerStateValue := kubeapi.ContainerStateValue{
		State: kubeapi.ContainerState_CONTAINER_RUNNING,
	}
	fliter := kubeapi.ContainerFilter{
		State: &containerStateValue,
	}
	//Test list containers
	containers, err := r.ListContainers(&fliter)
	assert.NoError(t, err)
	assert.Len(t, containers, 3)
	assert.Len(t, fakeClient.containerInfoMap, 3)
	expected := []*kubeapi.Container{}
	for i := 0; i < 3; i++ {
		attempt := containers[i].Metadata.Attempt
		container := kubeapi.Container{
			//We don't know the id until it's created
			Id:           containers[i].Id,
			PodSandboxId: fmt.Sprintf("%s%d", sandbox, attempt),
			ImageRef:     fmt.Sprintf("%s%s%d", image, ":v", attempt),
			Image:        &kubeapi.ImageSpec{Image: ""},
			Metadata: &kubeapi.ContainerMetadata{
				Name:    fmt.Sprintf("%s%d", containerName, attempt),
				Attempt: attempt,
			},
			State: kubeapi.ContainerState_CONTAINER_RUNNING,
			Labels: map[string]string{
				labelKey: fmt.Sprintf("%s%d", labelValue, attempt),
			},
			Annotations: map[string]string{
				annotationKey: fmt.Sprintf("%s%d", annotationValue, attempt),
			},
		}
		expected = append(expected, &container)
	}
	assert.Equal(t, expected, containers)
	//Test stop container
	err = r.StopContainer(containerIDs[0], 0)
	assert.NoError(t, err)
	containers, err = r.ListContainers(&fliter)
	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	assert.Len(t, fakeClient.containerInfoMap, 3)
	//Test remove container
	err = r.RemoveContainer(containerIDs[1])
	assert.NoError(t, err)
	containers, err = r.ListContainers(&fliter)
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Len(t, fakeClient.containerInfoMap, 2)

}

func TestContainerStatus(t *testing.T) {
	r, fakeClient, fakeClock := newTestRuntime()
	podName, namespace := "foo", "bar"
	containerName, image := "sidecar", "logger"
	//'hostPath' has to be existed ,select current file
	containerPath, hostPath := "/var/log/", "container_test.go"
	sandbox := "sandboxid"
	labelKey, labelValue := "abc.xyz", "label"
	annotationKey, annotationValue := "foo.bar", "annotation"
	sConfig := makeSandboxConfig(fmt.Sprintf("%s%d", podName, 0),
		fmt.Sprintf("%s%d", namespace, 0), fmt.Sprintf("%d", 0), 0)

	labels := map[string]string{labelKey: fmt.Sprintf("%s%d", labelValue, 0)}
	annotations := map[string]string{annotationKey: fmt.Sprintf("%s%d", annotationValue, 0)}
	mounts := []*kubeapi.Mount{
		{
			ContainerPath: fmt.Sprintf("%s%d%s", containerPath, 0, ".go"),
			HostPath:      hostPath,
		},
	}
	config := makeContainerConfig(sConfig, fmt.Sprintf("%s%d", containerName, 0),
		fmt.Sprintf("%s:v%d", image, 0), uint32(0), labels, annotations, mounts)

	sandboxID := fmt.Sprintf("%s%d", sandbox, 0)
	containerID, err := r.CreateContainer(sandboxID, config, sConfig)
	assert.NoError(t, err)
	//We don't know the Name until it's created
	volumName := fakeClient.containerInfoMap[containerID].Container.VolumeMounts[0].Name
	pods := []*FakePod{}
	podVolumes := []*types.PodVolume{
		{
			Name:   volumName,
			Source: "/var/mount1",
		},
	}
	fakePod := FakePod{
		PodID:     sandboxID,
		PodVolume: podVolumes,
	}
	pods = append(pods, &fakePod)
	fakeClient.SetFakePod(pods)
	containerStatusReturn, err := r.ContainerStatus(containerID)
	//Convert time to nanoseconds
	timestamp := fakeClock.Now().UTC().UnixNano()
	expected := kubeapi.ContainerStatus{
		Id:        containerID,
		State:     kubeapi.ContainerState_CONTAINER_RUNNING,
		StartedAt: timestamp,
		ImageRef:  fmt.Sprintf("%s%s%d", image, ":v", 0),
		Image:     &kubeapi.ImageSpec{Image: ""},
		Metadata: &kubeapi.ContainerMetadata{
			Name:    fmt.Sprintf("%s%d", containerName, 0),
			Attempt: 0,
		},

		Labels: map[string]string{
			labelKey: fmt.Sprintf("%s%d", labelValue, 0),
		},
		Annotations: map[string]string{
			annotationKey: fmt.Sprintf("%s%d", annotationValue, 0),
		},
		Mounts: []*kubeapi.Mount{
			{
				ContainerPath: fmt.Sprintf("%s%d%s", containerPath, 0, ".go"),
				HostPath:      "/var/mount1",
			},
		},
	}
	assert.NoError(t, err)
	assert.Equal(t, &expected, containerStatusReturn)
}
