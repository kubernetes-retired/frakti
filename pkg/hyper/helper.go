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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	libcontainercgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/net/context"
	"k8s.io/frakti/pkg/hyper/types"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	// kubePrefix is used to identify the containers/sandboxes on the node managed by kubelet
	kubePrefix = "k8s"
	// kubeSandboxNamePrefix is used to identify a sandbox name
	kubeSandboxNamePrefix = "POD"
	// fraktiAnnotationLabel is used to save annotations into labels
	fraktiAnnotationLabel = "io.kubernetes.frakti.annotations"

	// BestEffort defines the BE qos pod
	// we can not import `pkg/kubelet/qos` because it redefines `log_dir`, conflicts
	BestEffort = "BestEffort"

	// default resources while the pod level qos of kubelet pod is not specified.
	defaultCPUNumber         = 1
	defaultMemoryinMegabytes = 64

	// More details about these: http://kubernetes.io/docs/user-guide/compute-resources/
	// cpuQuotaCgroupFile is the `cfs_quota_us` value set by kubelet pod qos
	cpuQuotaCgroupFile = "cpu.cfs_quota_us"
	// memoryCgroupFile is the `limit_in_bytes` value set by kubelet pod qos
	memoryCgroupFile = "memory.limit_in_bytes"
	// cpuPeriodCgroupFile is the `cfs_period_us` value set by kubelet pod qos
	cpuPeriodCgroupFile = "cpu.cfs_period_us"

	MiB = 1024 * 1024
)

type sandboxByCreated []*kubeapi.PodSandbox

// getContextWithTimeout returns a context with timeout.
func getContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// getContextWithCancel returns a context and cancel func
func getContextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// getHyperAuthConfig converts kubeapi.AuthConfig to hyperd's AuthConfig.
func getHyperAuthConfig(auth *kubeapi.AuthConfig) *types.AuthConfig {
	if auth == nil {
		return &types.AuthConfig{}
	}

	return &types.AuthConfig{
		Username:      auth.GetUsername(),
		Password:      auth.GetPassword(),
		Auth:          auth.GetAuth(),
		Registrytoken: auth.GetRegistryToken(),
		Serveraddress: auth.GetServerAddress(),
	}
}

// Get a repos name and returns the right reposName + tag|digest
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
//     Digest ex: localhost:5000/foo/bar@sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb
func parseRepositoryTag(repos string) (string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, "latest"
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, "latest"
}

// inList checks if a string is in a list
func inList(in string, list []string) bool {
	for _, str := range list {
		if in == str {
			return true
		}
	}

	return false
}

// buildKubeGenericName creates a name which can be reversed to identify container/sandbox name.
// This function returns the unique name.
func buildKubeGenericName(sandboxConfig *kubeapi.PodSandboxConfig, containerName string) string {
	stableName := fmt.Sprintf("%s_%s_%s_%s_%s",
		kubePrefix,
		containerName,
		sandboxConfig.Metadata.GetName(),
		sandboxConfig.Metadata.GetNamespace(),
		sandboxConfig.Metadata.GetUid(),
	)
	UID := fmt.Sprintf("%08x", rand.Uint32())
	return fmt.Sprintf("%s_%s", stableName, UID)
}

// buildSandboxName creates a name which can be reversed to identify sandbox full name.
func buildSandboxName(sandboxConfig *kubeapi.PodSandboxConfig) string {
	sandboxName := fmt.Sprintf("%s.%d", kubeSandboxNamePrefix, sandboxConfig.Metadata.GetAttempt())
	return buildKubeGenericName(sandboxConfig, sandboxName)
}

// parseSandboxName unpacks a sandbox full name, returning the pod name, namespace, uid and attempt.
func parseSandboxName(name string) (string, string, string, uint32, error) {
	podName, podNamespace, podUID, _, attempt, err := parseContainerName(name)
	if err != nil {
		return "", "", "", 0, err
	}

	return podName, podNamespace, podUID, attempt, nil
}

// buildContainerName creates a name which can be reversed to identify container name.
// This function returns stable name, unique name and an unique id.
func buildContainerName(sandboxConfig *kubeapi.PodSandboxConfig, containerConfig *kubeapi.ContainerConfig) string {
	containerName := fmt.Sprintf("%s.%d", containerConfig.Metadata.GetName(), containerConfig.Metadata.GetAttempt())
	return buildKubeGenericName(sandboxConfig, containerName)
}

// parseContainerName unpacks a container name, returning the pod name, namespace, UID,
// container name and attempt.
func parseContainerName(name string) (podName, podNamespace, podUID, containerName string, attempt uint32, err error) {
	parts := strings.Split(name, "_")
	if len(parts) == 0 || parts[0] != kubePrefix {
		err = fmt.Errorf("failed to parse container name %q into parts", name)
		return "", "", "", "", 0, err
	}
	if len(parts) < 6 {
		glog.Warningf("Found a container with the %q prefix, but too few fields (%d): %q", kubePrefix, len(parts), name)
		err = fmt.Errorf("container name %q has fewer parts than expected %v", name, parts)
		return "", "", "", "", 0, err
	}

	nameParts := strings.Split(parts[1], ".")
	containerName = nameParts[0]
	if len(nameParts) > 1 {
		attemptNumber, err := strconv.ParseUint(nameParts[1], 10, 32)
		if err != nil {
			glog.Warningf("invalid container attempt %q in container %q", nameParts[1], name)
		}

		attempt = uint32(attemptNumber)
	}

	return parts[2], parts[3], parts[4], containerName, attempt, nil
}

