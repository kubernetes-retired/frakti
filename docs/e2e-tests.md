# End-to-End Testing in Frakti

Updated: 9/4/2016

- [End-to-End Testing in Frakti](#end-to-end-testing-in-frakti)
  - [Overview](#overview)
  - [Running the Tests](#running-the-tests)


## Overview

End-to-end (e2e) tests for frakti provide a mechanism to test end-to-end behavior of the system. Similar as kubernetes, the e2e tests in frakti are built atop of
[Ginkgo](http://onsi.github.io/ginkgo/) and
[Gomega](http://onsi.github.io/gomega/).


## Running the Tests

Frakti acts as a manager of hyper container runtime, before we could run e2e tests
in frakti, `hyperd` should be guaranteed running on localhost. In default way, `hyperd` should start with gRPC endpoint `127.0.0.1:22318`. If you are not sure hyperd is configured properly, here are the steps:

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

There are a variety of ways to run e2e tests, we only recommend two canonical ways:

```sh
make test-e2e
```

or 

```sh
hack/test-e2e.sh
```
