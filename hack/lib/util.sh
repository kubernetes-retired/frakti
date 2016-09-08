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

frakti::util::find-binary() {
  local lookfor="${1}"
  local locations=(
    "${FRAKTI_ROOT}/out/${lookfor}"
  )
  local bin=$( (ls -t "${locations[@]}" 2>/dev/null || true) | head -1 )
  echo -n "${bin}"
}

frakti::util::wait_for_url() {
  local url=$1
  local prefix=${2:-}
  local wait=${3:-0.5}
  local times=${4:-25}

  which curl >/dev/null || {
    frakti::log::error_exit "curl must be installed"
  }

  local i
  for i in $(seq 1 $times); do
    local out
    if out=$(curl -fs $url 2>/dev/null); then
      frakti::log::status "On try ${i}, ${prefix}: ${out}"
      return 0
    fi
    sleep ${wait}
  done
  frakti::log::error "Timed out waiting for ${prefix} to answer at ${url}; tried ${times} waiting ${wait} between each"
  return 1
}

frakti::util::trap_add() {
  local trap_add_cmd
  trap_add_cmd=$1
  shift
 
  for trap_add_name in "$@"; do
    local existing_cmd
    local new_cmd

    # Grab the currently define trap commands for this trap
    existing_cmd=`trap -p "${trap_add_name}" | awk -F"'" '{print $2}'`

    if [[ -z "${existing_cmd}" ]]; then
      new_cmd="${trap_add_cmd}"
    else
      new_cmd="${existing_cmd};${trap_add_cmd}"
    fi

    # Assign the test
    trap "${new_cmd}" "${trap_add_name}"
  done
}

frakti::util::kill_process() {
  local pid=${1:-0}
  local process_name=${2:-}
  local sig=${3:-SIGTERM}

  if ps --ppid ${pid} > /dev/null 2>&1 ; then
    read pid other < <(ps --ppid ${pid}|grep ${process_name})
  fi
  [[ -n "${pid-}" ]] && sudo kill "-${sig}" "${pid}" 1>&2 2>/dev/null
  t=1
  while ps -p ${pid} > /dev/null 2>&1 ; do
    echo "wait $process_name($pid) stop"
    sleep 1
    [ $((t++)) -ge 15 ] && break
  done
  pid=
}
