#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
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

FRAKTI_VERSION="v1.1.1"
CLUSTER_CIDR="10.244.0.0/16"
MASTER_CIDR="10.244.1.0/24"

install-hyperd-ubuntu() {
    apt-get update && apt-get install -y qemu libvirt-bin
    curl -sSL https://hypercontainer.io/install | bash
    echo -e "Kernel=/var/lib/hyper/kernel\n\
Initrd=/var/lib/hyper/hyper-initrd.img\n\
Hypervisor=qemu\n\
StorageDriver=overlay\n\
gRPCHost=127.0.0.1:22318" > /etc/hyper/config
    systemctl enable hyperd
    systemctl restart hyperd
}

install-hyperd-centos() {
    curl -sSL https://hypercontainer.io/install | bash
    echo -e "Kernel=/var/lib/hyper/kernel\n\
Initrd=/var/lib/hyper/hyper-initrd.img\n\
Hypervisor=qemu\n\
StorageDriver=overlay\n\
gRPCHost=127.0.0.1:22318" > /etc/hyper/config
    systemctl enable hyperd
    systemctl restart hyperd
}

install-docker-ubuntu() {
    apt-get update
    apt-get install -y docker.io
    systemctl enable docker
    systemctl start docker
}

install-docker-centos() {
    yum install -y docker
    systemctl enable docker
    systemctl start docker
}

install-frakti() {
    if [ -f "/usr/bin/frakti" ]; then
      rm -f /usr/bin/frakti
    fi
    curl -sSL https://github.com/kubernetes/frakti/releases/download/${FRAKTI_VERSION}/frakti -o /usr/bin/frakti
    chmod +x /usr/bin/frakti
    cgroup_driver=$(docker info | awk '/Cgroup Driver/{print $3}')
    cat <<EOF > /lib/systemd/system/frakti.service
[Unit]
Description=Hypervisor-based container runtime for Kubernetes
Documentation=https://github.com/kubernetes/frakti
After=network.target
[Service]
ExecStart=/usr/bin/frakti --v=3 \
          --log-dir=/var/log/frakti \
          --logtostderr=false \
          --cgroup-driver=${cgroup_driver} \
          --listen=/var/run/frakti.sock \
          --streaming-server-addr=%H \
          --hyper-endpoint=127.0.0.1:22318
MountFlags=shared
TasksMax=8192
LimitNOFILE=1048576
LimitNPROC=1048576
LimitCORE=infinity
TimeoutStartSec=0
Restart=on-abnormal
[Install]
WantedBy=multi-user.target
EOF
    systemctl enable frakti
    systemctl start frakti
}

install-kubelet-centos() {
    cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=http://yum.kubernetes.io/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF
    setenforce 0
    sed -i 's/SELINUX=enforcing/SELINUX=disabled/g' /etc/selinux/config
    yum install -y kubernetes-cni kubelet kubeadm kubectl
}

install-kubelet-ubuntu() {
    apt-get update && apt-get install -y apt-transport-https
    curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
    cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
    apt-get update
    apt-get install -y kubernetes-cni kubelet kubeadm kubectl
}

config-kubelet() {
    sed -i '2 i\Environment="KUBELET_EXTRA_ARGS=--container-runtime=remote --container-runtime-endpoint=/var/run/frakti.sock --feature-gates=AllAlpha=true"' /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    systemctl daemon-reload
}

config-cni() {
    mkdir -p /etc/cni/net.d
    cat >/etc/cni/net.d/10-mynet.conf <<-EOF
{
    "cniVersion": "0.3.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "${MASTER_CIDR}",
        "routes": [
            { "dst": "0.0.0.0/0"  }
        ]
    }
}
EOF
    cat >/etc/cni/net.d/99-loopback.conf <<-EOF
{
    "cniVersion": "0.3.0",
    "type": "loopback"
}
EOF
}

setup-master() {
    kubeadm init --pod-network-cidr ${CLUSTER_CIDR} --kubernetes-version stable
    # Also enable schedule pods on the master for allinone.
    export KUBECONFIG=/etc/kubernetes/admin.conf
    kubectl taint nodes --all node-role.kubernetes.io/master-
    # approve kublelet's csr for the node.
    kubectl certificate approve $(kubectl get csr | awk '/^csr/{print $1}')
    # increase memory limits for kube-dns
    kubectl -n kube-system patch deployment kube-dns -p '{"spec":{"template":{"spec":{"containers":[{"name":"kubedns","resources":{"limits":{"memory":"256Mi"}}},{"name":"dnsmasq","resources":{"limits":{"memory":"128Mi"}}},{"name":"sidecar","resources":{"limits":{"memory":"64Mi"}}}]}}}}'
}

command_exists() {
    command -v "$@" > /dev/null 2>&1
}

lsb_dist=''
if command_exists lsb_release; then
    lsb_dist="$(lsb_release -si)"
fi
if [ -z "$lsb_dist" ] && [ -r /etc/lsb-release ]; then
    lsb_dist="$(. /etc/lsb-release && echo "$DISTRIB_ID")"
fi
if [ -z "$lsb_dist" ] && [ -r /etc/centos-release ]; then
    lsb_dist='centos'
fi
if [ -z "$lsb_dist" ] && [ -r /etc/redhat-release ]; then
    lsb_dist='redhat'
fi
if [ -z "$lsb_dist" ] && [ -r /etc/os-release ]; then
    lsb_dist="$(. /etc/os-release && echo "$ID")"
fi

lsb_dist="$(echo "$lsb_dist" | tr '[:upper:]' '[:lower:]')"

case "$lsb_dist" in

    ubuntu)
        install-hyperd-ubuntu
        install-docker-ubuntu
        install-frakti
        install-kubelet-ubuntu
        config-cni
        config-kubelet
        setup-master
    ;;

    fedora|centos|redhat)
        install-hyperd-centos
        install-docker-centos
        install-frakti
        install-kubelet-centos
        config-cni
        config-kubelet
        setup-master
    ;;

    *)
        echo "$lsb_dist is not supported (not in centos|ubuntu)"
    ;;

esac
