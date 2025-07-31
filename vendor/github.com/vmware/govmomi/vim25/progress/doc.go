// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

/*
The progress package contains functionality to deal with progress reporting.
The functionality is built to serve progress reporting for infrastructure
operations when talking the vSphere API, but is generic enough to be used
elsewhere.

At the core of this progress reporting API lies the Sinker interface. This
interface is implemented by any object that can act as a sink for progress
reports. Callers of the Sink() function receives a send-only channel for
progress reports. They are responsible for closing the channel when done.
This semantic makes it easy to keep track of multiple progress report channels;
they are only created when Sink() is called and assumed closed when any
function that receives a Sinker parameter returns.
*/
