# Baseboard Management Controller Remote Console

[![CI](https://github.com/gebn/bmc/actions/workflows/build.yaml/badge.svg)](https://github.com/gebn/bmc/actions/workflows/build.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gebn/bmc.svg)](https://pkg.go.dev/github.com/gebn/bmc)
[![Go Report Card](https://goreportcard.com/badge/github.com/gebn/bmc)](https://goreportcard.com/report/github.com/gebn/bmc)

This project implements an IPMI v2.0 remote console in pure Go, to interact with BMCs.

## Specifications

All section references in the code use the following documents:

 - ASF
    - [v2.0](specifications/asf_v2.0.pdf)
 - DCMI
    - [v1.0](specifications/dcmi_v1.0.pdf)
    - [v1.1](specifications/dcmi_v1.1.pdf)
    - [v1.5](specifications/dcmi_v1.5.pdf)
 - IPMI
    - [v1.5](specifications/ipmi_v1.5.pdf)
    - [v2.0](specifications/ipmi_v2.0.pdf)

## Contributing

Contributions in the form of bug reports and PRs are greatly appreciated.
Please see [`CONTRIBUTING.md`](CONTRIBUTING.md) for a few guidelines.
