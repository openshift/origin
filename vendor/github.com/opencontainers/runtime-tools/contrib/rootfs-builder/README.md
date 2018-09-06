# Building `rootfs-*.tar.gz`

The root filesystem tarball is based on [Gentoo][]'s stage3s (e.g. the
[amd64 stage3][stage3-amd64]).  We check for a valid [GnuPG
signature][gentoo-signatures]), copying a [minimal
subset](rootfs-files) to the root filesytem, and adding symlinks for
all BusyBox commands.  To rebuild the tarball based on a newer stage3,
run:

```
$ touch get-stage3.sh
$ make rootfs-amd64.tar.gz
```

## Platform tarballs

The extraction requires a local machine capable of executing the
extracted BusyBox.  Extraction assumes amd64, but if your local
machine is a different platform, set `ARCHES`.  For example:

```
$ make ARCHES='i486 i686 amd64'
```

The architecture identifiers can be found by browsing Gentoo's
[releases][] for `autobuilds/latest-stage3-{arch}.txt`.  Examples
include:

* `amd64` (the default)
* `arm64`
* `arm7a`
* `i486`
* `i686`
* `ppc64-64ul`

and many more.

## Getting Gentoo's Release Engineering public key

If `make rootfs-….tar.gz` gives an error like:

```
gpg --verify downloads/stage3-amd64-current.tar.bz2.DIGESTS.asc
gpg: Signature made Thu 14 Jan 2016 09:00:11 PM EST using RSA key ID 2D182910
gpg: Can't check signature: public key not found
```

you will need to [add the missing public key to your
keystore][gentoo-signatures].  One way to do that is by [asking a
keyserver][recv-keys]:

```
$ gpg --keyserver pool.sks-keyservers.net --recv-keys 2D182910
```

[Gentoo]: https://www.gentoo.org/
[gentoo-signatures]: https://www.gentoo.org/downloads/signatures/
[recv-keys]: https://www.gnupg.org/documentation/manuals/gnupg/Operational-GPG-Commands.html
[releases]: http://distfiles.gentoo.org/releases/
[stage3-amd64]: http://distfiles.gentoo.org/releases/amd64/autobuilds/
