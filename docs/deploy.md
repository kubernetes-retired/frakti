# Frakti deploying

Updated: 5/3/2017

- [Frakti deploying](#frakti-deploying)
    - [Overview](#overview)
    - [All in one](#all-in-one)
    - [Kubernetes cluster](#kubernetes-cluster)
        - [Install packages](#install-packages)
            - [Install hyperd](#install-hyperd)
            - [Install docker](#install-docker)
            - [Install frakti](#install-frakti)
            - [Install CNI](#install-cni)
            - [Start frakti](#start-frakti)
            - [Install kubelet](#install-kubelet)
        - [Setting up the master node](#setting-up-the-master-node)
        - [Setting up the worker nodes](#setting-up-the-worker-nodes)


## Overview

This document shows how to easily install a kubernetes cluster with frakti runtime.

Frakti is a hypervisor-based container runtime, which depends on a few packages besides kubernetes:

- hyperd: the hyper container engine (main container runtime)
- docker: the docker container engine (auxiliary container runtime)
- cni: the network plugin

## All in one

An all-in-one kubernetes cluster with frakti runtime could be deployed by running:

```sh
cluster/allinone.sh
```

## Kubernetes cluster

### Install packages

Firstly, hyperd, docker, frakti, CNI and kubelet should be installed on all nodes (including master).

#### Install hyperd

On Ubuntu 16.04+:

```sh
apt-get update && apt-get install -y qemu libvirt-bin
curl -sSL https://hypercontainer.io/install | bash
```

On CentOS 7:

```sh
curl -sSL https://hypercontainer.io/install | bash
```

Configure hyperd:

```sh
echo -e "Kernel=/var/lib/hyper/kernel\n\
Initrd=/var/lib/hyper/hyper-initrd.img\n\
Hypervisor=qemu\n\
StorageDriver=overlay\n\
gRPCHost=127.0.0.1:22318" > /etc/hyper/config
systemctl enable hyperd
systemctl restart hyperd
```

#### Install docker

On Ubuntu 16.04+:

```sh
apt-get update
apt-get install -y docker.io
```

On CentOS 7:

```sh
yum install -y docker
```

Configure and start docker:

```sh
systemctl enable docker
systemctl start docker
```

#### Install frakti

```sh
curl -sSL https://github.com/kubernetes/frakti/releases/download/v1.1.1/frakti -o /usr/bin/frakti
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
```

#### Install CNI

On Ubuntu 16.04+:

```sh
apt-get update && apt-get install -y apt-transport-https
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubernetes-cni
```

On CentOS 7:

```sh
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
yum install -y kubernetes-cni
```

CNI networks should also be configured:

- Skip this section if you want to use existing CNI plugins like Fannel, Weave, Calico etc.
- Otherwise, you can use **bridge** network plugin, it's the simplest way.
    - Subnets should be different on different nodes. e.g. `10.244.1.0/24` for the master and `10.244.2.0/24` for the first node

```sh
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
        "subnet": "10.244.1.0/24",
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
```

#### Start frakti

```sh
systemctl enable frakti
systemctl start frakti
```

#### Install kubelet

On Ubuntu 16.04+:

```sh
apt-get install -y kubelet kubeadm kubectl
```

On CentOS 7:

```sh
yum install -y kubelet kubeadm kubectl
```

Configure kubelet with frakti runtime:

```sh
sed -i '2 i\Environment="KUBELET_EXTRA_ARGS=--container-runtime=remote --container-runtime-endpoint=/var/run/frakti.sock --feature-gates=AllAlpha=true"' /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
systemctl daemon-reload
```

### Setting up the master node

```sh
kubeadm init --pod-network-cidr 10.244.0.0/16 --kubernetes-version stable
```

Configure CNI network plugin (skip this step if you already configured simple bridge plugin)

```sh
kubectl create -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

For other plugins, please check [networking doc](./networking.md).

We prefer to use Linux container runtime to handle kube-dns since the default resource limit of hypervisor runtime is not enough.
So let's annotate the Pod and let Kubernetes do the rolling update for you.

```sh
kubectl -n kube-system patch deployment kube-dns -p '{"spec":{"template":{"metadata":{"annotations":{"runtime.frakti.alpha.kubernetes.io/OSContainer": "true"}}}}}'
```

Optional: enable schedule pods on the master

```sh
export KUBECONFIG=/etc/kubernetes/admin.conf
kubectl taint nodes --all node-role.kubernetes.io/master:NoSchedule-
```

Optional: approve kubelet's certificate signing requests (csr) on the master

Kubernetes v1.7 introduces [csrapproving](https://kubernetes.io/docs/admin/kubelet-tls-bootstrapping/#approval-controller) but the signing controller does not immediately sign all certificate requests. For the alpha version, it should be done manually by a cluster administrator using kubectl, e.g. approving all csr:

```sh
kubectl certificate approve $(kubectl get csr | awk '/^csr/{print $1}')
```

### Setting up the worker nodes

```sh
# get token on master node
token=$(kubeadm token list | grep authentication,signing | awk '{print $1}')

# join master on worker nodes
kubeadm join --token $token ${master_ip:port}
```

### Setting CNI network routes

Containers across multi-node could be connected via direct route. You should set up the routes for all nodes, e.g. suppose one master node and two worker nodes:

```
NODE   IP_ADDRESS   CONTAINER_CIDR
master 10.140.0.1  10.244.1.0/24
node-1 10.140.0.2  10.244.2.0/24
node-2 10.140.0.3  10.244.3.0/24
```

CNI routes could be added by running

```sh
# on master
ip route add 10.244.2.0/24 via 10.140.0.2
ip route add 10.244.3.0/24 via 10.140.0.3

# on node-1
ip route add 10.244.1.0/24 via 10.140.0.1
ip route add 10.244.3.0/24 via 10.140.0.3

# on node-2
ip route add 10.244.1.0/24 via 10.140.0.1
ip route add 10.244.2.0/24 via 10.140.0.2
```

## Use `frakti` in production environment
In a production environment, we recommend user to have their own CNI plugin (Flannel, Calico, Neutron etc), and persistent volume provider (GlusterFS, Cephfs, NFS etc). Please follow Kubernetes admin doc for  details about integration, and it makes no difference if you are using `frakti`.

On the other hand, https://github.com/openstack/stackube is a production ready upstream Kubernetes cluster with `frakti` as container runtime, standalone Neutron, Cinder and Keystone to provide multi-tenancy, networking and storage. Please feel free to explore.

And, if you would like to try `frakti` with more integrations in your own environment, contribution will always be appreciated!
