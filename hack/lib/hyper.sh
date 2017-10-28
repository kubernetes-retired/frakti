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

# a helper to build hyper suit, including hyperd and hyperstart

HYPERD_TEMP=${HYPERD_TEMP:-/tmp}
GO_HYPERHQ_PATH=${GO_HYPERHQ_PATH:-${HYPERD_TEMP}/src/github.com/hyperhq}

frakti::hyper::install_hypercontainer() {
  _saved_gopath=$GOPATH
  export GOPATH=${HYPERD_TEMP}
  mkdir -p "${GO_HYPERHQ_PATH}"

  frakti::log::status "install necessary tools"
  frakti::hyper::preinstall

  frakti::log::status "build hyperd"
  frakti::hyper::build_hyperd

  frakti::log::status "build hyperstart"
  frakti::hyper::build_hyperstart

  frakti::log::status "install hyperd and hyperstart"
  frakti::hyper::export_related_path
  export GOPATH=${_saved_gopath}
}

frakti::hyper::build_hyperstart() {
  local hyperstart_root=${GO_HYPERHQ_PATH}/hyperstart
  frakti::log::info "clone hyperstart repo"
  git clone https://github.com/hyperhq/hyperstart ${hyperstart_root}
  cd ${hyperstart_root}
  frakti::log::info "build hyperstart"
  ./autogen.sh
  ./configure
  make

  HYPER_KERNEL_PATH="${hyperstart_root}/build/arch/x86_64/kernel"
  if [ ! -f ${HYPER_KERNEL_PATH} ]; then
      return 1
  fi
  HYPER_INITRD_PATH="${hyperstart_root}/build/hyper-initrd.img"
  if [ ! -f ${HYPER_INITRD_PATH} ]; then
      return 1
  fi
}

frakti::hyper::build_hyperd() {
  local hyperd_root=${GO_HYPERHQ_PATH}/hyperd

  frakti::log::info "clone hyperd repo"
  git clone https://github.com/hyperhq/hyperd ${hyperd_root}

  cd ${hyperd_root}
  frakti::log::info "build hyperd"
  ./autogen.sh
  ./configure
  make

  HYPERD_BINARY_PATH="${hyperd_root}/cmd/hyperd/hyperd"
  if [ ! -f ${HYPERD_BINARY_PATH} ]; then
      return 1
  fi
}

# install dependencies to build hyperd and hyperstart
# only support ubuntu disto for now
frakti::hyper::preinstall() {
  if ! type "apt-get" > /dev/null 2>&1 ; then
    return 0
  fi
  sudo apt-get update -qq
  sudo apt-get install -y wget autoconf automake pkg-config libdevmapper-dev libsqlite3-dev libvirt-dev libvirt-bin libaio1 libpixman-1-0 -qq
  wget https://s3-us-west-1.amazonaws.com/hypercontainer-download/qemu-hyper/qemu-hyper_2.4.1-1_amd64.deb && sudo dpkg -i --force-all qemu-hyper_2.4.1-1_amd64.deb
}

frakti::hyper::export_related_path() {
  # hyperstart kernel and image path
  HYPER_KERNEL_PATH="${GO_HYPERHQ_PATH}/hyperstart/build/arch/x86_64/kernel"
  HYPER_INITRD_PATH="${GO_HYPERHQ_PATH}/hyperstart/build/hyper-initrd.img"

  # hyperd binary path
  HYPERD_BINARY_PATH="${GO_HYPERHQ_PATH}/hyperd/cmd/hyperd/hyperd"
}

frakti::hyper::cleanup() {
  rm -rf "${GO_HYPERHQ_PATH}/*"
}
