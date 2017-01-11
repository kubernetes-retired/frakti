#!/bin/bash
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

set -o errexit
set -o nounset
set -o pipefail

FRAKTI_ROOT=$(dirname "${BASH_SOURCE}")/..
HYPERD_API_ROOT="${FRAKTI_ROOT}/pkg/hyper/types"

if [[ -z "$(which protoc)" || "$(protoc --version)" != "libprotoc 3."* ]]; then
  echo "Generating protobuf requires protoc 3.0 or newer. Please download and"
  echo "install the platform appropriate Protobuf package for your OS: "
  echo
  echo "  try `hack/install-protoc.sh`"
  echo "  or turn to https://github.com/google/protobuf/releases"
  echo
  echo "WARNING: Protobuf changes are not being validated"
  exit 1
fi

function cleanup {
	rm -f ${HYPERD_API_ROOT}/types.pb.go.bak
}

trap cleanup EXIT

hack/build-protoc-gen-gogo.sh
export PATH=${FRAKTI_ROOT}/cmd/protoc-gen-gogo:$PATH
protoc -I${HYPERD_API_ROOT} --gogo_out=plugins=grpc:${HYPERD_API_ROOT} ${HYPERD_API_ROOT}/types.proto
echo "$(cat hack/boilerplate/boilerplate.go.txt ${HYPERD_API_ROOT}/types.pb.go)" > ${HYPERD_API_ROOT}/types.pb.go
sed -i".bak" "s/Copyright YEAR/Copyright $(date '+%Y')/g" ${HYPERD_API_ROOT}/types.pb.go
