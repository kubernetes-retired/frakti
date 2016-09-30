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

	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// buildUserContainer builds hyperd's UserContainer based kubelet ContainerConfig.
func buildUserContainer(config *kubeapi.ContainerConfig, sandboxConfig *kubeapi.PodSandboxConfig) (*types.UserContainer, error) {
	if config.GetLinux().GetSecurityContext().GetPrivileged() {
		return nil, fmt.Errorf("Priviledged containers are not supported in hyper")
	}

	containerSpec := &types.UserContainer{
		Name:       buildContainerName(sandboxConfig, config),
		Image:      config.Image.GetImage(),
		Workdir:    config.GetWorkingDir(),
		Tty:        config.GetTty(),
		Command:    config.GetArgs(),
		Entrypoint: config.GetCommand(),
		Labels:     buildLabelsWithAnnotations(config.Labels, config.Annotations),
	}

	// TODO: support adding device in upstream hyperd when creating container.

	// make volumes
	volumes := make([]*types.UserVolumeReference, len(config.Mounts))
	for idx, v := range config.Mounts {
		volumes[idx] = &types.UserVolumeReference{
			Volume:   v.GetHostPath(),
			Path:     v.GetContainerPath(),
			ReadOnly: v.GetReadonly(),
		}
	}
	containerSpec.Volumes = volumes

	// make environments
	environments := make([]*types.EnvironmentVar, len(config.Envs))
	for idx, env := range config.Envs {
		environments[idx] = &types.EnvironmentVar{
			Env:   env.GetKey(),
			Value: env.GetValue(),
		}
	}
	containerSpec.Envs = environments

	return containerSpec, nil
}
