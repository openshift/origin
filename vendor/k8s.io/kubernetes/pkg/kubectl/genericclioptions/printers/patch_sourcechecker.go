package printers

func init() {
	disallowedPackagePrefixes = append(disallowedPackagePrefixes,
		"github.com/openshift/origin/pkg/apps/apis/",
		"github.com/openshift/origin/pkg/authorization/apis/",
		"github.com/openshift/origin/pkg/build/apis/",
		"github.com/openshift/origin/pkg/image/apis/",
		"github.com/openshift/origin/pkg/network/apis/",
		"github.com/openshift/origin/pkg/oauth/apis/",
		"github.com/openshift/origin/pkg/project/apis/",
		"github.com/openshift/origin/pkg/quota/apis/",
		"github.com/openshift/origin/pkg/route/apis/",
		"github.com/openshift/origin/pkg/security/apis/",
		"github.com/openshift/origin/pkg/template/apis/",
		"github.com/openshift/origin/pkg/user/apis/",
	)
}
