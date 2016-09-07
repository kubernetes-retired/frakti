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

readonly FRAKTI_GO_PACKAGE=k8s.io/frakti

frakti::golang::build_binaries() {
  # Create a sub-shell so that we don't pollute the outer environment
  (
    V=2 frakti::log::info "Go version: $(go version)"

    local -a targets=()

    targets+=($@)
    if [[ ${#targets[@]} -eq 0 ]]; then
        targets=("${FRAKTI_ALL_TARGETS[@]}")
    fi

    local tests=()
    local normal=()

    for target in "${targets[@]}"; do
        if [[ "${target}" =~ ".test"$ ]]; then
            tests+=($target)
        else
            normal+=($target)
        fi
    done

    frakti::log::progress "    "
    for binary in "${normal[@]:+${normal[@]}}"; do
        local outfile=$(frakti::golang::output_filename_for_binary "${binary}")
        go build -o "${outfile}" \
            "${binary}"
        frakti::log::progress "*"
    done

    for test in "${tests[@]:+${tests[@]}}"; do
        local outfile=$(frakti::golang::output_filename_for_binary "${test}")
        local testpkg="$(dirname ${test})"

        go test -c -v \
            -o "${outfile}" \
            "${testpkg}"
        frakti::log::progress "-"
    done
    frakti::log::progress "\n"
  )
}

frakti::golang::output_filename_for_binary() {
  local binary=$1
  local output_path="${FRAKTI_ROOT}/out"
  local bin=$(basename "${binary}")
  echo "${output_path}/${bin}"
}

frakti::golang::unit_test_dirs() {
  local test_dirs=($(frakti::test::find_dirs))
  for dir in "${test_dirs[@]:+${test_dirs[@]}}"; do
      echo "${FRAKTI_GO_PACKAGE}/${dir}"
  done
}
