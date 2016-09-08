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

# The root of the build/dist directory
FRAKTI_ROOT=$(readlink -f $(dirname "${BASH_SOURCE}")/../..)

FRAKTI_OUTPUT_BINDIR="${FRAKTI_ROOT}/out"
# Expose frakti directly for readability
PATH="${FRAKTI_OUTPUT_BINDIR}":$PATH
shopt -s expand_aliases
alias sudo='sudo env PATH=$PATH'

source "${FRAKTI_ROOT}/hack/lib/util.sh"
source "${FRAKTI_ROOT}/hack/lib/logging.sh"
source "${FRAKTI_ROOT}/hack/lib/golang.sh"
source "${FRAKTI_ROOT}/hack/lib/hyper.sh"

frakti::log::install_errexit

source "${FRAKTI_ROOT}/hack/lib/test.sh"
