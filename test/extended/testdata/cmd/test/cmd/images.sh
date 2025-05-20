#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  original_context="$( oc config current-context )"
  os::cmd::expect_success 'oc login -u system:admin'
  cluster_admin_context="$( oc config current-context )"
  os::cmd::expect_success "oc config use-context '${original_context}'"
  oc delete project test-cmd-images-2 merge-tags --context=${cluster_admin_context}
  oc delete all,templates --all --context=${cluster_admin_context}

  exit 0
) &> /dev/null

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}"
# This test validates images and image streams along with the tag and import-image commands

# some steps below require that we use system:admin privileges, but we don't
# want to stomp on whatever context we were given when we started
original_context="$( oc config current-context )"
os::cmd::expect_success 'oc login -u system:admin'
cluster_admin_context="$( oc config current-context )"
os::cmd::expect_success "oc config use-context '${original_context}'"

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/images"
os::cmd::expect_success "oc get images --context='${cluster_admin_context}'"
os::cmd::expect_success "oc create -f '${TEST_DATA}/test-image.json' --context='${cluster_admin_context}'"
os::cmd::expect_success "oc delete images test --context='${cluster_admin_context}'"
echo "images: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/imagestreams"
os::cmd::expect_success 'oc get imageStreams'
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-image-stream.json'
# can't check status since there is no registry running so internalRegistryHostname isn't populated.
#os::cmd::expect_success_and_text "oc get imageStreams test --template='{{.status.dockerImageRepository}}'" 'test'
os::cmd::expect_success 'oc delete imageStreams test'
os::cmd::expect_failure 'oc get imageStreams test'

os::cmd::expect_success 'oc create -f ${TEST_DATA}/image-streams/image-streams-centos7.json'
# can't check status since there is no registry running so internalRegistryHostname isn't populated.
#os::cmd::expect_success_and_text "oc get imageStreams ruby --template='{{.status.dockerImageRepository}}'" 'ruby'
#os::cmd::expect_success_and_text "oc get imageStreams nodejs --template='{{.status.dockerImageRepository}}'" 'nodejs'
#os::cmd::expect_success_and_text "oc get imageStreams wildfly --template='{{.status.dockerImageRepository}}'" 'wildfly'
#os::cmd::expect_success_and_text "oc get imageStreams mysql --template='{{.status.dockerImageRepository}}'" 'mysql'
#os::cmd::expect_success_and_text "oc get imageStreams postgresql --template='{{.status.dockerImageRepository}}'" 'postgresql'
#os::cmd::expect_success_and_text "oc get imageStreams mongodb --template='{{.status.dockerImageRepository}}'" 'mongodb'
#os::cmd::expect_success_and_text "oc get imageStreams httpd --template='{{.status.dockerImageRepository}}'" 'httpd'

# verify the image repository had its tags populated
os::cmd::try_until_success 'oc get imagestreamtags wildfly:latest'
os::cmd::expect_success_and_text "oc get imageStreams wildfly --template='{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}'" '[0-9]{4}\-[0-9]{2}\-[0-9]{2}' # expect a date like YYYY-MM-DD
os::cmd::expect_success_and_text 'oc get istag' 'wildfly'

# create an image stream and post a mapping to it
#os::cmd::expect_success 'oc create imagestream test'
#os::cmd::expect_success 'oc create -f test/testdata/mysql-image-stream-mapping.yaml'
#os::cmd::expect_success_and_text 'oc get istag/test:new --template="{{ index .image.dockerImageMetadata.Config.Entrypoint 0 }}"' "docker-entrypoint.sh"
#os::cmd::expect_success_and_text 'oc get istag/test:new -o jsonpath={.image.metadata.name}' 'sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237'
# reference should point to the current repository, and that repository should match the reported dockerImageRepository for pushes
#repository="$( oc get is/test -o jsonpath='{.status.dockerImageRepository}' )"
#os::cmd::expect_success_and_text 'oc get istag/test:new -o jsonpath={.image.dockerImageReference}' "^$repository@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237"
#os::cmd::expect_success_and_text 'oc get istag/test:new -o jsonpath={.image.dockerImageReference}' "/$project/test@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237"

#repository="$( oc get is/test -o jsonpath='{.status.dockerImageRepository}' )"
#os::cmd::expect_success "oc annotate --context='${cluster_admin_context}' --overwrite image/sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237 images.openshift.io/deny-execution=true"
# TODO: re-enable before 4.0 release
#os::cmd::expect_failure_and_text "oc run vulnerable --image=${repository}:new --restart=Never" 'spec.containers\[0\].image: Forbidden: this image is prohibited by policy'

