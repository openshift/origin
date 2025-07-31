// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

/*
Package vim25 provides a minimal client implementation to use with other
packages in the vim25 tree. The code in this package intentionally does not
take any dependendies outside the vim25 tree.

The client implementation in this package embeds the soap.Client structure.
Additionally, it stores the value of the session's ServiceContent object. This
object stores references to a variety of subsystems, such as the root property
collector, the session manager, and the search index. The client is fully
functional after serialization and deserialization, without the need for
additional requests for initialization.
*/
package vim25
