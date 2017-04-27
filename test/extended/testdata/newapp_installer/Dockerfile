FROM openshift/origin-base

RUN yum install -y centos-release-openshift-origin
RUN yum install -y origin-clients

LABEL io.openshift.generate.job="true"
LABEL io.openshift.generate.token.as="serviceaccount"

ENV GIT_COMMITTER_NAME ___user
ENV GIT_COMMITTER_EMAIL ___user@redhat.com

ENTRYPOINT [ "/usr/bin/oc", "new-app", "https://github.com/openshift/ruby-hello-world" ]
