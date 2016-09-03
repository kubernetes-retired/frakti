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

# check if hyperd is running
# and listening on port $1
# $1 is hyperd grpc port
frakti::util::ensure_hyperd_running() {
    local hyperd_port=$1

    if ! sudo netstat -an -p|grep hyperd|grep $hyperd_port > /dev/null 2>&1 ; then
        frakti::log::error_exit "hyperd not running or grpc server not listening on port ${1}" 1
    fi
}

frakti::util::find-binary() {
  local lookfor="${1}"
  local locations=(
    "${FRAKTI_ROOT}/out/${lookfor}"
  )
  local bin=$( (ls -t "${locations[@]}" 2>/dev/null || true) | head -1 )
  echo -n "${bin}"
}
