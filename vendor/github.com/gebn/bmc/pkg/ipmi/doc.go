// Package ipmi implements the message and layer formats of IPMI v1.5 and v2.0.
// It is a subset of these specifications, with a long-term goal of implementing
// more commands as they become relevant.
//
// This contains everything needed to build a particular IPMI packet, but it has
// no knowledge about how to string them together. That is done by the root bmc
// package, which heavily depends on this. This package is not internal because
// the root package leaks types like AuthenticationAlgorithm.
package ipmi