# test image stream tag operations
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.generation}' '2'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.kind}' 'ImageStreamTag'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.name}' '21.0'
os::cmd::expect_success 'oc annotate istag/wildfly:latest foo=bar'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.metadata.annotations.foo}' 'bar'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.annotations.foo}' 'bar'
os::cmd::expect_success 'oc annotate istag/wildfly:latest foo-'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.metadata.annotations}' 'bar'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.tag.annotations}' 'bar'
os::cmd::expect_success "oc patch istag/wildfly:latest -p='{\"tag\":{\"from\":{\"kind\":\"DockerImage\",\"name\":\"quay.io/wildfly/wildfly-centos7:19.0\"}}}'"
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.kind}' 'DockerImage'
os::cmd::expect_success_and_text 'oc get istag/wildfly:latest -o jsonpath={.tag.from.name}' 'quay.io/wildfly/wildfly-centos7:19.0'
os::cmd::expect_success_and_not_text 'oc get istag/wildfly:latest -o jsonpath={.tag.generation}' '2'

# create an image stream tag
os::cmd::expect_success 'oc create imagestreamtag tag:1 --from=wildfly:21.0'
os::cmd::expect_success 'oc create imagestreamtag tag:2 --from-image=quay.io/openshifttest/hello-openshift:openshift'
os::cmd::try_until_success 'oc get imagestreamtags tag:2'
os::cmd::expect_success 'oc create imagestreamtag tag:3 -A foo=bar'
os::cmd::expect_success 'oc create imagestreamtag tag:4 --from=:2'
os::cmd::expect_success 'oc create imagestreamtag tag:5 --from=tag:2'
os::cmd::expect_success 'oc create imagestreamtag tag:6 --reference --from-image=quay.io/openshifttest/hello-openshift:openshift'
os::cmd::expect_success 'oc create imagestreamtag tag:7 --reference-policy=Local --from=tag:2'
os::cmd::expect_success 'oc create istag tag:8 --insecure --from-image=quay.io/openshifttest/hello-openshift:openshift'
os::cmd::try_until_success 'oc get imagestreamtags tag:8'
os::cmd::expect_success 'oc create imagestreamtag tag:9 --scheduled --reference-policy=Local --from-image=quay.io/openshifttest/hello-openshift:openshift'
os::cmd::expect_success 'oc create imagestream tag-b'
os::cmd::expect_success 'oc create imagestreamtag tag-b:1 --from-image=quay.io/wildfly/wildfly-centos7:20.0'
os::cmd::expect_success 'oc create imagestreamtag tag-c:1 -A annotation.with.dots=are.ok'

os::cmd::expect_failure_and_text 'oc create imagestreamtag tag-b:1 --from=quay.io/wildfly/wildfly-centos7:20.0' 'registry may not be specified'
os::cmd::expect_failure_and_text 'oc create imagestreamtag tag-c --from-image=quay.io/openshifttest/hello-openshift:openshift' 'must be of the form <stream_name>:<tag>'
os::cmd::expect_failure_and_text 'oc create imagestreamtag tag-c:1 -A foo' 'annotations must be of the form key=value, but is "foo"'
os::cmd::expect_failure_and_text 'oc create imagestreamtag tag-c:2 --from=mysql --from-image=quay.io/openshifttest/hello-openshift:openshift' '\--from and --from-image may not be used together'

os::cmd::expect_success_and_text 'oc get istag/tag:1 -o jsonpath={.image.dockerImageReference}' 'wildfly-centos7.*@sha256:'
tag1=$( oc get istag/wildfly:20.0 -o jsonpath={.image.metadata.name} )
os::cmd::expect_success_and_text 'oc get istag/tag-b:1 -o jsonpath={.image.metadata.name}' "${tag1}"
os::cmd::expect_success_and_text 'oc get istag/tag:2 -o jsonpath={.image.dockerImageReference}' 'hello-openshift@sha256:'
tag2=$( oc get istag/tag:2 -o jsonpath={.image.metadata.name} )
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"3\")].annotations.foo}'" 'bar'
os::cmd::expect_success_and_text 'oc get istag/tag:4 -o jsonpath={.image.metadata.name}' "${tag2}"
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"4\")].from.name}'" '^2$'
os::cmd::expect_success_and_text 'oc get istag/tag:5 -o jsonpath={.image.metadata.name}' "${tag2}"
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"6\")].reference}'" 'true'
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"7\")].referencePolicy}'" 'Local'
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"8\")].importPolicy.insecure}'" 'true'
os::cmd::expect_success_and_text "oc get is/tag -o 'jsonpath={.spec.tags[?(@.name==\"9\")].importPolicy.scheduled}'" 'true'

