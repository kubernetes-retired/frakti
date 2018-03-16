# Frakti v2

[![Build Status](https://travis-ci.org/kubernetes/frakti.svg?branch=master)](https://travis-ci.org/kubernetes/frakti) [![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes/frakti)](https://goreportcard.com/report/github.com/kubernetes/frakti)

## Kubernetes + Containerd + Kata##

Frakti v2 is an open-source project created by [sig-node](https://github.com/kubernetes/community/tree/master/sig-node) and [KataContainers](https://katacontainers.io/) maintainers to enable Secure Container Runtime in Kubernetes project.

Instead of a monolithic CRI shim like the current [frakti](https://github.com/kubernetes/frakti), Frakti v2 **only** provides a set of "kit" components, specifically, containerd-kata, CNI libraries, persistent volume libraries which can be used as building blocks in conjunction with Kubernetes and [containerd](https://github.com/containerd/containerd).

So we only expect users to: setup Kubernetes, choose containerd as runtime, with these kits for Kata. 

Done!

## "KISS"

We only maintains minimal components in Frakti v2 by following [KISS](https://en.wikipedia.org/wiki/KISS_principle) principle. Though some kits may have to temporarily stay here for cooperating reason, they will be contributed back to upstream org like [containerd](https://github.com/containerd) eventually.

## Design Documentation

[Please check the design doc here](https://docs.google.com/document/d/1znUEfsl-J5WGVpRGZEFQtD-kNwqhFSvRSKly7cS7d8M)

## For GSoC '18 candidates

We really appreciate your interest in this `Kata + container + Kubernetes` project! 

> This topic is not easy, but core maintainer of Kata @laijs, sig-node member @resouer, and Google engineer @Random-liu will mentor your work.

Please always remember to send PR to this `containerd-kata` branch of `frakti`. Master branch will not accept any PR related to this project.


## License

The work done has been licensed under Apache License 2.0.The license file can be found [here](LICENSE). You can find out more about license at [http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).