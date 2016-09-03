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

package framework

import (
	"flag"
	"time"

	"github.com/onsi/ginkgo/config"
)

type TestContextType struct {
	ImageServiceAddr      string
	ImageServiceTimeout   time.Duration
	RuntimeServiceAddr    string
	RuntimeServiceTimeout time.Duration
	RepoRoot              string
	OutputDir             string
	ReportDir             string
	ReportPrefix          string
	Prefix                string
	FraktiPath            string
	// Currently supported values are 'hr' for human-readable and 'json'. It's a comma separated list.
	OutputPrintType string
}

var TestContext TestContextType

// Register flags common to all e2e test suites.
func RegisterCommonFlags() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = true

	flag.StringVar(&TestContext.OutputPrintType, "output-print-type", "hr", "Comma separated list: 'hr' for human readable summaries 'json' for JSON ones.")
	flag.StringVar(&TestContext.ReportPrefix, "report-prefix", "", "Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.")
	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")

	flag.StringVar(&TestContext.Prefix, "prefix", "e2e", "A prefix to be added to cloud resources created during testing.")
}

func RegisterFraktiFlags() {
	flag.StringVar(&TestContext.RepoRoot, "repo-root", "../../", "Root directory of frakti repository, for finding test files.")
	flag.StringVar(&TestContext.FraktiPath, "frakti-path", "frakti", "The frakti binary to use.")
	flag.StringVar(&TestContext.OutputDir, "e2e-output-dir", "/tmp", "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")
	flag.StringVar(&TestContext.ImageServiceAddr, "image-service-addr", "/var/run/frakti.sock", "image service socket for client to connect")
	flag.DurationVar(&TestContext.ImageServiceTimeout, "image-serivce-timeout", 300*time.Second, "Timeout when trying to connect to image service")
	flag.StringVar(&TestContext.RuntimeServiceAddr, "runtime-service-addr", "/var/run/frakti.sock", "runtime service socket for client to connect")
	flag.DurationVar(&TestContext.RuntimeServiceTimeout, "runtime-serivce-timeout", 300*time.Second, "Timeout when trying to connect to a runtime service")
}
