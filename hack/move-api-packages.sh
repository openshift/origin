#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"



function fixup_imports() {
	echo "Moving $1 to $2..."
	startingPath=${OS_ROOT}/pkg/$1/api
	endingPath=${OS_ROOT}/pkg/$1/apis/$2
	startingPackagePrefix="github.com/openshift/origin/pkg/$1/api"
	set +e

	files=$(find . -path ./tools -prune -name "*generated*" -prune -o -type f -name "*.go" -print0 | xargs -0 grep "	\"${startingPackagePrefix}\"" -l)
	echo $files | xargs sed -i "s|	\"${startingPackagePrefix}\"|	$1api \"${startingPackagePrefix}\"|g"
	echo $files | xargs sed -i "s|api\.|$1api\.|g"
	echo $files | xargs sed -i "s|k$1api\.|kapi\.|g"
	echo $files | xargs sed -i "s|o$1api\.|oapi\.|g"
	echo $files | xargs sed -i "s|s2i$1api\.|s2iapi\.|g"
	echo $files | xargs sed -i "s|authorization$1api\.|authorizationapi\.|g"
	echo $files | xargs sed -i "s|build$1api\.|buildapi\.|g"
	echo $files | xargs sed -i "s|deploy$1api\.|deployapi\.|g"
	echo $files | xargs sed -i "s|image$1api\.|imageapi\.|g"
	echo $files | xargs sed -i "s|oauth$1api\.|oauthapi\.|g"
	echo $files | xargs sed -i "s|project$1api\.|projectapi\.|g"
	echo $files | xargs sed -i "s|quota$1api\.|quotaapi\.|g"
	echo $files | xargs sed -i "s|route$1api\.|routeapi\.|g"
	echo $files | xargs sed -i "s|sdn$1api\.|sdnapi\.|g"
	echo $files | xargs sed -i "s|security$1api\.|securityapi\.|g"
	echo $files | xargs sed -i "s|template$1api\.|templateapi\.|g"
	echo $files | xargs sed -i "s|user$1api\.|userapi\.|g"
	echo $files | xargs sed -i "s|auth$1api\.|authapi\.|g"
	echo $files | xargs sed -i "s|config$1api\.|configapi\.|g"
	echo $files | xargs sed -i "s|clientcmd$1api\.|clientcmdapi\.|g"
	echo $files | xargs sed -i "s|server$1api\.|serverapi\.|g"
	echo $files | xargs sed -i "s|meta$1api\.|metaapi\.|g"
	echo $files | xargs sed -i "s|kapi$1api\.|kapi\.|g"
	echo $files | xargs sed -i "s|meta$1apiv1\.|metaapiv1\.|g"
	echo $files | xargs sed -i "s|kapi$1apiv1\.|kapiv1\.|g"
	files=$(find . -path ./tools -prune -name "*generated*" -prune -o -type f  -name "*.go" -print0 | xargs -0 grep "	\"${startingPackagePrefix}/v1\"" -l)
	echo $files | xargs sed -i "s|	\"${startingPackagePrefix}/v1\"|	$1apiv1 \"${startingPackagePrefix}/v1\"|g"
	echo $files | xargs sed -i "s|v1\.|$1apiv1\.|g"
	echo $files | xargs sed -i "s|meta$1apiv1\.|metav1\.|g"
	echo $files | xargs sed -i "s|kapi$1apiv1\.|kapiv1\.|g"
	echo $files | xargs sed -i "s|k$1apiv1\.|kv1\.|g"

	set -e
}