os::cmd::expect_success 'oc delete imageStreams ruby'
os::cmd::expect_success 'oc delete imageStreams nodejs'
os::cmd::expect_success 'oc delete imageStreams wildfly'
os::cmd::expect_success 'oc delete imageStreams postgresql'
os::cmd::expect_success 'oc delete imageStreams mongodb'
os::cmd::expect_failure 'oc get imageStreams ruby'
os::cmd::expect_failure 'oc get imageStreams nodejs'
os::cmd::expect_failure 'oc get imageStreams postgresql'
os::cmd::expect_failure 'oc get imageStreams mongodb'
os::cmd::expect_failure 'oc get imageStreams wildfly'
os::cmd::try_until_success 'oc get imagestreamTags mariadb:10.5'
os::cmd::expect_success_and_text "oc get imagestreams mysql --template='{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}'" '[0-9]{4}\-[0-9]{2}\-[0-9]{2}' # expect a date like YYYY-MM-DD
os::cmd::expect_success 'oc describe istag/mysql:latest'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Environment:'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Image Created:'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Image Name:'
os::cmd::expect_success_and_text 'oc describe istag/mysql:latest' 'Layers:'
name=$(oc get istag/mysql:latest --template='{{ .image.metadata.name }}')
imagename="isimage/mysql@${name:0:15}"
os::cmd::expect_success "oc describe ${imagename}"
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Environment:'
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Image Created:'
os::cmd::expect_success_and_text "oc describe ${imagename}" 'Image Name:'

# test prefer-os and prefer-arch annotations
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-multiarch-stream.yaml'
os::cmd::try_until_success 'oc get istag test-multiarch-stream:linux-amd64'
os::cmd::try_until_success 'oc get istag test-multiarch-stream:linux-s390x'
os::cmd::expect_success_and_text 'oc get istag test-multiarch-stream:linux-amd64 --template={{.image.dockerImageMetadata.Architecture}}' 'amd64'
os::cmd::expect_success_and_text 'oc get istag test-multiarch-stream:linux-s390x --template={{.image.dockerImageMetadata.Architecture}}' 's390x'
os::cmd::expect_success 'oc delete is test-multiarch-stream'
echo "imageStreams: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/import-image"
# should follow the latest reference to 10.5 and update that, and leave latest unchanged
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 1).from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 2).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 2).from.name}}'" '10.5'
# import existing tag (implicit latest)
os::cmd::expect_success_and_text 'oc import-image mariadb' 'sha256:'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 1).from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 2).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 2).from.name}}'" '10.5'
# should prevent changing source
os::cmd::expect_failure_and_text 'oc import-image mariadb --from=quay.io/openshifttest/hello-openshift:openshift' "use the 'tag' command if you want to change the source"
os::cmd::expect_success 'oc describe is/mariadb'
# import existing tag (explicit)
os::cmd::expect_success_and_text 'oc import-image mariadb:10.5' "sha256:"
os::cmd::expect_success_and_text 'oc import-image mariadb:latest' "sha256:"
# import existing image stream creating new tag
os::cmd::expect_success_and_text 'oc import-image mariadb:external --from=quay.io/openshifttest/hello-openshift:openshift' "sha256:"
os::cmd::expect_success_and_text "oc get istag/mariadb:external --template='{{.tag.from.kind}}'" 'DockerImage'
os::cmd::expect_success_and_text "oc get istag/mariadb:external --template='{{.tag.from.name}}'" 'quay.io/openshifttest/hello-openshift:openshift'
# import creates new image stream with single tag
os::cmd::expect_failure_and_text 'oc import-image mariadb-new-single:latest --from=quay.io/openshifttest/hello-openshift:openshift' '\-\-confirm'
os::cmd::expect_success_and_text 'oc import-image mariadb-new-single:latest --from=quay.io/openshifttest/hello-openshift:openshift --confirm' 'sha256:'
os::cmd::expect_success_and_text "oc get is/mariadb-new-single --template='{{(len .spec.tags)}}'" '1'
os::cmd::expect_success 'oc delete is/mariadb-new-single'
# import creates new image stream with all tags
os::cmd::expect_failure_and_text 'oc import-image mariadb-new-all --from=quay.io/openshifttest/hello-openshift --all' '\-\-confirm'
os::cmd::expect_success_and_text 'oc import-image mariadb-new-all --from=quay.io/openshifttest/hello-openshift --all --confirm --request-timeout=1m' 'sha256:'
name=$(oc get istag/mariadb-new-all:openshift --template='{{ .image.metadata.name }}')
echo "import-image: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/tag"
# oc tag
os::cmd::expect_success 'oc tag mariadb:latest mariadb:tag1 --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 4).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success "oc tag mariadb@${name} mariadb:tag2 --alias"
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 5).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success 'oc tag mariadb:notfound mariadb:tag3 --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 6).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success 'oc tag --source=imagestreamtag mariadb:latest mariadb:tag4 --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 7).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success 'oc tag --source=istag mariadb:latest mariadb:tag5 --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 8).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_success "oc tag --source=imagestreamimage mariadb@${name} mariadb:tag6 --alias"
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 9).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success "oc tag --source=isimage mariadb@${name} mariadb:tag7 --alias"
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 10).from.kind}}'" 'ImageStreamImage'

