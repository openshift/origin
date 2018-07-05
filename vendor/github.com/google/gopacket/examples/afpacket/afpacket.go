// Copyright 2018 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// afpacket provides a simple example of using afpacket with zero-copy to read
// packet data.
package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/google/gopacket/afpacket"

	_ "github.com/google/gopacket/layers"
)

var (
	iface      = flag.String("i", "eth0", "Interface to read from")
	cpuprofile = flag.String("cpuprofile", "", "If non-empty, write CPU profile here")
	count      = flag.Int64("c", -1, "If >= 0, # of packets to capture before returning")
	verbose    = flag.Int64("log_every", 1, "Write a log every X packets")
	addVLAN    = flag.Bool("add_vlan", false, "If true, add VLAN header")
)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		log.Printf("Writing CPU profile to %q", *cpuprofile)
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	log.Printf("Starting on interface %q", *iface)
	source, err := afpacket.NewTPacket(
		afpacket.OptInterface(*iface),
		afpacket.OptBlockSize(1<<20 /*1MB*/),
		afpacket.OptAddVLANHeader(*addVLAN))
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	bytes := uint64(0)
	packets := uint64(0)
	for ; *count != 0; *count-- {
		data, _, err := source.ZeroCopyReadPacketData()
		if err != nil {
			log.Fatal(err)
		}
		bytes += uint64(len(data))
		packets++
		if *count%*verbose == 0 {
			log.Printf("Read in %d bytes in %d packets", bytes, packets)
		}
	}
}
