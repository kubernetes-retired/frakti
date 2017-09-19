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
source "${FRAKTI_ROOT}/hack/lib/init.sh"
GODEP="${GODEP:-godep}"
PRE_RESTORE_FLAG="--pre-restore"

# Some special pkg dependencies we may need restore them
# before save them to vendor when try to update
PRE_RESTORE_PKG=(
  "k8s.io/client-go"
)

if [ $# -gt 0 ]; then
  if [ "$1" = "$PRE_RESTORE_FLAG" ]; then
    for pkg in ${PRE_RESTORE_PKG[@]}
    do
      cd ${GOPATH}/src/$pkg
      ${GODEP} restore
      echo "- Pre-restore pkg $pkg"
    done
  else
    echo "For now, we only support '--pre-restore' flag"
    exit 1
  fi
fi

cd ${FRAKTI_ROOT}

# Add existing GOPATH files to vendor
pushd "${FRAKTI_ROOT}" > /dev/null
  GO15VENDOREXPERIMENT=1 govendor add +external
popd > /dev/null
