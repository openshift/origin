# osrelease - Go library/binary for parsing osrelease

See the
[os-release](https://www.freedesktop.org/software/systemd/man/os-release.html)
documentation.

## Building

```bash
make deps      # Install dependencies
...
make osrelease # Create the binary
...
```

## Usage

**Note**: ``OSRelease`` type directly supports fields as defined by the
``os-release`` documentation. Fields that are present but are not
explicitly part of said documentation are added in ``ADDITIONAL_FIELDS``.
When using the command line binary these fields are accesses the same
way as supported fields. When using osrelease as a library to find unsupported
fields ``ADDITIONAL_FIELDS`` (``map[string]string``) will need to be searched.

### Command Line
```bash
# A single field
./osrelease field ID
fedora$

# A field that is not considered part of the supported set
./osrelease field REDHAT_BUGZILLA_PRODUCT_VERSION
26$

# A field that doesn't exist
$ ./osrelease field idonotexist
$ echo $?
1

# In a format
./osrelease yaml
name: Fedora
version: 26 (Workstation Edition)
id: fedora
...
$
```


### Library


```golang

import (
	"errors"

	"github.com/ashcrow/osrelease"
)

func RequireFedora() error {
	// The OSRelease instance using the default paths
	or, err := osrelease.New(nil)
	// Or to inspect the files in /sysroot
	//or, err := osrelease.NewWithOverrides("/sysroot/etc/osrelease", "/tmp/someplace/usr/lib/os-release"))
	// Handle the error however you see fit
	if err != nil {
		return err
	}

	if or.ID != "fedora" {
		return errors.New("Fedora Linux is a requirement")
	}

	// Everything is fine
	return nil
}
```
