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

FRAKTI_ROOT=$(readlink -f $(dirname "${BASH_SOURCE}")/..)
source "${FRAKTI_ROOT}/hack/lib/init.sh"

function cleanup()
{
    stop_frakti

    frakti::log::status "Clean up complete"
}

function start_frakti()
{
    frakti::log::status "Starting frakti"
    sudo "${FRAKTI_OUTPUT_BINDIR}/frakti" \
        --listen="${FRAKTI_LISTEN_ADDR}" \
        --hyper-endpoint="127.0.0.1:${HYPERD_PORT}" \
        --v=3 1>&2 & \
    FRAKTI_PID=$!
}

function stop_frakti()
{
    if ps --ppid ${FRAKTI_PID} > /dev/null 2>&1 ; then
        read FRAKTI_PID other < <(ps --ppid ${FRAKTI_PID}|grep frakti)
    fi
    [[ -n "${FRAKTI_PID-}" ]] && sudo kill "${FRAKTI_PID}" 1>&2 2>/dev/null
    t=1
    while ps -p ${FRAKTI_PID} >/dev/null 2>&1 ; do
        echo "wait frakti(${FRAKTI_PID}) stop"
        sleep 1
        [ $((t++)) -ge 15 ] && break
    done
    FRAKTI_PID=
}

FRAKTI_LISTEN_ADDR=${FRAKTI_LISTEN_ADDR:-/var/run/frakti.sock}
HYPERD_PORT=${HYPERD_PORT:-22318}

runTests() {
    # Ensure hyperd is running with correct grpc port 
    frakti::util::ensure_hyperd_running $HYPERD_PORT

    start_frakti

    frakti::test::e2e

    stop_frakti
}

runTests

cleanup
frakti::log::status "TEST_FINISHED"
