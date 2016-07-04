# Frakti

[![GoDoc](https://godoc.org/k8s.io/frakti?status.svg)](https://godoc.org/k8s.io/frakti)

## The hypervisor-based container runtime for Kubernetes

Frakti lets Kubernetes run pods and containers directly inside hypervisors via [HyperContainer](http://hypercontainer.io/). It is light weighted and portable, but can provide much stronger isolation with independent kernel than linux-namespace-based container runtimes.

<p align="center">
  <img src="docs/images/frakti.png" width="600">
</p>

Frakti serves as a kubelet container runtime API server. Its endpoint should be configured while starting kubelet.

## QuickStart

### Install

TODO

### Start

TODO

## Documentation

Further information could be found at:

- [WIP: Kubelet container runtime API](https://github.com/kubernetes/kubernetes/tree/master/docs/proposals/runtime-client-server.md)
- [HyperContainer](http://hypercontainer.io/)
- [The blog on k8s.io about Hypernetes](http://blog.kubernetes.io/2016/05/hypernetes-security-and-multi-tenancy-in-kubernetes.html)

## License

The work done has been licensed under Apache License 2.0.The license file can be found [here](LICENSE). You can find out more about license at [http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).
