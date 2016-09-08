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

frakti::test::e2e() {
  frakti::log::progress "build e2e test binary\n"
  frakti::golang::build_binaries "k8s.io/frakti/test/e2e/e2e.test"

  # Find the ginkgo binary build.
  local ginkgo=$(frakti::util::find-binary "ginkgo")
  local e2e_test=$(frakti::util::find-binary "e2e.test")

  frakti::log::progress "run frakti e2e test case\n"
  export PATH=$(dirname "${e2e_test}"):"${PATH}"
  sudo "${ginkgo}" "${e2e_test}"
}

frakti::test::find_dirs() {
  (
    cd ${FRAKTI_ROOT}
    find -L . -not \( \
        \( \
          -path './out/*' \
          -o -path './test/e2e/*' \
          -o -path './vendor/*' \
        \) -prune \
      \) -name '*_test.go' -print0 | xargs -0n1 dirname | sed 's|^\./||' | sort -u
  )
}