os::cmd::expect_success 'oc tag --source=docker mariadb:latest mariadb:tag8 --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 11).from.kind}}'" 'DockerImage'

os::cmd::expect_success 'oc tag mariadb:latest mariadb:zzz mariadb:yyy --alias'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 12).from.kind}}'" 'ImageStreamTag'
os::cmd::expect_success_and_text "oc get is/mariadb --template='{{(index .spec.tags 13).from.kind}}'" 'ImageStreamTag'

os::cmd::expect_failure_and_text 'oc tag mariadb:latest tagtest:tag1 --alias' 'cannot set alias across'

# label image
imgsha256=$(oc get istag/mariadb:latest --template='{{ .image.metadata.name }}')
os::cmd::expect_success "oc label image ${imgsha256} foo=bar || true"
os::cmd::expect_success_and_text "oc get image ${imgsha256} --show-labels" 'foo=bar'

# tag labeled image
os::cmd::expect_success 'oc label is/mariadb labelA=value'
os::cmd::expect_success 'oc tag mariadb:latest mariadb:labeled'
os::cmd::expect_success_and_text "oc get istag/mariadb:labeled -o jsonpath='{.metadata.labels.labelA}'" 'value'
# test copying tags
os::cmd::expect_success 'oc tag quay.io/openshift/origin-cli:4.6 newrepo:latest'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'DockerImage'
os::cmd::try_until_success 'oc get istag/mariadb:10.5'
# default behavior is to copy the current image, but since this is an external image we preserve the dockerImageReference
os::cmd::expect_success 'oc tag mariadb:10.5 newrepo:latest'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .status.tags 0 \"items\" 0).dockerImageReference}}'" '^quay.io/centos7/mariadb-105-centos7@sha256:'
# when copying a tag that points to the internal registry, update the container image reference
#os::cmd::expect_success "oc tag test:new newrepo:direct"
#os::cmd::expect_success_and_text 'oc get istag/newrepo:direct -o jsonpath={.image.dockerImageReference}' "/$project/newrepo@sha256:"
# test references
os::cmd::expect_success 'oc tag mariadb:10.5 reference:latest --reference'
os::cmd::expect_success_and_text "oc get is/reference --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success_and_text "oc get is/reference --template='{{(index .spec.tags 0).reference}}'" 'true'
# create a second project to test tagging across projects
os::cmd::expect_success 'oc new-project test-cmd-images-2'
os::cmd::expect_success "oc tag $project/mariadb:10.5 newrepo:latest"
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success_and_text 'oc get istag/newrepo:latest -o jsonpath={.image.dockerImageReference}' 'quay.io/centos7/mariadb-105-centos7@sha256:'
# tag across projects without specifying the source's project
os::cmd::expect_success_and_text "oc tag newrepo:latest '${project}/mariadb:tag1'" "mariadb:tag1 set to"
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).name}}'" "latest"
# tagging an image with a DockerImageReference that points to the internal registry across namespaces updates the reference
#os::cmd::expect_success "oc tag $project/test:new newrepo:direct"
# reference should point to the current repository, and that repository should match the reported dockerImageRepository for pushes
# can't check status since there is no registry running so internalRegistryHostname isn't populated.
#repository="$( oc get is/newrepo -o jsonpath='{.status.dockerImageRepository}' )"
#os::cmd::expect_success_and_text 'oc get istag/newrepo:direct -o jsonpath={.image.dockerImageReference}' "^$repository@sha256:"
#os::cmd::expect_success_and_text 'oc get istag/newrepo:direct -o jsonpath={.image.dockerImageReference}' '/test-cmd-images-2/newrepo@sha256:'
# tagging an image using --reference does not
#os::cmd::expect_success "oc tag $project/test:new newrepo:indirect --reference"
#os::cmd::expect_success_and_text 'oc get istag/newrepo:indirect -o jsonpath={.image.dockerImageReference}' "/$project/test@sha256:"
os::cmd::expect_success "oc project $project"
# test scheduled and insecure tagging
os::cmd::expect_success 'oc tag --source=docker mariadb:10.5 newrepo:latest --scheduled'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).importPolicy.scheduled}}'" 'true'
os::cmd::expect_success_and_text "oc describe is/newrepo" 'updates automatically from registry mariadb:10.5'
os::cmd::expect_success 'oc tag --source=docker mariadb:10.5 newrepo:latest --insecure'
os::cmd::expect_success_and_text "oc describe is/newrepo" 'will use insecure HTTPS or HTTP connections'
os::cmd::expect_success_and_not_text "oc describe is/newrepo" 'updates automatically from'
os::cmd::expect_success_and_text "oc get is/newrepo --template='{{(index .spec.tags 0).importPolicy.insecure}}'" 'true'

