<!-- TOC -->
- [v1.1.1](#v111)
    - [Features and updates](#features-and-updates)
    - [External Dependency Version Information](#external-dependency-version-information)
- [v1.1](#v11)
    - [Features and updates](#features-and-updates)
    - [External Dependency Version Information](#external-dependency-version-information)
- [v1.0](#v10)
    - [Features and updates](#features-and-updates)
    - [External Dependency Version Information](#external-dependency-version-information)
- [v0.3](#v03)
    - [Features and updates](#features-and-updates-1)
    - [External Dependency Version Information](#external-dependency-version-information-1)
    - [Kubelet Node e2e tests](#kubelet-node-e2e-tests)
    - [Known issues](#known-issues)
- [v0.2](#v02)
    - [Features and updates](#features-and-updates-2)
    - [Kubelet Node e2e tests](#kubelet-node-e2e-tests-1)
    - [Known issues](#known-issues-1)
- [v0.1](#v01)
    - [Features](#features)
    - [HyperContainer specific notes](#hypercontainer-specific-notes)
    - [External Dependency Version Information](#external-dependency-version-information-2)
    - [Kubelet Node e2e tests](#kubelet-node-e2e-tests-2)
    - [Known issues](#known-issues-2)

<!-- /TOC -->

# v1.1.1

This release includes enhances and bug fixes. It has also passed all node e2e conformance tests.

**Features and Updates**

- #256 Fix IP with mask in pod status by resouer
- #255 Fix CI build failures caused by hyperd update by bergwolf
- #254 Fix full docker image path inconsistency by resouer
- #251 Annotate kube-dns to use Linux container to manage it by resouer
- #246 Fix nil pointer when hostpath is invalid by resouer

**External Dependency Version Information**

Kubernetes v1.8
Hyperd v1.0
Docker v1.12-v17.03

# v1.1

This release includes enhances and bug fixes. It has also passed all node e2e conformance tests.

**Features and Updates**

- #192 Improves CNI plugin compatibility
- #196 #208 Adds general support for CNI plugins, e.g. flannel and calico plugin
- #219 #223 #226 Adds experimental support for unikernel
- #217 Fixes problem of weave plugin
- #199 #207 Adds cinder flexvolume plugin
- #188 Fixes CNI podName problem
- #189 Fixes CNI cleanup when sandbox runs failure
- #205 Increases memory limits for kube-dns
- #211 #212 #221 #224 #230 Adds unit tests for various packages
- #237 Update frakti and its vendor to ensure compatibility with Kubernetes 1.8

**External Dependency Version Information**

Kubernetes v1.8
Hyperd v1.0
Docker v1.12-v17.03


# v1.0

This release includes enhances and bug fixes. It has also passed all node e2e conformance tests.

## Features and updates

- Upgrade to hyperd v1.0 and kubernetes v1.7
- Have passed full node e2e conformance tests and CRI validation tests
- Enhanced deployment steps and scripts
- [#160](https://github.com/kubernetes/frakti/pull/160) Add support for CNI plugin chaining
- [#162](https://github.com/kubernetes/frakti/pull/162) Add support for port mapping
- [#170](https://github.com/kubernetes/frakti/pull/170) Add support for container readonly rootfs

## External Dependency Version Information

Frakti v1.0 has been tested against:

- Kubernetes v1.7
- Hyperd v1.0
- Docker v1.12.6

# v0.3

This release includes enhances and bug fixes.

## Features and updates

- Have passed full node e2e conformance tests
- Enhanced deployment steps and scripts
- [#144](https://github.com/kubernetes/frakti/pull/144) Fix logpath in container status
- [#145](https://github.com/kubernetes/frakti/pull/145) Fix CodeExitError in streaming exec
- [#147](https://github.com/kubernetes/frakti/pull/147) Set force when removing image
- [#152](https://github.com/kubernetes/frakti/pull/152) Make default cpu and memory configurable

## External Dependency Version Information

Frakti v0.3 has been tested against:

- Kubernetes v1.6.4
- Hyperd v0.8.1
- Docker v1.12.6

## Kubelet Node e2e tests

Frakti has passed 120 of 121 node e2e tests. ALl failed cases are related with upstream hyperd issues.

See [#109](https://github.com/kubernetes/frakti/issues/109) for more details.

## Known issues

- [#161](https://github.com/kubernetes/frakti/issues/161) readonly rootfs is not supported because hyperd (hyperhq/hyperd#638)



# v0.2

This release includes enhances and bug fixes.

## Features and updates

- [#113](https://github.com/kubernetes/frakti/pull/113) Support setting CNI network plugins dynamically
- [#114](https://github.com/kubernetes/frakti/pull/114) Enable force kill container on timeout
- [#116](https://github.com/kubernetes/frakti/pull/116) Do not fail on removing nonexist pods
- [#118](https://github.com/kubernetes/frakti/pull/118) Avoid panic when labels are nil
- [#119](https://github.com/kubernetes/frakti/pull/119) Support dns options and searches
- [#126](https://github.com/kubernetes/frakti/pull/126) Fix hostPid support
- [#130](https://github.com/kubernetes/frakti/pull/130) Update latest frakti architecture

## Kubelet Node e2e tests

Frakti has passed 118 of 121 node e2e tests. ALl failed cases are related with upstream hyperd issues.

See [#109](https://github.com/kubernetes/frakti/issues/109) for more details.

## Known issues

- [#124](https://github.com/kubernetes/frakti/issues/124) nosuchimagehash: No such image with digest
- [#122](https://github.com/kubernetes/frakti/issues/122) port mapping is not supported yet

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
