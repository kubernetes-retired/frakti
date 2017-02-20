// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package libcni

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

func ConfFromBytes(bytes []byte) (*NetworkConfig, error) {
	conf := &NetworkConfig{Bytes: bytes}
	if err := json.Unmarshal(bytes, &conf.Network); err != nil {
		return nil, fmt.Errorf("error parsing configuration: %s", err)
	}
	return conf, nil
}

func ConfFromFile(filename string) (*NetworkConfig, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", filename, err)
	}
	return ConfFromBytes(bytes)
}

func ConfListFromBytes(bytes []byte) (*NetworkConfigList, error) {
	rawList := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &rawList); err != nil {
		return nil, fmt.Errorf("error parsing configuration list: %s", err)
	}

	rawName, ok := rawList["name"]
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: no name")
	}
	name, ok := rawName.(string)
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: invalid name type %T", rawName)
	}

	var cniVersion string
	rawVersion, ok := rawList["cniVersion"]
	if ok {
		cniVersion, ok = rawVersion.(string)
		if !ok {
			return nil, fmt.Errorf("error parsing configuration list: invalid cniVersion type %T", rawVersion)
		}
	}

	list := &NetworkConfigList{
		Name:       name,
		CNIVersion: cniVersion,
		Bytes:      bytes,
	}

	var plugins []interface{}
	plug, ok := rawList["plugins"]
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: no 'plugins' key")
	}
	plugins, ok = plug.([]interface{})
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: invalid 'plugins' type %T", plug)
	}
	if len(plugins) == 0 {
		return nil, fmt.Errorf("error parsing configuration list: no plugins in list")
	}

	for i, conf := range plugins {
		newBytes, err := json.Marshal(conf)
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal plugin config %d: %v", i, err)
		}
		netConf, err := ConfFromBytes(newBytes)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse plugin config %d: %v", i, err)
		}
		list.Plugins = append(list.Plugins, netConf)
	}

	return list, nil
}

func ConfListFromFile(filename string) (*NetworkConfigList, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", filename, err)
	}
	return ConfListFromBytes(bytes)
}

func ConfFiles(dir string, extensions []string) ([]string, error) {
	// In part, adapted from rkt/networking/podenv.go#listFiles
	files, err := ioutil.ReadDir(dir)
	switch {
	case err == nil: // break
	case os.IsNotExist(err):
		return nil, nil
	default:
		return nil, err
	}

	confFiles := []string{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileExt := filepath.Ext(f.Name())
		for _, ext := range extensions {
			if fileExt == ext {
				confFiles = append(confFiles, filepath.Join(dir, f.Name()))
			}
		}
	}
	return confFiles, nil
}

func LoadConf(dir, name string) (*NetworkConfig, error) {
	files, err := ConfFiles(dir, []string{".conf", ".json"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, fmt.Errorf("no net configurations found")
	}
	sort.Strings(files)

	for _, confFile := range files {
		conf, err := ConfFromFile(confFile)
		if err != nil {
			return nil, err
		}
		if conf.Network.Name == name {
			return conf, nil
		}
	}
	return nil, fmt.Errorf(`no net configuration with name "%s" in %s`, name, dir)
}

func LoadConfList(dir, name string) (*NetworkConfigList, error) {
	files, err := ConfFiles(dir, []string{".conflist"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, fmt.Errorf("no net configuration lists found")
	}
	sort.Strings(files)

	for _, confFile := range files {
		conf, err := ConfListFromFile(confFile)
		if err != nil {
			return nil, err
		}
		if conf.Name == name {
			return conf, nil
		}
	}
	return nil, fmt.Errorf(`no net configuration list with name "%s" in %s`, name, dir)
}

func InjectConf(original *NetworkConfig, key string, newValue interface{}) (*NetworkConfig, error) {
	config := make(map[string]interface{})
	err := json.Unmarshal(original.Bytes, &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal existing network bytes: %s", err)
	}

	if key == "" {
		return nil, fmt.Errorf("key value can not be empty")
	}

	if newValue == nil {
		return nil, fmt.Errorf("newValue must be specified")
	}

	config[key] = newValue

	newBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	return ConfFromBytes(newBytes)
}
