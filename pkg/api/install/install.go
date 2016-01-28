package install

import (
	_ "github.com/openshift/origin/pkg/authorization/api/install"
	_ "github.com/openshift/origin/pkg/build/api/install"
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	_ "github.com/openshift/origin/pkg/image/api/install"
	_ "github.com/openshift/origin/pkg/oauth/api/install"
	_ "github.com/openshift/origin/pkg/project/admission/requestlimit/install"
	_ "github.com/openshift/origin/pkg/project/api/install"
	_ "github.com/openshift/origin/pkg/route/api/install"
	_ "github.com/openshift/origin/pkg/sdn/api/install"
	_ "github.com/openshift/origin/pkg/template/api/install"
	_ "github.com/openshift/origin/pkg/user/api/install"
)
