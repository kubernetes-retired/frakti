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

package ocicni

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/golang/glog"
	kubeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// TODO: upgrade CNI Plugin to its stable realease
// when v0.5.0 is released
type cniNetworkPlugin struct {
	sync.RWMutex
	defaultNetwork *cniNetwork

	netDir             string
	pluginDirs         []string
	vendorCNIDirPrefix string
}

type cniNetwork struct {
	name          string
	NetworkConfig *libcni.NetworkConfigList
	CNIConfig     libcni.CNI
}

func InitCNI(netDir string, pluginDirs ...string) (CNIPlugin, error) {
	plugin := probeNetworkPluginsWithVendorCNIDirPrefix(netDir, pluginDirs, "")

	plugin.syncNetworkConfig()
	return plugin, nil
}

func probeNetworkPluginsWithVendorCNIDirPrefix(netDir string, pluginDirs []string, vendorCNIDirPrefix string) *cniNetworkPlugin {
	plugin := &cniNetworkPlugin{
		defaultNetwork:     nil,
		netDir:             netDir,
		pluginDirs:         pluginDirs,
		vendorCNIDirPrefix: vendorCNIDirPrefix,
	}

	// sync NetworkConfig in best effort during probing.
	plugin.syncNetworkConfig()
	return plugin
}

func getDefaultCNINetwork(netDir string, pluginDirs []string, vendorCNIDirPrefix string) (*cniNetwork, error) {
	if netDir == "" {
		netDir = DefaultNetDir
	}
	if len(pluginDirs) == 0 {
		pluginDirs = []string{DefaultCNIDir}
	}

	files, err := libcni.ConfFiles(netDir, []string{".conf", ".conflist", ".json"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, fmt.Errorf("No networks found in %s", netDir)
	}

	sort.Strings(files)
	for _, confFile := range files {
		var confList *libcni.NetworkConfigList
		if strings.HasSuffix(confFile, ".conflist") {
			confList, err = libcni.ConfListFromFile(confFile)
			if err != nil {
				glog.Warningf("Error loading CNI config list file %s: %v", confFile, err)
				continue
			}
		} else {
			conf, err := libcni.ConfFromFile(confFile)
			if err != nil {
				glog.Warningf("Error loading CNI config file %s: %v", confFile, err)
				continue
			}
			confList, err = libcni.ConfListFromConf(conf)
			if err != nil {
				glog.Warningf("Error converting CNI config file %s to list: %v", confFile, err)
				continue
			}
		}
		if len(confList.Plugins) == 0 {
			glog.Warningf("CNI config list %s has no networks, skipping", confFile)
			continue
		}
		confType := confList.Plugins[0].Network.Type

		// Search for vendor-specific plugins as well as default plugins in the CNI codebase.
		vendorDir := vendorCNIDir(vendorCNIDirPrefix, confType)
		cninet := &libcni.CNIConfig{
			Path: append(pluginDirs, vendorDir),
		}
		network := &cniNetwork{name: confList.Name, NetworkConfig: confList, CNIConfig: cninet}
		return network, nil
	}
	return nil, fmt.Errorf("No valid networks found in %s", netDir)
}

func vendorCNIDir(prefix, pluginType string) string {
	return fmt.Sprintf(VendorCNIDirTemplate, prefix, pluginType)
}

func (plugin *cniNetworkPlugin) syncNetworkConfig() {
	network, err := getDefaultCNINetwork(plugin.netDir, plugin.pluginDirs, plugin.vendorCNIDirPrefix)
	if err != nil {
		glog.Errorf("error updating cni config: %s", err)
		return
	}
	plugin.setDefaultNetwork(network)
}

func (plugin *cniNetworkPlugin) getDefaultNetwork() *cniNetwork {
	plugin.RLock()
	defer plugin.RUnlock()
	return plugin.defaultNetwork
}

func (plugin *cniNetworkPlugin) setDefaultNetwork(n *cniNetwork) {
	plugin.Lock()
	defer plugin.Unlock()
	plugin.defaultNetwork = n
}

func (plugin *cniNetworkPlugin) checkInitialized() error {
	if plugin.getDefaultNetwork() == nil {
		return errors.New("cni config uninitialized")
	}
	return nil
}

func (plugin *cniNetworkPlugin) Name() string {
	if err := plugin.checkInitialized(); err != nil {
		return CNIPluginName
	}

	return plugin.getDefaultNetwork().name
}

func (plugin *cniNetworkPlugin) SetUpPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) (cnitypes.Result, error) {
	if err := plugin.checkInitialized(); err != nil {
		return nil, err
	}

	res, err := plugin.getDefaultNetwork().addToNetwork(podNetnsPath, podID, metadata, capabilities)
	if err != nil {
		glog.Errorf("Error while adding to cni network: %s", err)
		return nil, err
	}
	glog.V(4).Infof("Pod: %s, PodNetnsPath: %s, Adding default network to cni", podID, podNetnsPath)

	return res, nil
}

