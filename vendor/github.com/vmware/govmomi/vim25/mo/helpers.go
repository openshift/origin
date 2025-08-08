// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package mo

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// Write implements the cli package's Write(io.Writer) error interface for
// emitting objects to the command line.
func (l HttpNfcLease) Write(w io.Writer) error {

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Lease:\t%s\n", l.Reference().String())
	fmt.Fprintf(tw, "InitializeProgress:\t%d\n", l.InitializeProgress)
	fmt.Fprintf(tw, "TransferProgress:\t%d\n", l.TransferProgress)
	fmt.Fprintf(tw, "Mode:\t%s\n", l.Mode)
	fmt.Fprintf(tw, "State:\t%s\n", l.State)
	fmt.Fprintf(tw, "Capabilities:\n")
	fmt.Fprintf(tw, "  CorsSupported:\t%v\n", l.Capabilities.CorsSupported)
	fmt.Fprintf(tw, "  PullModeSupported:\t%v\n", l.Capabilities.PullModeSupported)

	if info := l.Info; info != nil {
		fmt.Fprintf(tw, "Info:\n")
		fmt.Fprintf(tw, "  Entity:\t%s\n", info.Entity.String())

		timeout := time.Second * time.Duration(info.LeaseTimeout)
		fmt.Fprintf(tw, "  Timeout:\t%s\n", timeout)

		fmt.Fprintf(tw, "  TotalDiskCapacityInKB:\t%d\n", info.TotalDiskCapacityInKB)

		fmt.Fprintf(tw, "  URLs:\n")
		for i := range info.DeviceUrl {
			du := info.DeviceUrl[i]
			fmt.Fprintf(tw, "    Datastore:\t%s\n", du.DatastoreKey)
			fmt.Fprintf(tw, "    DeviceKey:\t%s\n", du.Key)
			isDisk := false
			if du.Disk != nil {
				isDisk = *du.Disk
			}
			fmt.Fprintf(tw, "    IsDisk:\t%v\n", isDisk)
			fmt.Fprintf(tw, "    SSLThumbprint:\t%s\n", du.SslThumbprint)
			fmt.Fprintf(tw, "    Target:\t%s\n", du.TargetId)
			fmt.Fprintf(tw, "    URL:\t%s\n", du.Url)
		}
	}

	if err := l.Error; err != nil {
		fmt.Fprintf(tw, "Error:\t%s\n", err.LocalizedMessage)
	}

	return tw.Flush()
}
