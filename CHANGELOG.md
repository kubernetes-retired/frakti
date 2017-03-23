<!-- TOC -->

- [v0.1](#v01)
    - [Features](#features)
    - [HyperContainer specific notes](#hypercontainer-specific-notes)
    - [External Dependency Version Information](#external-dependency-version-information)
    - [Kubelet Node e2e tests](#kubelet-node-e2e-tests)
    - [Known issues](#known-issues)

<!-- /TOC -->

# v0.1

Frakti lets Kubernetes run pods and containers directly inside hypervisors via HyperContainer. And with hybrid runtimes, privileged pods are still running by docker.

## Features

* full pod/container/image lifecycle management
* streaming interfaces (exec/attach/port-forward)
* CNI network plugin integration
* hybrid docker and hypercontainer runtimes to fully support both regular and privileged pods
  * pods are running by hypercontainer by default
  * pods will be running by docker if they are
    * privileged
    * with annotation `runtime.frakti.alpha.kubernetes.io/OSContainer=true`
    * configured to use host namespaces (net, pid, ipc)

## HyperContainer specific notes
 
* for pods are running inside hypervisors, resource limits (cpu/memory) should be set. If not, they will be running with 1 vcpu and 64MB memory by default
* [Security context](https://kubernetes.io/docs/user-guide/security-context/) is not supported by hypercontainer runtime (but it works properly with docker)

## External Dependency Version Information

Frakti v0.1 has been tested against:

- Kubernetes v1.6.0
- Hyperd v0.8.0
- Docker v1.12.6 (and any other Docker version work with Kubernetes v1.6.0)
- CNI v0.5.1

## Kubelet Node e2e tests

Frakti has passed 112 of 121 node e2e tests with kubernetes v1.6.0-beta.4. In the failed 9 test cases:

- 6 of them are related with volume mounting, which seems buggy in hyperd
- 1 of them is a known issue of hyperd ([#564](https://github.com/hyperhq/hyperd/issues/564))
- 2 of them still have no clear root causes found

See [#109](https://github.com/kubernetes/frakti/issues/109) for more details.


## Known issues

* Only CNI bridge plugin is supported yet ([#69](https://github.com/kubernetes/frakti/issues/69))
* Kill container is not enforced while timeout ([#100](https://github.com/kubernetes/frakti/issues/100))
* Setting CNI network plugins dynamically is not supported yet ([#110](https://github.com/kubernetes/frakti/issues/110))