func (plugin *cniNetworkPlugin) TearDownPod(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, annotations map[string]string, capabilities map[string]interface{}) error {
	if err := plugin.checkInitialized(); err != nil {
		return err
	}

	return plugin.getDefaultNetwork().deleteFromNetwork(podNetnsPath, podID, metadata, capabilities)
}

func (network *cniNetwork) addToNetwork(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, capabilities map[string]interface{}) (cnitypes.Result, error) {
	rt, err := buildCNIRuntimeConf(podNetnsPath, podID, metadata, capabilities)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error adding network: %v", podID, podNetnsPath, err)
		return nil, err
	}

	netConf, cniNet := network.NetworkConfig, network.CNIConfig
	glog.V(4).Infof("About to add CNI network %v (type=%v)", netConf.Name, netConf.Plugins[0].Network.Type)
	res, err := cniNet.AddNetworkList(netConf, rt)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error adding network: %v", podID, podNetnsPath, err)
		return nil, err
	}

	return res, nil
}

func (network *cniNetwork) deleteFromNetwork(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, capabilities map[string]interface{}) error {
	rt, err := buildCNIRuntimeConf(podNetnsPath, podID, metadata, capabilities)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error deleting network: %v", podID, podNetnsPath, err)
		return err
	}

	netConf, cniNet := network.NetworkConfig, network.CNIConfig

	glog.V(4).Infof("About to del CNI network %v (type=%v)", netConf.Name, netConf.Plugins[0].Network.Type)
	err = cniNet.DelNetworkList(netConf, rt)
	if err != nil {
		// ignore the error that ns has already not existed
		if strings.Contains(err.Error(), "no such file or directory") {
			return nil
		}
		glog.Errorf("Pod: %s, Netns: %s, Error deleting network: %v", podID, podNetnsPath, err)
		return err
	}
	return nil
}

func buildCNIRuntimeConf(podNetnsPath string, podID string, metadata *kubeapi.PodSandboxMetadata, capabilities map[string]interface{}) (*libcni.RuntimeConf, error) {
	glog.V(4).Infof("Got netns path %v", podNetnsPath)
	glog.V(4).Infof("Using netns path %v", podNetnsPath)

	rt := &libcni.RuntimeConf{
		ContainerID: podID,
		NetNS:       podNetnsPath,
		IfName:      DefaultInterfaceName,
		Args: [][2]string{
			{"IgnoreUnknown", "1"},
			{"K8S_POD_NAME", metadata.GetName()},
			{"K8S_POD_NAMESPACE", metadata.GetNamespace()},
			{"K8S_POD_INFRA_CONTAINER_ID", podID},
		},
		CapabilityArgs: capabilities,
	}

	return rt, nil
}

func (plugin *cniNetworkPlugin) Status() error {
	plugin.syncNetworkConfig()
	return plugin.checkInitialized()
}
