# Copyright 2016 The Kubernetes Authors.
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

# Use the native vendor/ dependency system
export GO15VENDOREXPERIMENT=1

FRAKIT_VERSION := 1.0
BUILD_DIR ?= ./out
BUILD_TAGS := $(shell hack/libvirt_tag.sh)
LOCALKUBEFILES := go list  -f '{{join .Deps "\n"}}' ./cmd/frakti/ | grep k8s.io | xargs go list -f '{{ range $$file := .GoFiles }} {{$$.Dir}}/{{$$file}}{{"\n"}}{{end}}'

.PHONY: frakti
frakti: $(shell $(LOCALKUBEFILES))
	go build -a --tags "$(BUILD_TAGS)" -o ${BUILD_DIR}/frakti ./cmd/frakti
	go build -a --tags "$(BUILD_TAGS)" -o ${BUILD_DIR}/flexvolume_driver ./cmd/flexvolume_driver

.PHONY: docker
docker:
	cp ${BUILD_DIR}/flexvolume_driver deployment/flexvolume/
	sudo docker build -t stackube/flex-volume:v${or ${IMAGE_VERSION},${FRAKIT_VERSION}} deployment/flexvolume/

.PHONY: install
install:
	cp -f ./out/frakti /usr/bin

clean:
	rm -rf ${BUILD_DIR}

# Build ginkgo
#
# Example:
# make ginkgo
.PHONY: ginkgo
ginkgo:
	hack/make-rules/build.sh ./vendor/github.com/onsi/ginkgo/ginkgo

.PHONY: test-e2e
test-e2e: ginkgo frakti
	hack/test-e2e.sh
