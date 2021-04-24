# library-go/build-machinery-go
These are the building blocks for this and many of our other repositories to share code for Makefiles, helper scripts and other build related machinery.

## Makefiles
`make/` directory contains several predefined makefiles `(*.mk)` to choose from and include one of them as a base in your final `Makefile`. These are the predefined flows providing you with e.g. `build`, `test` or `verify` targets. To start with it is recommended you base Makefile on the corresponding `*.example.mk` using copy&paste.

As some advanced targets are generated, every Makefile contains `make help` target listing all the available ones. All of the "example" makefiles have a corresponding `.help` file listing all the targets available there.

Also for advanced use and if none of the predefined flows doesn't fit your needs, you can compose the flow from modules in similar way to how the predefined flows do,  

### Golang
Standard makefile for building pure Golang projects.
 - [make/golang.mk](make/golang.mk)
 - [make/golang.example.mk](make/golang.example.mk)
 - [make/golang.example.mk.help](make/golang.example.mk.help)

### Default
Standard makefile for OpenShift Golang projects. 

Extends [#Golang]().

 - [make/default.mk](make/default.mk)
 - [make/default.example.mk](make/default.example.mk)
 - [make/default.example.mk.help](make/default.example.mk.help)

### Operator
Standard makefile for OpenShift Golang projects. 

Extends [#Default]().

 - [make/operator.mk](make/operator.mk)
 - [make/operator.example.mk](make/operator.example.mk)
 - [make/operator.example.mk.help](make/operator.example.mk.help)


## Scripts
`scripts` contain more complicated logic that is used in some make targets.

## Contributing
### Updating generated files
We track the log output from the makefile tests to make sure any change is visible and can be audited. Unfortunately due to subtle linux tooling differences in distributions and versions, `make update` may not get you the exact output as the CI. To avoid it, just run the command in the same container as CI:   
```
podman run -it --rm --pull=always -v $( pwd ):/go/src/$( go list -m ) --workdir=/go/src/$( go list -m ) registry.svc.ci.openshift.org/openshift/release:rhel-8-release-golang-1.15-openshift-4.7 make update
```
