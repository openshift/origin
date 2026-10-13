// required for gomod to pull in packages.

package alpha_build_machinery

// this is a dependency magnet to make it easier to pull in the build-machinery.  We want a single import to pull all of it in.
import (
	_ "github.com/openshift/build-machinery-go/make"
	_ "github.com/openshift/build-machinery-go/make/lib"
	_ "github.com/openshift/build-machinery-go/make/targets"
	_ "github.com/openshift/build-machinery-go/make/targets/golang"
	_ "github.com/openshift/build-machinery-go/make/targets/openshift"
	_ "github.com/openshift/build-machinery-go/make/targets/openshift/operator"
	_ "github.com/openshift/build-machinery-go/scripts"
)
