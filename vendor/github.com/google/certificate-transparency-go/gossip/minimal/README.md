# Minimal Gossip

This directory holds **experimental** and unofficial code for an implementation
of minimal gossip for CT logs.

## Background

The base Certicate Transparency
[specification](https://tools.ietf.org/html/rfc6962.html) does not include any
explicit protection against an attack where a Log presents different views of
its contents to different users (a "split view").

The general idea of **gossip** for CT logs is that each log's signed values
(SCTs and STHs) are exchanged amongst gossip participants, so that a split
view can be detected.

The CT spec [recommends](https://tools.ietf.org/html/rfc6962.html#section-5)
that clients of CT logs gossip amongst themselves; however, this has two
problems:

 - It requires the details of the gossip mechanism to be defined, agreed upon
   and implemented by the clients.
 - It opens up the possibility of *fingerprinting* individual clients, based
   on the SCTs (or even STHs, for a log that issues them frequently) that
   they gossip.

## Design

The *minimal gossip* approach allows the STH values for CT logs to be gossiped
to other CT logs and stored in those logs.  If log X's STH is periodically
retrieved and stored in the Merkle trees of (independent) logs A, B and C,
then it becomes difficult for X to present a split tree in general --
particularly as the web of cross-logged STHs expands.

Note, however, that this minimal gossip approach does not protect against the
scenario where a malevolent CT log presents a split view to a single specific
client (unless that client happens to be an auditor that is cross-checking other
logs).

This approach does not require any new actors in the CT ecosystem, thus reducing
impact.  The impact is reduced further by embedding the gossiped STH values
inside synthetic X.509 certificates (using a new extension); this allows the
existing CT mechanisms to be used unmodified.

The only code change required by this approach is that the Log's certificate
chain validation must not reject the STH extension, which is marked as critical
to reduce the chances of the synthetic certificates being treated as valid web
PKI X.509 certificates.

## Operation

An entity that chooses to perform minimal gossip first chooses the set of source
logs whose STHs will be tracked.  The gossip operator then generates a new private
key, and builds a self-signed root certificate associated to that key; this then
needs to be added to each destination log's set of acceptable roots.

By accepting the gossiper's root, the destination log operator is implicitly
trusting that the gossiper will not overwhelm the log with an excess of
synthetic certificates.  This is obviously the case when the gossiper is the
same entity as the destination log operator.

The minimal gossip implementation is configured with this private key and set of
source logs; it then periodically:

 - retrieves STHs from the source logs
 - converts each STH to a leaf certificate that chains to the gossip root, with
   the STH embedded in the leaf certificate
 - adds the leaf to each configured destination log.

## X.509 Certificate Details

The synthetic certificate chains produced for minimal gossip consist of two
certificates.

 - The root certificate is a self-signed certificate that the receiving log has
   been configured to accept as a root.  The main feature of note is that:
    - This root is configured with an
      [extended key usage](https://tools.ietf.org/html/rfc5280.html#section-4.2.1.12)
      that indicates minimal gossip use; the OID used for this is currently
      1.3.6.1.4.1.11129.2.4.6.
 - The leaf certificate has:
    - A common name that includes STH information for convenience, currently of the form:
      ```
      STH-for-Pilot <http://ct.googleapis.com/pilot> @1519488735960: size=7834486 hash=1823b948e9bf8e34188810bd3d41455f4a48852f3ebcfd98cc05167a388b5712
      ```
    - A validity period of 24 hours starting from the STH's timestamp.
    - A minimal gossip EKU.
    - A critical X.509 extension identified by the OID 1.3.6.1.4.1.11129.2.4.5;
      the corresponding `OCTET STRING` holds the
      [TLS-encoding](https://tools.ietf.org/html/rfc5246.html#section-4) of the
      structure defined below.  This extension is marked as critical to reduce
      the chances of this synthetic leaf certificate being treated as a valid
      X.509 certificate.

**NOTE**: OID values are unassigned and subject to change.

The contents of the STH extension are defined as the following TLS structure:

```
enum { v1(0), (255) } Version;  /* From RFC6962 s3.2 */

struct {
    uint8 log_url<0..255>;
    Version version;
    uint64 tree_size;
    uint64 timestamp;
    opaque sha256_root_hash[32];
    digitally-signed struct {
        Version version;
        SignatureType signature_type = tree_hash;
        uint64 timestamp;
        uint64 tree_size;
        opaque sha256_root_hash[32];
    } TreeHeadSignature;  /* From RFC6962 s3.5 */
} LogSTHInfo;
```

The combination of the contents of this structure with the source log's public
key allows for
[STH consistency verification](https://tools.ietf.org/html/rfc6962.html#section-2.1.2).

## Verification

The STH entries stored in the destination log can be checked with the `goshawk`
tool. This tool takes a log configuration file similar to the gossiper's
configuration, describing:

 - A single destination log to be scanned for STH-holding certificates.
 - A set of source logs that will be checked for STH consistency.

The tool scans the destination log and processes certificates that have the
`LogSTHInfo` extension identified by OID 1.3.6.1.4.1.11129.2.4.5.  If the source
URL for the STH is one that has been configured, then an STH consistency check
is performed against a recent STH from that log.
