# Frakti

[![Build Status](https://travis-ci.org/kubernetes/frakti.svg?branch=master)](https://travis-ci.org/kubernetes/frakti) [![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes/frakti)](https://goreportcard.com/report/github.com/kubernetes/frakti)

## The hypervisor-based container runtime for Kubernetes

Frakti lets Kubernetes run pods and containers directly inside hypervisors via [HyperContainer](http://hypercontainer.io/). It is light weighted and portable, but can provide much stronger isolation with independent kernel than linux-namespace-based container runtimes.

<p align="center">
  <img src="docs/images/frakti.png" width="600">
</p>

Frakti serves as a kubelet container runtime API server. Its endpoint should be configured while starting kubelet.

## QuickStart

Build frakti:

```sh
mkdir -p $GOPATH/src/k8s.io
git clone https://github.com/kubernetes/frakti.git $GOPATH/src/k8s.io/frakti
cd $GOPATH/src/k8s.io/frakti
make && make install
```

Start hyperd with gRPC endpoint `127.0.0.1:22318`:

```sh
cat >/etc/hyper/config <<EOF
# Boot kernel
Kernel=/var/lib/hyper/kernel
# Boot initrd
Initrd=/var/lib/hyper/hyper-initrd.img
# Storage driver for hyperd, valid value includes devicemapper, overlay, and aufs
StorageDriver=overlay
# Hypervisor to run containers and pods, valid values are: libvirt, qemu, kvm, xen
Hypervisor=libvirt
# The tcp endpoint of gRPC API
gRPCHost=127.0.0.1:22318
EOF

systemctl restart hyperd
```

Setup CNI networking using bridge plugin

```sh
$ go get -d github.com/containernetworking/cni
$ cd $GOPATH/src/github.com/containernetworking/cni
$ sudo mkdir -p /etc/cni/net.d
$ sudo sh -c 'cat >/etc/cni/net.d/10-mynet.conf <<-EOF
{
    "cniVersion": "0.3.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "10.10.0.0/16",
        "routes": [
            { "dst": "0.0.0.0/0"  }
        ]
    }
}
EOF'
$ sudo sh -c 'cat >/etc/cni/net.d/99-loopback.conf <<-EOF
{
    "cniVersion": "0.3.0",
    "type": "loopback"
}
EOF'
$ ./build
$ sudo mkdir -p /opt/cni/bin
$ sudo cp bin/* /opt/cni/bin/
```

Then start frakti:

```sh
frakti --v=3 --logtostderr --listen=/var/run/frakti.sock --hyper-endpoint=127.0.0.1:22318
```

Finally, start kubernetes with frakti runtime:

```sh
cd $GOPATH/src/k8s.io/kubernetes
export KUBERNETES_PROVIDER=local
export CONTAINER_RUNTIME=remote
export CONTAINER_RUNTIME_ENDPOINT=/var/run/frakti.sock
hack/local-up-cluster.sh
```

To start using the cluster, open up another termimal and run:

```sh
export KUBERNETES_PROVIDER=local

cluster/kubectl.sh config set-cluster local --server=http://127.0.0.1:8080 --insecure-skip-tls-verify=true
cluster/kubectl.sh config set-context local --cluster=local
cluster/kubectl.sh config use-context local
cluster/kubectl.sh
```

## Documentation

Further information could be found at:

- [Deploying](docs/deploy.md)
- [End-to-end testing](docs/e2e-tests.md)
- [Kubelet container runtime API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/runtime-client-server.md)
- [HyperContainer](http://hypercontainer.io/)
- [The blog on k8s.io about Hypernetes](http://blog.kubernetes.io/2016/05/hypernetes-security-and-multi-tenancy-in-kubernetes.html)

## License

The work done has been licensed under Apache License 2.0.The license file can be found [here](LICENSE). You can find out more about license at [http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).
