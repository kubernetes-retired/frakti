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

package flags

import (
	goflag "flag"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

var (
	// these are flags from vendored cadvisor
	hiddenVendorFlags = []string{
		"boot-id-file",
		"container-hints",
		"docker",
		"docker-env-metadata-whitelist",
		"docker-only",
		"docker-root",
		"event-storage-age-limit",
		"event-storage-event-limit",
		"global-housekeeping-interval",
		"housekeeping-interval",
		"log-cadvisor-usage",
		"machine-id-file",
		"stderrthreshold",
		"storage-driver-buffer-duration",
		"storage-driver-db ",
		"storage-driver-host",
		"storage-driver-password",
		"storage-driver-secure",
		"storage-driver-table",
		"storage-driver-user",
		"vmodule",
	}
)

// WordSepNormalizeFunc changes all flags that contain "_" separators
func WordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		return pflag.NormalizedName(strings.Replace(name, "_", "-", -1))
	}
	return pflag.NormalizedName(name)
}

// WarnWordSepNormalizeFunc changes and warns for flags that contain "_" separators
func WarnWordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		nname := strings.Replace(name, "_", "-", -1)
		glog.Warningf("%s is DEPRECATED and will be removed in a future version. Use %s instead.", name, nname)

		return pflag.NormalizedName(nname)
	}
	return pflag.NormalizedName(name)
}

// InitFlags normalizes and parses the command line flags
func InitFlags() {
	pflag.CommandLine.SetNormalizeFunc(WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	for _, hiddenFlag := range hiddenVendorFlags {
		pflag.CommandLine.MarkHidden(hiddenFlag)
	}

	pflag.Parse()

	path := pflag.Lookup("log-dir").Value.String()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, 0755)
	}
}
