Directory Contents
==================

This directory holds data files for testing; **under no circumstances should these
files be used in production**.

Some of the data files are generated from other data files; the [`Makefile`](Makefile) has commands for doing this, but
the generated files are checked in for convenience.


Trillian Server Keys
--------------------

Files of the form `*-server.privkey.pem` hold private keys for Trillian servers, with the corresponding public keys
stored in `*-server.pubkey.pem`.  The following sets of files are available:

 - `ct-http-server`: CT Log personality; password `dirk`.


X.509 Certificates
------------------

A fake certificate authority is used when testing a Certificate Transparency personality for Trillian, which uses:

 - `fake-ca.privkey.pem`: Private key for the CA; password `gently`.
 - `fake-ca.cert`: CA certificate.

 There is also an intermediate certificate authority, which uses:

 - `int-ca.privkey.pem`: Private key; password `babelfish`.
 - `int-ca.cert`: Certificate.
