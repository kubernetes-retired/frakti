# frakti

[![GoDoc](https://godoc.org/k8s.io/frakti?status.svg)](https://godoc.org/k8s.io/frakti)

Frakti lets kubernetes run pods and containers directly inside hypervisors via [HyperContainer](http://hypercontainer.io/). It runs each pod inside a different virtual machine, and provides more strong resource isolation than linux-namespace-based container runtimes.

Frakti serves as a kubelet container runtime API server. Its endpoint should be configured while starting kubelet.

## Usage

### Install

### Start

## Documentation



Further information could be found at:

- [Kubelet container runtime API](https://github.com/kubernetes/kubernetes/tree/master/docs/proposals/runtime-client-server.md)
- [HyperContainer](http://hypercontainer.io/)

## License

The work done has been licensed under Apache License 2.0.The license file can be found [here](LICENSE). You can find out more about license at [http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).
