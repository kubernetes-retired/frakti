# Networking

Frakti is using CNI for setting up pod's networking. A list of validated plugins includes

- Bridge
- Flannel
- Calico
- Weave

## [Bridge](https://github.com/containernetworking/plugins/tree/master/plugins/main/bridge)

Firstly, install CNI:

```sh
$ sudo mkdir -p /etc/cni/net.d  /opt/cni/bin
$ git clone https://github.com/containernetworking/plugins $GOPATH/src/github.com/containernetworking/plugins
$ cd $GOPATH/src/github.com/containernetworking/plugins
$ ./build.sh
$ sudo cp bin/* /opt/cni/bin/
```

Then config CNI to use bridge and portmap plugin:

```sh
$ sudo sh -c 'cat >/etc/cni/net.d/10-mynet.conflist <<-EOF
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
$ sudo sh -c 'cat >/etc/cni/net.d/99-loopback.conf <<-EOF
{
    "cniVersion": "0.3.1",
    "type": "loopback"
}
EOF'
```

## [Flannel](https://github.com/coreos/flannel)

Remove other cni network configure if they are already configured:

```sh
rm -f /etc/cni/net.d/*
```

Then setup flannel plugin by running:

```sh
kubectl create -f https://github.com/coreos/flannel/raw/master/Documentation/kube-flannel.yml
```

## [Calico](https://www.projectcalico.org)

Remove other cni network configure if they are already configured:

```sh
rm -f /etc/cni/net.d/*
```

Then setup calico plugin by running:

```sh
kubectl apply -f https://docs.projectcalico.org/v2.4/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml
```

## [Weave](https://www.weave.works/)

Remove other cni network configure if they are already configured:

```sh
rm -f /etc/cni/net.d/*
```

Then setup weave plugin by running:

```sh
kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')"
```
