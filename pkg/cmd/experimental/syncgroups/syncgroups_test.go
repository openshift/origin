package syncgroups

import "github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"

var _ interfaces.LDAPMemberExtractor = &LDAPInterface{}
var _ interfaces.LDAPGroupGetter = &LDAPInterface{}
var _ interfaces.LDAPGroupLister = &LDAPInterface{}
