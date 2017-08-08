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

CNI_VERSION=${CNI_VERSION:-"v0.5.2"}
FRAKTI_ROOT=$(readlink -f $(dirname "${BASH_SOURCE}")/..)
source "${FRAKTI_ROOT}/hack/lib/init.sh"

function cleanup() {
  # stop frakti
  frakti::util::kill_process ${FRAKTI_PID} "frakti"

  # stop hyperd
  if [[ -n ${HYPERD_PID-} ]] ; then
    frakti::util::kill_process ${HYPERD_PID} "hyperd"
  fi

  frakti::log::status "Clean up complete"
}

function start_frakti() {
    frakti::log::status "Starting frakti"
    sudo "${FRAKTI_OUTPUT_BINDIR}/frakti" \
        --listen="${FRAKTI_LISTEN_ADDR}" \
        --hyper-endpoint="127.0.0.1:${HYPERD_PORT}" \
        --logtostderr \
        --v=3 1>&2 & \
    FRAKTI_PID=$!
}

function start_hyperd() {
  frakti::hyper::export_related_path
  HYPERD_BINARY_PATH=${HYPERD_BINARY_PATH:-${HYPERD_BIN_DIR:-.}/hyperd}

  # make sure hyperd is running and listen on ${HYPERD_PORT}
  if sudo netstat -an -p|grep hyperd|grep ${HYPERD_PORT} > /dev/null 2>&1; then
    frakti::log::info "hyperd is running."
  else
    if ! [[ -e "${HYPERD_BINARY_PATH}" ]]; then
      frakti::log::status "installing hyperd"
      frakti::hyper::install_hypercontainer
    fi
    
    if sudo pgrep hyperd >/dev/null 2>&1; then
      frakti::log::status "stopping hyperd"
      pgrep hyperd | xargs sudo kill
      sleep 3
    fi
    frakti::log::status "starting hyperd"
    local config=${FRAKTI_TEMP}/hyper_config
    local hyper_api_port=12346
    cat > ${config} << __EOF__
Kernel=${HYPER_KERNEL_PATH}
Initrd=${HYPER_INITRD_PATH}
StorageDriver=${HYPER_STORAGE_DRIVER}
gRPCHost=127.0.0.1:${HYPERD_PORT}
__EOF__
    sudo "${HYPERD_BINARY_PATH}" \
      --host="tcp://127.0.0.1:${hyper_api_port}" \
      --logtostderr \
      --v=3 \
      --config="${config}" &
    HYPERD_PID=$!
    # wait hyperd start
    frakti::util::wait_for_url "http://127.0.0.1:${hyper_api_port}/info" "hyper-info"
  fi
}

function install_remote_hyperd() {
  wget -qO- http://hypercontainer.io/install | bash
}

function configure_cni() {
  # install cni
  sudo mkdir -p /etc/cni/net.d  /opt/cni/bin
  
  git clone https://github.com/containernetworking/plugins $GOPATH/src/github.com/containernetworking/plugins
  cd $GOPATH/src/github.com/containernetworking/plugins

  ./build.sh
  sudo cp bin/* /opt/cni/bin/

  # create network configure file
  sudo sh -c 'cat >/etc/cni/net.d/10-mynet.conflist <<-EOF
{
    "cniVersion": "0.3.1",
    "name": "mynet",
    "plugins": [
        {
            "type": "bridge",
            "bridge": "cni0",
            "isGateway": true,
            "ipMasq": true,
            "ipam": {
                "type": "host-local",
                "subnet": "10.30.0.0/16",
                "routes": [
                    { "dst": "0.0.0.0/0"   }
                ]
            }
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}
EOF'

  sudo sh -c 'cat >/etc/cni/net.d/99-loopback.conf <<-EOF
{
    "cniVersion": "0.3.1",
    "type": "loopback"
}
EOF'
}

function test_cri() {
  # install critest
  go get github.com/kubernetes-incubator/cri-tools/cmd/critest

  # run critest
  sudo env PATH=$PATH:$GOPATH/bin GOPATH=$GOPATH critest -r=/var/run/frakti.sock --focus="Conformance" --skip="port mapping" validation
}

FRAKTI_LISTEN_ADDR=${FRAKTI_LISTEN_ADDR:-/var/run/frakti.sock}
HYPERD_PORT=${HYPERD_PORT:-22318}
HYPERD_HOME=${HYPERD_HOME:-/var/lib/hyper}
HYPERD_BIN_DIR=${HYPERD_BIN_DIR:-/usr/local/bin}
HYPERD_PID=
FRAKTI_PID=
FRAKTI_TEMP=${FRAKTI_TEMP:-/tmp}
HYPERD_TEMP=${HYPERD_TEMP:-/tmp}

frakti::util::trap_add cleanup EXIT SIGINT

HYPER_KERNEL_PATH=${HYPERD_HOME}/kernel
HYPER_INITRD_PATH=${HYPERD_HOME}/hyper-initrd.img
HYPER_STORAGE_DRIVER=${HYPER_STORAGE_DRIVER:-overlay}

runTests() {
  start_hyperd

  configure_cni

  start_frakti

  frakti::test::e2e

  test_cri
}

runTests

frakti::log::status "TEST_FINISHED"