// buildLabelsWithAnnotations merges annotations into labels.
func buildLabelsWithAnnotations(labels, annotations map[string]string) map[string]string {
	if annotations == nil {
		return labels
	}

	rawAnnotations, err := json.Marshal(annotations)
	if err != nil {
		glog.Warningf("Unable to marshal annotations %q: %v", annotations, err)
	}

	labels[fraktiAnnotationLabel] = string(rawAnnotations)
	return labels
}

// getAnnotationsFromLabels gets annotations from labels.
func getAnnotationsFromLabels(labels map[string]string) map[string]string {
	annotations := make(map[string]string)
	if strValue, found := labels[fraktiAnnotationLabel]; found {
		err := json.Unmarshal([]byte(strValue), &annotations)
		if err != nil {
			glog.Warningf("Unable to get annotations from labels %q", labels)
		}
	}

	return annotations
}

// toPodSandboxState transfers state to kubelet sandbox state.
func toPodSandboxState(state string) kubeapi.PodSandboxState {
	if state == "running" || state == "Running" {
		return kubeapi.PodSandboxState_SANDBOX_READY
	}

	return kubeapi.PodSandboxState_SANDBOX_NOTREADY
}

//getKubeletLabels gets kubelet labels from labels.
func getKubeletLabels(lables map[string]string) map[string]string {
	delete(lables, fraktiAnnotationLabel)
	return lables
}

// inMap checks if a map is in dest map.
func inMap(in, dest map[string]string) bool {
	for k, v := range in {
		if value, ok := dest[k]; ok {
			if value != v {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

// Len is a method for Sort to compute the length of s.
func (s sandboxByCreated) Len() int {
	return len(s)
}

// Less is a method for Sort to compute while one is less between two items of s.
func (s sandboxByCreated) Less(i, j int) bool {
	return *s[i].CreatedAt > *s[j].CreatedAt
}

// Swap is a method for Sort to swap the items in s.
func (s sandboxByCreated) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// sortByCreatedAt sorts podSandboxList by creation time (newest first).
func sortByCreatedAt(podSandboxList []*kubeapi.PodSandbox) {
	sort.Sort(sandboxByCreated(podSandboxList))
}

// parseTimeString parses string to time.UnixNano.
func parseTimeString(str string) (int64, error) {
	t := time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)
	// TODO: fix null timestamp in upstream hyperd.
	if str == "" {
		return t.UnixNano(), nil
	}

	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, str)
	if err != nil {
		return t.UnixNano(), err
	}

	return t.UnixNano(), nil
}

// toKubeContainerState transfers state to kubelet container state.
func toKubeContainerState(state string) kubeapi.ContainerState {
	switch state {
	case "running":
		return kubeapi.ContainerState_CONTAINER_RUNNING
	case "pending":
		return kubeapi.ContainerState_CONTAINER_CREATED
	case "failed", "succeeded":
		return kubeapi.ContainerState_CONTAINER_EXITED
	default:
		return kubeapi.ContainerState_CONTAINER_UNKNOWN
	}
}

// TODO(harry) These two methods will find subsystem mount point frequently, consider move FindCgroupMountpoint into a unified place.
// getCpuLimitFromCgroup get the cpu limit from given cgroupParent
func getCpuLimitFromCgroup(cgroupParent string) (int32, error) {
	mntPath, err := libcontainercgroups.FindCgroupMountpoint("cpu")
	if err != nil {
		return -1, err
	}
	cgroupPath := filepath.Join(mntPath, cgroupParent)
	cpuQuota, err := readCgroupFileToInt64(cgroupPath, cpuQuotaCgroupFile)
	if err != nil {
		return -1, err
	}
	cpuPeriod, err := readCgroupFileToInt64(cgroupPath, cpuPeriodCgroupFile)
	if err != nil {
		return -1, err
	}

	// HyperContainer only support int32 vcpu number, and we need to use `math.Ceil` to make sure vcpu number is always enough.
	vcpuNumber := float64(cpuQuota) / float64(cpuPeriod)
	return int32(math.Ceil(vcpuNumber)), nil
}

// readCgroupFileToInt64 reads contents from given `cgroupPath/cgroupFile` and returns its int64 value
func readCgroupFileToInt64(cgroupPath, cgroupFile string) (int64, error) {
	contents, err := ioutil.ReadFile(filepath.Join(cgroupPath, cgroupFile))
	if err != nil {
		return -1, err
	}
	strValue := strings.TrimSpace(string(contents))
	if value, err := strconv.ParseInt(strValue, 10, 64); err == nil {
		return value, nil
	} else {
		return -1, err
	}
}

// getMemeoryLimitFromCgroup get the memory limit from given cgroupParent
func getMemeoryLimitFromCgroup(cgroupParent string) (int32, error) {
	mntPath, err := libcontainercgroups.FindCgroupMountpoint("memory")
	if err != nil {
		return -1, err
	}
	cgroupPath := filepath.Join(mntPath, cgroupParent)
	memoryInBytes, err := readCgroupFileToInt64(cgroupPath, memoryCgroupFile)
	if err != nil {
		return -1, err
	}

	memoryinMegabytes := int32(memoryInBytes / MiB)
	// HyperContainer requires at least 64Mi memory
	if memoryinMegabytes < defaultMemoryinMegabytes {
		memoryinMegabytes = defaultMemoryinMegabytes
	}
	return memoryinMegabytes, nil
}

// promiseGo is a basic promise implementation
func promiseGo(f func() error) chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- f()
	}()
	return ch
}