# test creating streams that don't exist
os::cmd::expect_failure_and_text 'oc get imageStreams tagtest1' 'not found'
os::cmd::expect_failure_and_text 'oc get imageStreams tagtest2' 'not found'
os::cmd::expect_success 'oc tag mariadb:latest tagtest1:latest tagtest2:latest'
os::cmd::expect_success_and_text "oc get is/tagtest1 --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success_and_text "oc get is/tagtest2 --template='{{(index .spec.tags 0).from.kind}}'" 'ImageStreamImage'
os::cmd::expect_success 'oc delete is/tagtest1 is/tagtest2'
os::cmd::expect_success_and_text 'oc tag mariadb:latest tagtest:new1' 'Tag tagtest:new1 set to mariadb@sha256:'

# test deleting a spec tag using oc tag
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-stream.yaml'
os::cmd::expect_success_and_text 'oc tag test-stream:latest -d' 'Deleted'
os::cmd::expect_success 'oc delete is/test-stream'
echo "tag: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/delete-istag"
# test deleting a tag using oc delete
os::cmd::expect_success_and_text "oc get is perl --template '{{(index .spec.tags 0).name}}'" '5.30-'
os::cmd::expect_success_and_text "oc get is perl --template '{{(index .status.tags 0).tag}}'" '5.30-'
os::cmd::expect_success_and_text "oc describe is perl | sed -n -e '0,/^Tags:/d' -e '/^\s\+/d' -e '/./p' | head -n 1" '5.30-'
os::cmd::expect_success "oc delete istag/perl:5.30-el7 --context='${cluster_admin_context}'"
os::cmd::expect_success_and_not_text 'oc get is/perl --template={{.spec.tags}}' 'name:5.30-el7'
os::cmd::expect_success_and_not_text 'oc get is/perl --template={{.status.tags}}' 'tag:5.30-el7'
os::cmd::expect_success 'oc delete all --all'

echo "delete istag: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/merge-tags-on-apply"
os::cmd::expect_success 'oc new-project merge-tags'
os::cmd::expect_success 'oc create -f ${TEST_DATA}/image-streams/image-streams-centos7.json'
os::cmd::expect_success_and_text 'oc get is ruby -o jsonpath={.spec.tags[*].name}' '2.7-ubi8 3.0-ubi7 3.0-ubi9 3.3-ubi8 latest'

os::cmd::expect_success 'oc apply -f ${TEST_DATA}/modified-ruby-imagestream.json'
os::cmd::expect_success_and_text 'oc get is ruby -o jsonpath={.spec.tags[*].name}' '2.7-ubi8 3.0-ubi7 3.0-ubi9 3.3-ubi8 latest newtag'
os::cmd::expect_success_and_text 'oc get is ruby -o jsonpath={.spec.tags[0].annotations.version}' '2.7 patched'
os::cmd::expect_success 'oc delete project merge-tags'
echo "apply new imagestream tags: ok"
os::test::junit::declare_suite_end

# test importing images with wrong docker secrets
os::test::junit::declare_suite_start "cmd/images${IMAGES_TESTS_POSTFIX:-}/import-public-images-with-fake-secret"
os::cmd::expect_success 'oc new-project import-images'
os::cmd::expect_success 'oc create secret docker-registry dummy-secret1 --docker-server=docker.io --docker-username=dummy1 --docker-password=dummy1 --docker-email==dummy1@example.com'
os::cmd::expect_success 'oc create secret docker-registry dummy-secret2 --docker-server=docker.io --docker-username=dummy2 --docker-password=dummy2 --docker-email==dummy2@example.com'
os::cmd::expect_success_and_text 'oc import-image example --from=openshift/hello-openshift --confirm' 'imagestream.image.openshift.io/example imported'
os::cmd::expect_success 'oc delete project import-images'
echo "import public images with fake secret ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