function move() {
	echo "Moving $1 to $2..."
	startingPath=${OS_ROOT}/pkg/$1/api
	endingPath=${OS_ROOT}/pkg/$1/apis/$2
	startingPackagePrefix="github.com/openshift/origin/pkg/$1/api"
	endingPackagePrefix="github.com/openshift/origin/pkg/$1/apis/$2"

	mkdir -p ${endingPath}
	mv ${startingPath}/* ${endingPath}
	find ${endingPath} -maxdepth 1 -name "*.go" | xargs sed -i "s|package api|package $2|g"
	find ${endingPath} -maxdepth 1 -name "*.go" | xargs sed -i "s|Convert_api_|Convert_$2_|g"
	find ${endingPath} -maxdepth 1 -name "*.go" | xargs sed -i "s|_To_api_|_To_$2_|g"
	find ${endingPath} -maxdepth 1 -name "*.go" | xargs sed -i "s|autoConvert_api_|autoConvert_$2_|g"
	find ${endingPath}/v1 -maxdepth 1 -name "*.go" | xargs sed -i "s|Convert_api_|Convert_$2_|g"
	find ${endingPath}/v1 -maxdepth 1 -name "*.go" | xargs sed -i "s|_To_api_|_To_$2_|g"
	find ${endingPath}/v1 -maxdepth 1 -name "*.go" | xargs sed -i "s|autoConvert_api_|autoConvert_$2_|g"
	
	find ${OS_ROOT} -path ./vendor -prune -o -path ./cmd/service-catalog -prune -o -path ./cmd/cluster-capacity -prune -o -name "*.go" -print0 | xargs -0 sed -i "s|${startingPackagePrefix}|${endingPackagePrefix}|g"
	# find ${OS_ROOT} -path ./vendor -prune -o -path ./cmd/service-catalog -prune -o -path ./cmd/cluster-capacity -prune -o -name "*.sh" -print0 | xargs -0 sed -i "s|${startingPackagePrefix}|${endingPackagePrefix}|g"
}

fixup_imports authorization authorization
fixup_imports build build
fixup_imports deploy apps
fixup_imports image image
fixup_imports oauth oauth
fixup_imports project project
fixup_imports quota quota
fixup_imports route route
fixup_imports sdn network
fixup_imports security security
fixup_imports template template
fixup_imports user user

move authorization authorization
move build build
move deploy apps
move image image
move oauth oauth
move project project
move quota quota
move route route
move sdn network
move security security
move template template
move user user

# one offs
sed -i "s|DeepCopy_api_PolicyRule|DeepCopy_authorization_PolicyRule|g" pkg/authorization/authorizer/scope/converter.go
sed -i "s|Convert_v1_ResourceQuotaStatus_To_quota_ResourceQuotaStatus|Convert_v1_ResourceQuotaStatus_To_api_ResourceQuotaStatus|g" pkg/quota/apis/quota/v1/conversion.go
sed -i "s|Convert_quota_ResourceQuotaStatus_To_v1_ResourceQuotaStatus|Convert_api_ResourceQuotaStatus_To_v1_ResourceQuotaStatus|g" pkg/quota/apis/quota/v1/conversion.go
sed -i '13d' pkg/project/apis/project/validation/validation.go
sed -i '18d' pkg/diagnostics/networkpod/util/util.go
sed -i '23d' pkg/project/registry/project/proxy/proxy.go
sed -i '16d' pkg/dockerregistry/testutil/fakeopenshift.go
sed -i "s|authorizationapi.Convert_api_ClusterRole_To_rbac_ClusterRole|authorizationapi.Convert_authorization_ClusterRole_To_rbac_ClusterRole|g" pkg/authorization/controller/authorizationsync/normalize.go
sed -i "s|authorizationapi.Convert_api_ClusterRoleBinding_To_rbac_ClusterRoleBinding|authorizationapi.Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding|g" pkg/authorization/controller/authorizationsync/normalize.go
sed -i "s|authorizationapi.Convert_api_Role_To_rbac_Role|authorizationapi.Convert_authorization_Role_To_rbac_Role|g" pkg/authorization/controller/authorizationsync/normalize.go
sed -i "s|authorizationapi.Convert_api_RoleBinding_To_rbac_RoleBinding|authorizationapi.Convert_authorization_RoleBinding_To_rbac_RoleBinding|g" pkg/authorization/controller/authorizationsync/normalize.go
sed -i "s|_api_Route|_route_Route|g" pkg/api/install/install.go
sed -i "s|_api_Build|_build_Build|g" pkg/api/install/install.go
sed -i "s|_api_OAuth|_oauth_OAuth|g" pkg/api/install/install.go
sed -i "s|_api_Project|_project_Project|g" pkg/api/install/install.go
sed -i "s|_api_Template|_template_Template|g" pkg/api/install/install.go
sed -i "s|_api_BrokerTemplateInstance|_template_BrokerTemplateInstance|g" pkg/api/install/install.go
sed -i "s|_api_DeploymentConfig|_apps_DeploymentConfig|g" pkg/api/install/install.go
sed -i "s|_api_Image|_image_Image|g" pkg/api/install/install.go
sed -i "s|_api_ClusterPolic|_authorization_ClusterPolic|g" pkg/api/install/install.go
sed -i "s|_api_Polic|_authorization_Polic|g" pkg/api/install/install.go
sed -i "s|_api_ClusterRole|_authorization_ClusterRole|g" pkg/api/install/install.go
sed -i "s|_api_Role|_authorization_Role|g" pkg/api/install/install.go
sed -i "s|_api_IsPersonalSubjectAccessRevie|_authorization_IsPersonalSubjectAccessRevie|g" pkg/api/install/install.go
sed -i "s|_api_User|_user_User|g" pkg/api/install/install.go
sed -i "s|_api_Identity|_user_Identity|g" pkg/api/install/install.go
sed -i "s|_api_Group|_user_Group|g" pkg/api/install/install.go


hack/update-generated-conversions.sh
hack/update-generated-deep-copies.sh
hack/update-generated-defaulters.sh
hack/update-generated-clientsets.sh
hack/update-generated-informers.sh
hack/update-generated-listers.sh
OS_BUILD_ENV_PRESERVE=api:docs:pkg ./hack/env hack/update-generated-protobuf.sh
hack/update-generated-openapi.sh

set +e
hack/verify-gofmt.sh | xargs -n 1 gofmt -s -w
set -e

nice make


# remove the old to avoid extra files.
rm -rf api/protobuf-spec
hack/update-generated-swagger-spec.sh