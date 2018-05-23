# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO := go
PROJECT := k8s.io/frakti
BUILD_TAGS := seccomp apparmor
VERSION := 0.1

.PHONY: all
all: verify

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'verify'           	- Execute the source code verification tools"
	@echo " * 'version'          	- Print current containerd-kata plugin version"
	@echo " * 'test'             	- Test containerd-kata with unit test"

.PHONY: verify
verify: lint gofmt boiler

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: lint
lint:
	@echo "checking lint"
	@./hack/verify-lint.sh

.PHONY: gofmt
gofmt:
	@echo "checking gofmt"
	@./hack/verify-gofmt.sh

.PHONY: boiler
boiler:
	@echo "checking boilerplate"
	@./hack/verify-boilerplate.sh

.PHONY: test
test:
	@./hack/test-add.sh
	$(GO) test -timeout=10m -race ./pkg/... -tags '$(BUILD_TAGS)' 
