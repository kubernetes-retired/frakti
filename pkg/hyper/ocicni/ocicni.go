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
	"sync"
	"time"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/golang/glog"
)

type cniNetworkPlugin struct {
	sync.RWMutex
	defaultNetwork *cniNetwork

	netDir             string
	pluginDirs         []string
	vendorCNIDirPrefix string
}

type cniNetwork struct {
	name          string
	NetworkConfig *libcni.NetworkConfig
	CNIConfig     libcni.CNI
}

func InitCNI(netDir string, pluginDirs ...string) (CNIPlugin, error) {
	plugin := probeNetworkPluginsWithVendorCNIDirPrefix(netDir, pluginDirs, "")
	var err error

	// check if a default network exists, otherwise dump the CNI search and return a noop plugin
	_, err = getDefaultCNINetwork(plugin.netDir, plugin.pluginDirs, plugin.vendorCNIDirPrefix)
	if err != nil {
		glog.Warningf("Error in finding usable CNI plugin - %v", err)
		// create a noop plugin instead
		return &cniNoOp{}, nil
	}

	// sync network config from netDir periodically to detect network config updates
	go func() {
		t := time.NewTimer(10 * time.Second)
		for {
			plugin.syncNetworkConfig()
			select {
			case <-t.C:
			}
		}
	}()
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

	files, err := libcni.ConfFiles(netDir, []string{".conf", ".json"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, fmt.Errorf("No networks found in %s", netDir)
	}

	sort.Strings(files)
	for _, confFile := range files {
		conf, err := libcni.ConfFromFile(confFile)
		if err != nil {
			glog.Warningf("Error loading CNI config file %s: %v", confFile, err)
			continue
		}

		// Search for vendor-specific plugins as well as default plugins in the CNI codebase.
		vendorDir := vendorCNIDir(vendorCNIDirPrefix, conf.Network.Type)
		cninet := &libcni.CNIConfig{
			Path: append(pluginDirs, vendorDir),
		}

		network := &cniNetwork{name: conf.Network.Name, NetworkConfig: conf, CNIConfig: cninet}
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
	return CNIPluginName
}

func (plugin *cniNetworkPlugin) SetUpPod(podNetnsPath string, podID string) (cnitypes.Result, error) {
	if err := plugin.checkInitialized(); err != nil {
		return nil, err
	}

	res, err := plugin.getDefaultNetwork().addToNetwork(podNetnsPath, podID)
	if err != nil {
		glog.Errorf("Error while adding to cni network: %s", err)
		return nil, err
	}
	glog.V(4).Infof("Pod: %s, PodNetnsPath: %s, Adding default network to cni", podID, podNetnsPath)

	return res, nil
}

func (plugin *cniNetworkPlugin) TearDownPod(podNetnsPath string, podID string) error {
	if err := plugin.checkInitialized(); err != nil {
		return err
	}

	return plugin.getDefaultNetwork().deleteFromNetwork(podNetnsPath, podID)
}

func (network *cniNetwork) addToNetwork(podNetnsPath string, podID string) (cnitypes.Result, error) {
	rt, err := buildCNIRuntimeConf(podNetnsPath, podID)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error adding network: %v", podID, podNetnsPath, err)
		return nil, err
	}

	netconf, cninet := network.NetworkConfig, network.CNIConfig
	glog.V(4).Infof("About to run with conf.Network.Type=%v", netconf.Network.Type)
	res, err := cninet.AddNetwork(netconf, rt)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error adding network: %v", podID, podNetnsPath, err)
		return nil, err
	}

	return res, nil
}

func (network *cniNetwork) deleteFromNetwork(podNetnsPath string, podID string) error {
	rt, err := buildCNIRuntimeConf(podNetnsPath, podID)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error deleting network: %v", podID, podNetnsPath, err)
		return err
	}

	netconf, cninet := network.NetworkConfig, network.CNIConfig
	glog.V(4).Infof("About to run with conf.Network.Type=%v", netconf.Network.Type)
	err = cninet.DelNetwork(netconf, rt)
	if err != nil {
		glog.Errorf("Pod: %s, Netns: %s, Error deleting network: %v", podID, podNetnsPath, err)
		return err
	}
	return nil
}

func buildCNIRuntimeConf(podNetnsPath string, podID string) (*libcni.RuntimeConf, error) {
	glog.V(4).Infof("Got netns path %v", podNetnsPath)
	glog.V(4).Infof("Using netns path %v", podNetnsPath)

	rt := &libcni.RuntimeConf{
		ContainerID: podID,
		NetNS:       podNetnsPath,
		IfName:      DefaultInterfaceName,
		Args: [][2]string{
			{"IgnoreUnknown", "1"},
			{"K8S_POD_NAME", podID},
		},
	}

	return rt, nil
}

func (plugin *cniNetworkPlugin) Status() error {
	return plugin.checkInitialized()
}
