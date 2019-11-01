// required for gomod to pull in packages.

package alpha_build_machinery

// this is a dependency magnet to make it easier to pull in the build-machinery.  We want a single import to pull all of it in.
import (
	_ "github.com/openshift/library-go/alpha-build-machinery/make"
	_ "github.com/openshift/library-go/alpha-build-machinery/make/lib"
	_ "github.com/openshift/library-go/alpha-build-machinery/make/targets"
	_ "github.com/openshift/library-go/alpha-build-machinery/make/targets/golang"
	_ "github.com/openshift/library-go/alpha-build-machinery/make/targets/openshift"
	_ "github.com/openshift/library-go/alpha-build-machinery/make/targets/openshift/operator"
	_ "github.com/openshift/library-go/alpha-build-machinery/scripts"
)
