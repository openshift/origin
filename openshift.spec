#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%{!?commit:
%global commit 0128300716d74b7ea4227adad09506bfbda452c9
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# Openshift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 3+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.3.2-21-g0128300-dirty -X github.com/openshift/origin/pkg/version.commitFromGit 0128300 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit c977a45 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.10.0-503-gc977a45
}
# String used for --images flag
# If you're setting docker_registry make sure it ends in a trailing /
%if "%{dist}" == ".el7ose"
  %global docker_registry registry.access.redhat.com/
  %global docker_namespace openshift3_beta
  %global docker_prefix ose
%else
  %global docker_namespace openshift
  %global docker_prefix origin
%endif
%global docker_images %{?docker_registry}%{docker_namespace}/%{docker_prefix}-${component}:${version}

Name:           openshift
Version:        0.3.2
#Release:        1git%{shortcommit}%{?dist}
Release:        0%{?dist}
Summary:        Open Source Platform as a Service by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

BuildRequires:  systemd
BuildRequires:  golang >= 1.2-7


%description
%{summary}

%package master
Summary:        OpenShift Master
Requires:       openshift = %{version}-%{release}
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description master
%{summary}

%package node
Summary:        OpenShift Node
Requires:       openshift = %{version}-%{release}
Requires:       docker-io >= 1.3.2
Requires:       tuned-profiles-openshift-node
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description node
%{summary}

%package -n tuned-profiles-openshift-node
Summary:        Tuned profiles for OpenShift Node hosts
Requires:       tuned >= 2.3

%description -n tuned-profiles-openshift-node
%{summary}


%prep
%setup -q

%build

# Don't judge me for this ... it's so bad.
mkdir _build

# Horrid hack because golang loves to just bundle everything
pushd _build
    mkdir -p src/github.com/openshift
    ln -s $(dirs +1 -l) src/%{import_path}
popd


# Gaming the GOPATH to include the third party bundled libs at build
# time. This is bad and I feel bad.
mkdir _thirdpartyhacks
pushd _thirdpartyhacks
    ln -s \
        $(dirs +1 -l)/Godeps/_workspace/src/ \
            src
popd
export GOPATH=$(pwd)/_build:$(pwd)/_thirdpartyhacks:%{buildroot}%{gopath}:%{gopath}

# Default to building all of the components
for cmd in openshift
do
    #go build %{import_path}/cmd/${cmd}
    go build -ldflags "%{ldflags}" %{import_path}/cmd/${cmd}
done
# set the IMAGES
sed -i 's|IMAGES=.*|IMAGES=%{docker_images}|' rel-eng/openshift-{master,node}.sysconfig

%install

install -d %{buildroot}%{_bindir}
for bin in openshift
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 ${bin} %{buildroot}%{_bindir}/${bin}
done

install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-master.service
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-node.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-master
install -m 0644 rel-eng/openshift-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-node

mkdir -p %{buildroot}%{_sharedstatedir}/%{name}

ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/osc

mkdir -p %{buildroot}/usr/lib/tuned/openshift-node
install -m 0644 -t %{buildroot}/usr/lib/tuned/openshift-node tuned/openshift-node/tuned.conf

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/osc
%{_sharedstatedir}/openshift

%files master
%defattr(-,root,root,-)
%{_unitdir}/openshift-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-master

%post master
%systemd_post %{basename:openshift-master.service}

%preun master
%systemd_preun %{basename:openshift-master.service}

%postun master
%systemd_postun


%files node
%defattr(-,root,root,-)
%{_unitdir}/openshift-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-node

%post node
%systemd_post %{basename:openshift-node.service}

%preun node
%systemd_preun %{basename:openshift-node.service}

%postun node
%systemd_postun

%files -n tuned-profiles-openshift-node
%defattr(-,root,root,-)
%{_prefix}/lib/tuned/openshift-node

%post -n tuned-profiles-openshift-node
/usr/sbin/tuned-adm profile openshift-node > /dev/null 2>&1

%preun -n tuned-profiles-openshift-node
# reset the tuned profile to the recommended profile
# $1 = 0 when we're being removed > 0 during upgrades
if [ "$1" = 0 ]; then
  recommended=`/usr/sbin/tuned-adm recommend`
  /usr/sbin/tuned-adm profile $recommended > /dev/null 2>&1
fi


%changelog
* Mon Feb 23 2015 Scott Dodson <sdodson@redhat.com> 0.3.2-0
- Merge tag 'v0.3.2' (sdodson@redhat.com)
- Merge pull request #1087 from csrwng/bug_1190578
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1088 from bparees/update_sti
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1092 from smarterclayton/run_integration_in_serial
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1094 from bparees/clone_buildcfg_labels
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #971 from derekwaynecarr/acl_cache
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1096 from bparees/master
  (dmcphers+openshiftbot@redhat.com)
- Fix loop period to stop pegging CPU, add project-spawner (decarr@redhat.com)
- List projects enforces authorization (decarr@redhat.com)
- Merge pull request #1090 from csrwng/bug_1190576
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1043 from smarterclayton/add_router_command
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1051 from deads2k/deads-prevent-escalation
  (dmcphers+openshiftbot@redhat.com)
- fix formatting (bparees@redhat.com)
- Integration tests need to run in separate processes (ccoleman@redhat.com)
- Merge pull request #1085 from liggitt/upgrade_with_auth
  (dmcphers+openshiftbot@redhat.com)
- Bug 1190578 - Should prompt clear error when generate an application list
  code in a non-source code repository. (cewong@redhat.com)
- copy build labels from buildconfig (bparees@redhat.com)
- Update test cases and docs to use `openshift ex router` (ccoleman@redhat.com)
- prevent privilege escalation (deads@redhat.com)
- Add a router command to install / check the routers (ccoleman@redhat.com)
- UPSTREAM: Expose converting client.Config files to Data (ccoleman@redhat.com)
- Merge pull request #982 from csrwng/template_labels
  (dmcphers+openshiftbot@redhat.com)
- Remove cors headers from proxied connections (jliggitt@redhat.com)
- Strip auth headers before proxying, add auth headers on upgrade requests
  (jliggitt@redhat.com)
- Strip access_token param from requests in auth layer (jliggitt@redhat.com)
- Bug 1190576: Improve error message when trying to use non-existent reference
  (cewong@redhat.com)
- bump(github.com/openshift/source-to-
  image):ad5adc054311686baf316cd8bf91c4d42ae1bd4e (bparees@redhat.com)
- Merge pull request #1081 from csrwng/fix_json
  (dmcphers+openshiftbot@redhat.com)
- Improve help for creating / being added to new projects (jliggitt@redhat.com)
- Fix dangling commas in example json files (cewong@redhat.com)
- First hack at docker builds for OSE (bleanhar@redhat.com)
- Add labels to templates (cewong@redhat.com)
- Merge remote-tracking branch 'upstream/master' (bleanhar@redhat.com)
- Vagrantfile: default libvirt box now with actual openshift
  (lmeyer@redhat.com)
- Merge pull request #1076 from bparees/image_links
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1075 from deads2k/deads-add-namespaces
  (dmcphers+openshiftbot@redhat.com)
- document sti images (bparees@redhat.com)
- Merge pull request #1066 from liggitt/kubeconfig_inline_cert
  (dmcphers+openshiftbot@redhat.com)
- Update docs (jliggitt@redhat.com)
- add namespaces to authorization rules (deads@redhat.com)
- Inline bootstrapped certs in client .kubeconfig files (jliggitt@redhat.com)
- UPSTREAM: Let .kubeconfig populate ca/cert/key data and basic-auth
  username/password in client configs (jliggitt@redhat.com)
- Merge pull request #998 from deads2k/deads-handle-non-resource-urls
  (dmcphers+openshiftbot@redhat.com)
- authorize non-resource urls (deads@redhat.com)
- Merge pull request #1068 from liggitt/kubeconfig_check
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1063 from bparees/remove_local
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #996 from pweil-/route-validate-name
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1072 from bparees/docker_regex
  (dmcphers+openshiftbot@redhat.com)
- use multiline in regex matching (bparees@redhat.com)
- Merge pull request #1071 from jwforres/service_selector_bug
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1024 from mfojtik/sti_rebase
  (dmcphers+openshiftbot@redhat.com)
- Handle empty label selectors in web console (jforrest@redhat.com)
- Remove dead links from cli doc (kargakis@users.noreply.github.com)
- Capture errors from BuildParameters conversion (mfojtik@redhat.com)
- Fix integration (mfojtik@redhat.com)
- Move ContextDir under BuildSource (mfojtik@redhat.com)
- Add support for ContextDir in STI build (mfojtik@redhat.com)
- Fix Origin code to incorporate changes in STI (mfojtik@redhat.com)
- bump(github.com/openshift/source-to-image):
  1338bff33b5c46acc02840f88a9b576a1b1fa404 (mfojtik@redhat.com)
- bump(github.com/fsouza/go-
  dockerclient):e1e2cc5b83662b894c6871db875c37eb3725a045 (mfojtik@redhat.com)
- UPSTREAM: Surface load errors when reading .kubeconfig files
  (jliggitt@redhat.com)
- Merge pull request #1054 from smarterclayton/tag_built_images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1062 from mnagy/fix_bash_substitution
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1013 from deads2k/deads-respect-namesapce
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1028 from jwforres/overview_edge_cases
  (dmcphers+openshiftbot@redhat.com)
- Handle k8s edge cases in the web console overview (jforrest@redhat.com)
- Merge pull request #1059 from ppalaga/150218-etcd-clone-or-fetch
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1047 from derekwaynecarr/turn_on_quota_controller
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1064 from deads2k/deads-reduce-logging-verbosity-somehow-
  this-will-be-wrong (dmcphers+openshiftbot@redhat.com)
- Give our local built docker images unique ids, and push that id
  (ccoleman@redhat.com)
- UPSTREAM: move setSelfLink logging to v(5) (deads@redhat.com)
- make useLocalImages the default and remove configurability
  (bparees@redhat.com)
- Fix build-in-docker.sh, use $(), not ${} (nagy.martin@gmail.com)
- validate route name and host (pweil@redhat.com)
- make sure rolebinding names are unique (deads@redhat.com)
- Use git fetch && reset if the given repo was cloned already
  (ppalaga@redhat.com)
- Replacing old openshift/nodejs-0-10-centos with new
  openshift/nodejs-010-centos7 (j.hadvig@gmail.com)
- Merge pull request #1050 from ironcladlou/deploy-error-refactor
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1040 from smarterclayton/add_template_api
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1053 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1052 from smarterclayton/cleanup_factories
  (dmcphers+openshiftbot@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Merge pull request #1038 from derekwaynecarr/project_is_namespace
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1049 from deads2k/deads-fix-namespace-requirement
  (dmcphers+openshiftbot@redhat.com)
- Project is Kubernetes Namespace (decarr@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Refactor deploy package for error handling support (ironcladlou@gmail.com)
- ignore selflinking error (deads@redhat.com)
- Merge pull request #1026 from deads2k/deads-tidy-up-policy.md
  (dmcphers+openshiftbot@redhat.com)
- Implement a Template REST endpoint using the generic store
  (ccoleman@redhat.com)
- sync policy doc to reality (deads@redhat.com)
- Merge pull request #1046 from ironcladlou/release-tar-naming
  (dmcphers+openshiftbot@redhat.com)
- Turn on resource quota manager to collect usage stats (decarr@redhat.com)
- Improve release tar naming (ironcladlou@gmail.com)
- Explicit build (ccoleman@redhat.com)
- Merge pull request #897 from ironcladlou/cross-compile-image-binaries
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1036 from deads2k/deads-use-kube-attributes
  (dmcphers+openshiftbot@redhat.com)
- Platform independent image builds (ironcladlou@gmail.com)
- bump(github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission:c977a4586
  42b4dbd8c3ad9cfc9eecafc85fb6183) (decarr@redhat.com)
- Add compile dependency for Kubernetes admission control plugins
  (decarr@redhat.com)
- switch to kubernetes authorization info (deads@redhat.com)
- UPSTREAM: expose info resolver (deads@redhat.com)
- Remove GOPATH from build-in-docker.sh script (mfojtik@redhat.com)
- Unify all client Factories into one location (ccoleman@redhat.com)
- Merge pull request #930 from sdodson/set-images-format
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1017 from kargakis/doc (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1027 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1030 from liggitt/request_context_map
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1015 from kargakis/minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1031 from jwforres/add_fixtures_edge_cases
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1032 from mfojtik/install-registry
  (dmcphers+openshiftbot@redhat.com)
- Move coverage output processing to the end (dmcphers@redhat.com)
- Merge pull request #1039 from pweil-/router-websockets
  (dmcphers+openshiftbot@redhat.com)
- Switch to using request context mapper (jliggitt@redhat.com)
- Merge pull request #999 from deads2k/deads-create-project
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1033 from liggitt/htmlmin
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1029 from jwforres/fix_namespaced_urls
  (dmcphers+openshiftbot@redhat.com)
- router websocket support (pweil@redhat.com)
- Merge pull request #1008 from nak3/Added-missing-Requires
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1023 from bparees/custom_builder
  (dmcphers+openshiftbot@redhat.com)
- add new-project command (deads@redhat.com)
- Add fixtures to test edge cases in the web console (jforrest@redhat.com)
- Fix htmlmin linebreak issue (jliggitt@redhat.com)
- Provide useful message when CERT_DIR is not set in install-registry.sh
  (mfojtik@redhat.com)
- Switch namespaced URL paths from ns/ to namespaces/ (jforrest@redhat.com)
- Bump specfile 0.3.1 (sdodson@redhat.com)
- Merge tag 'v0.3.1' (sdodson@redhat.com)
- Merge pull request #1018 from rhcarvalho/typokiller
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Distinguish between NamespaceAll and NamespaceDefault
  (ccoleman@redhat.com)
- use origin-base for base image (bparees@redhat.com)
- e2e without root fails because certs can't be viewed by wait_for_url
  (ccoleman@redhat.com)
- Tolerate Docker not being present when using all-in-one (ccoleman@redhat.com)
- Update registries to remove async channel (ccoleman@redhat.com)
- Compile time changes (ccoleman@redhat.com)
- UPSTREAM: Fix cross-namespace queries (ccoleman@redhat.com)
- UPSTREAM: Allow kubelet to run without Docker (ccoleman@redhat.com)
- UPSTREAM: Allow SetList to work against api.List (ccoleman@redhat.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Add 'release' field to raven-go (ccoleman@redhat.com)
- UPSTREAM: special command "help" must be aware of context
  (contact@fabianofranz.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):c977a458642b4dbd8c3ad9cfc9eec
  afc85fb6183 (ccoleman@redhat.com)
- Stop referencing kubecfg now that it is deleted upstream
  (ccoleman@redhat.com)
- Fix several typos (rhcarvalho@gmail.com)
- Note on using docker with --insecure-registry
  (kargakis@users.noreply.github.com)
- Use p12-encoded certs on OS X (jliggitt@redhat.com)
- Initialize image by using struct literal (kargakis@users.noreply.github.com)
- Merge pull request #800 from deads2k/deads-the-rest-of-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1009 from deads2k/deads-upstream-kubeconfig-merge
  (dmcphers+openshiftbot@redhat.com)
- *AccessReviews (deads@redhat.com)
- pass authorizer/attributebuilder pair into master (deads@redhat.com)
- Merge pull request #990 from kargakis/prof-comment
  (dmcphers+openshiftbot@redhat.com)
- Revert "Drop the version variable from --images for now" (sdodson@redhat.com)
- Update the custom tagger and builder to provide OpenShift ldflags
  (sdodson@redhat.com)
- Merge pull request #1004 from liggitt/logout_uri
  (dmcphers+openshiftbot@redhat.com)
- put templates in correct dir for install registry script (bparees@redhat.com)
- UPSTREAM: properly handle mergo map versus value rules: 4416
  (deads@redhat.com)
- Add missing Requires (nakayamakenjiro@gmail.com)
- Explain why Stop is called like that (michaliskargakis@gmail.com)
- Merge pull request #993 from kargakis/validate
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1002 from bparees/run_in_container
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1003 from deads2k/deads-tolerate-missing-role
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1000 from liggitt/qualify_certs
  (dmcphers+openshiftbot@redhat.com)
- Allow setting final logout uri (jliggitt@redhat.com)
- Merge pull request #1001 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- tolerate missing roles (deads@redhat.com)
- Merge pull request #991 from deads2k/deads-make-user-tilde-work
  (dmcphers+openshiftbot@redhat.com)
- update in-container steps to setup registry properly (bparees@redhat.com)
- Making race an option for sh scripts (dmcphers@redhat.com)
- Merge pull request #995 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Fixing test coverage reporting (dmcphers@redhat.com)
- Generate client configs for nodes, provider-qualify and add groups to certs
  (jliggitt@redhat.com)
- Merge pull request #994 from jwforres/use_label_selector
  (dmcphers+openshiftbot@redhat.com)
- add resourceName to policy (deads@redhat.com)
- Merge pull request #989 from soltysh/issue865
  (dmcphers+openshiftbot@redhat.com)
- Use LabelSelector in overview to associate pods to a service
  (jforrest@redhat.com)
- Issue 865: Fixed build integration tests. (maszulik@redhat.com)
- Validate image repository (michaliskargakis@gmail.com)
- Merge pull request #985 from liggitt/prevent_panic
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #986 from jwforres/namespace_in_path
  (dmcphers+openshiftbot@redhat.com)
- Fix off-by-1 error (jliggitt@redhat.com)
- Support namespace in path for requesting k8s api from web console
  (jforrest@redhat.com)
- Bug 1191824 - Fixing typo (bleanhar@redhat.com)
- Merge pull request #978 from mnagy/fix_logging
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #974 from soltysh/copyright_removal
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #984 from dobbymoodge/service_order_fixup
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #954 from jwforres/quota
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #983 from liggitt/auth_config
  (dmcphers+openshiftbot@redhat.com)
- Require network when starting openshift-master (jolamb@redhat.com)
- Add project settings page and show quota and limit ranges for the project
  (jforrest@redhat.com)
- Attempt to manipulate images path conditionally (sdodson@redhat.com)
- Merge pull request #981 from fabianofranz/osc_login_further
  (dmcphers+openshiftbot@redhat.com)
- Change ORIGIN_OAUTH_* env vars to OPENSHIFT_OAUTH_* (jliggitt@redhat.com)
- Merge pull request #980 from liggitt/unify_user
  (dmcphers+openshiftbot@redhat.com)
- Bug 1191354 - must save username to .kubeconfig correctly
  (contact@fabianofranz.com)
- Merge pull request #979 from deads2k/deads-add-policy-watches
  (dmcphers+openshiftbot@redhat.com)
- Use kubernetes user.Info interface (jliggitt@redhat.com)
- Merge pull request #977 from brenton/master
  (dmcphers+openshiftbot@redhat.com)
- add policy watches (deads@redhat.com)
- Merge pull request #970 from jwforres/console_v1beta3
  (dmcphers+openshiftbot@redhat.com)
- Update the web console to request k8s api on v1beta3 (jforrest@redhat.com)
- Use Warningf instead of Warning so formatting works (nagy.martin@gmail.com)
- Bug 1190095 - useless entry "nameserver <nil>" is in docker container
  resolv.conf (bleanhar@redhat.com)
- deploymentconfigs permission typo (deads@redhat.com)
- Removed all Google copyrights leftovers (maszulik@redhat.com)
- Merge pull request #962 from goldmann/docker_vagrant
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #885 from deads2k/deads-tighten-bootstrap-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #966 from mnagy/chown_docker_build_output
  (dmcphers+openshiftbot@redhat.com)
- e2e formatting (deads@redhat.com)
- tighten bootstrap policy (deads@redhat.com)
- Default session secret to unknowable value (jliggitt@redhat.com)
- Make token lifetimes configurable (jliggitt@redhat.com)
- Allow denying a prompted OAuth grant (jliggitt@redhat.com)
- Make user header configurable (jliggitt@redhat.com)
- Merge pull request #953 from deads2k/deads-smarter-kubeconfig-merge
  (dmcphers+openshiftbot@redhat.com)
- remove deny and negations (deads@redhat.com)
- Use chown for build-in-docker.sh output binaries (nagy.martin@gmail.com)
- Make sure vagrant development environment works well with new docker-io
  (marek.goldmann@gmail.com)
- Merge remote-tracking branch 'origin-sdodson/set-images-format'
  (sdodson@redhat.com)
- Fix .el7dist string (sdodson@redhat.com)
- move cleanup to its own section (bparees@redhat.com)
- Merge pull request #951 from pweil-/router-sample-app
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #957 from liggitt/auth_tweaks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #955 from liggitt/browser_cert_prompt
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #956 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Comment on redirecting to login on 0 codes (jliggitt@redhat.com)
- Add better instructions around using vagrant (dmcphers@redhat.com)
- Prevent browsers from prompting to send bogus client certs
  (jliggitt@redhat.com)
- Merge pull request #952 from liggitt/mime (dmcphers+openshiftbot@redhat.com)
- add edge terminated route to sample app (pweil@redhat.com)
- make more intelligent kubeconfig merge (deads@redhat.com)
- Merge pull request #950 from liggitt/example
  (dmcphers+openshiftbot@redhat.com)
- Register used mime types (jliggitt@redhat.com)
- use tags from imagerepos when constructing new builds (bparees@redhat.com)
- Update example project json (jliggitt@redhat.com)
- Merge pull request #946 from mnagy/test_selinux_labels
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #921 from kargakis/various-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #945 from kargakis/fix (dmcphers+openshiftbot@redhat.com)
- Merge pull request #943 from smarterclayton/generate_shorter_tags
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #898 from ironcladlou/pod-deployment-annotations
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #886 from pweil-/make-skip-build
  (dmcphers+openshiftbot@redhat.com)
- Check SELinux labels when building in docker (nagy.martin@gmail.com)
- Merge pull request #937 from liggitt/html_newlines
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #939 from deads2k/deads-fix-remove-group
  (dmcphers+openshiftbot@redhat.com)
- Various idiomatic fixes throughout the repo (michaliskargakis@gmail.com)
- Check for unspecified port once (michaliskargakis@gmail.com)
- Merge remote-tracking branch 'origin-sdodson/set-images-format'
  (sdodson@redhat.com)
- Merge tag 'v0.3' (sdodson@redhat.com)
- Generate shorter tags when building release tars (ccoleman@redhat.com)
- Update the custom tagger and builder to provide OpenShift ldflags
  (sdodson@redhat.com)
- Merge pull request #940 from smarterclayton/warn_on_empty_repo
  (dmcphers+openshiftbot@redhat.com)
- Update start help (ccoleman@redhat.com)
- Tweak the flag for --images to indicate applies to master and node
  (ccoleman@redhat.com)
- Merge pull request #938 from csrwng/build_logging
  (dmcphers+openshiftbot@redhat.com)
- Improve the default osc help page (ccoleman@redhat.com)
- Display more information when an app is automatically created
  (ccoleman@redhat.com)
- make remove-group from role check all bindings (deads@redhat.com)
- Improve error handling and logging for builds (cewong@redhat.com)
- UPSTREAM: Use name from server when displaying create/update
  (ccoleman@redhat.com)
- Drop the version variable from --images for now (sdodson@redhat.com)
- Attempt to manipulate images path conditionally (sdodson@redhat.com)
- Merge pull request #922 from sdodson/ootb-configs
  (dmcphers+openshiftbot@redhat.com)
- Correlate deployed pods with their deployments (ironcladlou@gmail.com)
- Remove comments from minified HTML (jliggitt@redhat.com)
- Preserve newlines in html (jliggitt@redhat.com)
- Merge pull request #924 from ncdc/proxy-upgrade
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #934 from csrwng/handle_multiple_matches
  (dmcphers+openshiftbot@redhat.com)
- Add upgrade-aware HTTP proxy (agoldste@redhat.com)
- Merge pull request #932 from liggitt/user_init
  (dmcphers+openshiftbot@redhat.com)
- Handle multiple builder matches in generate command (cewong@redhat.com)
- Merge pull request #931 from liggitt/osc_oauth_describers
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #904 from smarterclayton/enable_new_app
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #929 from fabianofranz/fix_help_usability
  (dmcphers+openshiftbot@redhat.com)
- OAuth api printers (jliggitt@redhat.com)
- Set user fullname correctly, initialize identity mappings fully
  (jliggitt@redhat.com)
- Merge pull request #923 from liggitt/delete_token_on_logout
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #928 from sdodson/fix-tuned-profile
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #830 from fabianofranz/osc_login
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #927 from bparees/fix_scripts_url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #918 from jwforres/bug_1189390_empty_project_selector
  (dmcphers+openshiftbot@redhat.com)
- Set KUBECONFIG path for openshift-node (sdodson@redhat.com)
- Sort ports for service generation and select first port (cewong@redhat.com)
- Enable `osc new-app` and simplify some of the rough edges for launch
  (ccoleman@redhat.com)
- Command for displaying global options is "osc options" in help
  (contact@fabianofranz.com)
- Delete token on logout (jliggitt@redhat.com)
- Merge pull request #926 from jwforres/start_node_kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- Syntax consistency between help templates (contact@fabianofranz.com)
- Merge pull request #916 from liggitt/localStorage
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #858 from ironcladlou/image-trigger-matching-fix
  (dmcphers+openshiftbot@redhat.com)
- Fixes base command reference (osc || openshift cli) in help
  (contact@fabianofranz.com)
- Merge pull request #784 from mfojtik/docs_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #910 from ironcladlou/enhance-deployment-describer
  (dmcphers+openshiftbot@redhat.com)
- RPMs: Fix upgrades for the tuned profile (sdodson@redhat.com)
- Better footer messages in help (contact@fabianofranz.com)
- Fix help line breaks (contact@fabianofranz.com)
- Remove the usage from the list of available commands in help
  (contact@fabianofranz.com)
- modify .kubeconfig (deads@redhat.com)
- Basic structure for osc login (contact@fabianofranz.com)
- Merge pull request #911 from ironcladlou/pod-diff-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #900 from liggitt/expire_session
  (dmcphers+openshiftbot@redhat.com)
- fix scripts http url to https (bparees@redhat.com)
- Start node with --kubeconfig should use master host from config file
  (jforrest@redhat.com)
- Bug 1189390 - fix project selector so it gets re-rendered after project list
  changes (jforrest@redhat.com)
- Switch to localStorage (jliggitt@redhat.com)
- Fix image change trigger imageRepo matching (ironcladlou@gmail.com)
- Show latest deployment in deploy config describe (ironcladlou@gmail.com)
- Merge pull request #901 from liggitt/tweak_projects_css
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #913 from ncdc/warn_on_chcon_only
  (dmcphers+openshiftbot@redhat.com)
- Update documentation to use 'osc create' consistently (mfojtik@redhat.com)
- Tighten header styles inside tiles (jliggitt@redhat.com)
- Only warn when chcon fails (ccoleman@redhat.com)
- Restore logic to disregard resources in pod diff (ironcladlou@gmail.com)
- Merge pull request #890 from fabianofranz/fix_more_help_issues
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Register nodes that already exist statically (ccoleman@redhat.com)
- Only allow session auth to be used for a single auth flow
  (jliggitt@redhat.com)
- Merge pull request #895 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #892 from sdodson/gitignore-pyc
  (dmcphers+openshiftbot@redhat.com)
- Remove aliases and options command from cli template
  (contact@fabianofranz.com)
- Merge pull request #891 from smarterclayton/add_user_agent
  (dmcphers+openshiftbot@redhat.com)
- make ovs-simple from openshift-sdn the default networking solution for
  vagrant (rchopra@redhat.com)
- Merge pull request #894 from mrunalp/vagrant_certs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #872 from bparees/remove_guestbook
  (dmcphers+openshiftbot@redhat.com)
- Use certs in vagrant mutli node environment. (mrunalp@gmail.com)
- Merge pull request #835 from pweil-/reencrypt-validation
  (dmcphers+openshiftbot@redhat.com)
- Nuke the default usage section from client templates
  (contact@fabianofranz.com)
- Introduces "osc options" to list global options (contact@fabianofranz.com)
- Custom template for cli and osc, hide global options
  (contact@fabianofranz.com)
- Merge pull request #888 from bparees/update_readme
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- fix service select for frontend (bparees@redhat.com)
- Merge pull request #855 from liggitt/login (dmcphers+openshiftbot@redhat.com)
- Ignore .pyc files (sdodson@redhat.com)
- OpenShift should set a client UserAgent on all calls (ccoleman@redhat.com)
- Merge pull request #884 from bparees/imagerepo_template
  (dmcphers+openshiftbot@redhat.com)
- Add OAuth login to console (jliggitt@redhat.com)
- Set a default user agent on all client.Client calls (ccoleman@redhat.com)
- remove use of osc namespace command (bparees@redhat.com)
- create template file for image repos (bparees@redhat.com)
- Merge pull request #883 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- add skip build flag support for test target (pweil@redhat.com)
- Merge pull request #881 from bparees/dockerrepository
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #794 from csrwng/experimental_appgen
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #882 from smarterclayton/increase_timeout_on_test_cmd
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #880 from bparees/update_readme
  (dmcphers+openshiftbot@redhat.com)
- Remove references to openshift/kubernetes (dmcphers@redhat.com)
- use Status.DockerImageRepository instead of DockerImageRepository
  (bparees@redhat.com)
- Merge pull request #878 from jwhonce/docs (dmcphers+openshiftbot@redhat.com)
- Merge pull request #877 from jwforres/fix_console_ie
  (dmcphers+openshiftbot@redhat.com)
- Reorganize command line code and move app-gen under experimental
  (cewong@redhat.com)
- Slightly increase the wait for hack/test-cmd.sh (ccoleman@redhat.com)
- Merge pull request #868 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #857 from ncdc/chcon-volumes-dir
  (dmcphers+openshiftbot@redhat.com)
- WIP - osc new-app with argument inference (ccoleman@redhat.com)
- [WIP] Simple generation flow for an application based on source
  (cewong@redhat.com)
- Simple source->build->deploy generation (ccoleman@redhat.com)
- Merge pull request #876 from smarterclayton/typo_in_master
  (dmcphers+openshiftbot@redhat.com)
- remove guestbook link (bparees@redhat.com)
- Merge pull request #864 from smarterclayton/release_push_guidelines
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #863 from smarterclayton/fix_end_to_end
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #853 from deads2k/deads-cli
  (dmcphers+openshiftbot@redhat.com)
- Add missing step when testing k8s rebase (jhonce@redhat.com)
- Merge pull request #869 from smarterclayton/restore_original_cors
  (dmcphers+openshiftbot@redhat.com)
- Bug 1188933 - Missing es5-dom-shim causes CustomEvent polyfill to fail in IE
  (jforrest@redhat.com)
- Resource round tripping has been fixed and new fields added to pod
  (ccoleman@redhat.com)
- Master takes IP instead of string (ccoleman@redhat.com)
- Use RESTMapper scopes for registering resources (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- UPSTREAM: spf13/cobra help display separate groups of flags
  (contact@fabianofranz.com)
- UPSTREAM: Add 'release' field to raven-go (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):72ad4f12bd7408a6f75e6a0bf37b3
  440e165bdf4 (ccoleman@redhat.com)
- hack/end-to-end should terminate jobs (ccoleman@redhat.com)
- Set volume dir SELinux context if possible (agoldste@redhat.com)
- Typo on master.go around pullIfNotPresent (ccoleman@redhat.com)
- Update HACKING.md with the release push instructions (ccoleman@redhat.com)
- Bash is the worst programming language ever invented (ccoleman@redhat.com)
- Restore CORS outside of go-restful (ccoleman@redhat.com)
- Merge pull request #854 from fabianofranz/fix_help_issues
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #873 from bparees/simplify_readme
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #860 from bparees/check_dockerfile
  (dmcphers+openshiftbot@redhat.com)
- use start-build instead of curl (bparees@redhat.com)
- Merge pull request #870 from mfojtik/context_dir
  (dmcphers+openshiftbot@redhat.com)
- Add CONTEXT_DIR support for sti-image-builder image (mfojtik@redhat.com)
- Merge pull request #845 from mnagy/add_upstream_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #866 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Generate openshift service on provision (dmcphers@redhat.com)
- check for error on missing docker file (bparees@redhat.com)
- Adding tests for help consistency to test-cmd.sh (contact@fabianofranz.com)
- UPSTREAM: special command "help" must be aware of context
  (contact@fabianofranz.com)
- experimental policy cli (deads@redhat.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- UPSTREAM: Use new resource builder in kubectl update #3805
  (nagy.martin@gmail.com)
- add missing reencrypt validations (pweil@redhat.com)

* Wed Feb 04 2015 Scott Dodson <sdodson@redhat.com> 0.2.2-0
- Merge tag 'v0.2.2' (sdodson@redhat.com)
- Merge pull request #861 from smarterclayton/version_images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #851 from bparees/remove_guestbook
  (dmcphers+openshiftbot@redhat.com)
- Expose two new flags on master --images and --latest-images
  (ccoleman@redhat.com)
- Create an openshift/origin-pod image (ccoleman@redhat.com)
- Merge pull request #840 from mfojtik/gofmt (dmcphers+openshiftbot@redhat.com)
- Merge pull request #850 from sg00dwin/refactor-css-and-filter
  (dmcphers+openshiftbot@redhat.com)
- remove guestbook example (bparees@redhat.com)
- Merge pull request #847 from bparees/build_description
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #849 from phemmer/kubernetes_capabilities
  (dmcphers+openshiftbot@redhat.com)
- Refactor css, variablize more values and restructure label filter markup
  (sgoodwin@redhat.com)
- Merge pull request #841 from mfojtik/build_in_docker
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #846 from ironcladlou/deployer-race-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #839 from smarterclayton/add_auth_proxy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #842 from deads2k/deads-registry-errors
  (dmcphers+openshiftbot@redhat.com)
- better description of output to field (bparees@redhat.com)
- move kubernetes capabilities to server start (patrick.hemmer@gmail.com)
- Add --check option to run golint and gofmt in ./hack/build-in-docker.sh
  (mfojtik@redhat.com)
- Add retry logic to recreate deployment strategy (ironcladlou@gmail.com)
- Very simple authorizing proxy for Kubernetes (ccoleman@redhat.com)
- Unify authorization logic into a more structured form (ccoleman@redhat.com)
- User registry should transform server errors (ccoleman@redhat.com)
- UPSTREAM: Handle case insensitive node names and squash logging
  (ccoleman@redhat.com)
- remove unnecessary rest methods (deads@redhat.com)
- UPSTREAM: add flag to manage $KUBECONFIG files: #4053, bugzilla 1188208
  (deads@redhat.com)
- Add ./hack/build-in-docker.sh script (mfojtik@redhat.com)
- Fix Go version checking in verify-gofmt (mfojtik@redhat.com)
- Merge pull request #833 from csrwng/update_docker_pkgs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #831 from
  derekwaynecarr/sample_projects_have_different_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #808 from sdodson/haproxy-1510
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #837 from liggitt/login (dmcphers+openshiftbot@redhat.com)
- Merge pull request #819 from sosiouxme/201502-sample-app-docs
  (dmcphers+openshiftbot@redhat.com)
- Use $http (jliggitt@redhat.com)
- Merge pull request #789 from fabianofranz/cobra_local_global_flags_separation
  (dmcphers+openshiftbot@redhat.com)
- sample-app docs: update for TLS, namespace, context (lmeyer@redhat.com)
- Merge pull request #801 from jwforres/filter_widget
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #834 from bparees/imagerepo_labels
  (dmcphers+openshiftbot@redhat.com)
- Removed "Additional Help Topics" section from help template
  (contact@fabianofranz.com)
- Use our own templates to cli help and usage (contact@fabianofranz.com)
- UPSTREAM: spf13/cobra help display separate groups of flags
  (contact@fabianofranz.com)
- Merge pull request #825 from smarterclayton/update_kubectl
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #798 from lhuard1A/fix_vagrant_net
  (dmcphers+openshiftbot@redhat.com)
- Label filtering widget in the web console, styles by @sg00dwin
  (jforrest@redhat.com)
- Merge pull request #829 from deads2k/deads-ignore-local-kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- remove unnecessary labels from sample imagerepos (bparees@redhat.com)
- Add shortcuts for build configs and deployment configs (ccoleman@redhat.com)
- Remove kubecfg, expose kubectl in its place (ccoleman@redhat.com)
- bump(github.com/docker/docker):211513156dc1ace48e630b4bf4ea0fcfdc8d9abf
  (cewong@redhat.com)
- Update project display name for sample app (decarr@redhat.com)
- UPSTREAM: typos (contact@fabianofranz.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #812 from liggitt/display_token
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #675 from deads2k/deads-openshift-authorization-impl
  (dmcphers+openshiftbot@redhat.com)
- ignore local .kubeconfig (deads@redhat.com)
- Improve /oauth/token/request page (jliggitt@redhat.com)
- Merge pull request #804 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Compile haproxy with generic CPU instructions (sdodson@redhat.com)
- Fix multi minion setup (dmcphers@redhat.com)
- policy authorizer (deads@redhat.com)
- policy client (deads@redhat.com)
- policy storage (deads@redhat.com)
- policy types (deads@redhat.com)
- Fix the Vagrant network setup (lhuard@amadeus.com)
- Remove empty log files and use a slightly different process kill method
  (ccoleman@redhat.com)
- Refactor to match upstream master changes (ccoleman@redhat.com)
- Better debug output in deployment_config_controller (ccoleman@redhat.com)
- Adapt to upstream changes for cache.Store (ccoleman@redhat.com)
- UPSTREAM: Relax validation around annotations (ccoleman@redhat.com)
- UPSTREAM: Support GetByKey so EventStore can de-dup (ccoleman@redhat.com)
- UPSTREAM: Add 'release' field to raven-go (ccoleman@redhat.com)
- UPSTREAM: Allow namespace short to be set (ccoleman@redhat.com)
- UPSTREAM: api registration right on mux makes it invisible to container
  (contact@fabianofranz.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):e335e2d3e26a9a58d3b189ccf41ce
  b3770d1bfa9 (ccoleman@redhat.com)
- Escape helper echo in rebase-kube (ccoleman@redhat.com)
- only kill and remove k8s managed containers (bparees@redhat.com)
- Gofmt whitespace flaw (ccoleman@redhat.com)
- Helper function for rebase output (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- Merge pull request #816 from smarterclayton/properly_vesrion
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #797 from smarterclayton/wire_up_api_to_node
  (dmcphers+openshiftbot@redhat.com)
- Godeps: update tags to be accurate (ccoleman@redhat.com)
- Properly version Kubernetes and OpenShift binaries (ccoleman@redhat.com)
- Merge pull request #815 from smarterclayton/travis_flake
  (dmcphers+openshiftbot@redhat.com)
- Connect the node to the master via and a built in client
  (ccoleman@redhat.com)
- Rebase fixes (ccoleman@redhat.com)
- Fix flaky travis by limiting parallel builds (ccoleman@redhat.com)
- UPSTREAM: api registration right on mux makes it invisible to container
  (contact@fabianofranz.com)
- UPSTREAM: Allow namespace short to be set (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):e0acd75629ec29bde764bcde29367
  146ae8b389b (jhonce@redhat.com)
- Merge pull request #811 from smarterclayton/pkill_must_be_sudo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #786 from ironcladlou/rollback-cli
  (dmcphers+openshiftbot@redhat.com)
- pkill on test-end-to-end.sh must be sudo (ccoleman@redhat.com)
- Merge pull request #810 from liggitt/oauth_token_user_uid
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #809 from liggitt/relative_kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- Set user UID in session (jliggitt@redhat.com)
- Implement deployment rollback CLI support (ironcladlou@gmail.com)
- Merge pull request #792 from liggitt/oauth_prefix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #807 from danmcp/allow_root
  (dmcphers+openshiftbot@redhat.com)
- Make file references relative in generated .kubeconfig files
  (jliggitt@redhat.com)
- Add ability to retrieve userIdentityMapping (jliggitt@redhat.com)
- Add "Oauth" prefix to oauth types (jliggitt@redhat.com)
- Register internal OAuth API objects correctly (jliggitt@redhat.com)
- Merge pull request #788 from sosiouxme/201501-vagrant-providers
  (dmcphers+openshiftbot@redhat.com)
- Allow bower to run as root (dmcphers@redhat.com)
- Vagrantfile: improve providers and usability/readability (lmeyer@redhat.com)
- UPSTREAM: resolve relative paths in .kubeconfig (deads@redhat.com)
- Pin haproxy to 1.5.10 (sdodson@redhat.com)
- Fix the Vagrant network setup (lhuard@amadeus.com)

* Fri Jan 30 2015 Scott Dodson <sdodson@redhat.com> 0.2.1-4
- new package built with tito

* Mon Jan 26 2015 Scott Dodson <sdodson@redhat.com> 0.2-3
- Update to 21fb40637c4e3507cca1fcab6c4d56b06950a149
- Split packaging of openshift-master and openshift-node

* Mon Jan 19 2015 Scott Dodson <sdodson@redhat.com> 0.2-2
- new package built with tito

* Fri Jan 09 2015 Adam Miller <admiller@redhat.com> - 0.2-2
- Add symlink for osc command line tooling (merged in from jhonce@redhat.com)

* Wed Jan 07 2015 Adam Miller <admiller@redhat.com> - 0.2-1
- Update to latest upstream release
- Restructured some of the golang deps  build setup for restructuring done
  upstream

* Thu Oct 23 2014 Adam Miller <admiller@redhat.com> - 0-0.0.9.git562842e
- Add new patches from jhonce for systemd units

* Mon Oct 20 2014 Adam Miller <admiller@redhat.com> - 0-0.0.8.git562842e
- Update to latest master snapshot

* Wed Oct 15 2014 Adam Miller <admiller@redhat.com> - 0-0.0.7.git7872f0f
- Update to latest master snapshot

* Fri Oct 03 2014 Adam Miller <admiller@redhat.com> - 0-0.0.6.gite4d4ecf
- Update to latest Alpha nightly build tag 20141003

* Wed Oct 01 2014 Adam Miller <admiller@redhat.com> - 0-0.0.5.git6d9f1a9
- Switch to consistent naming, patch by jhonce

* Tue Sep 30 2014 Adam Miller <admiller@redhat.com> - 0-0.0.4.git6d9f1a9
- Add systemd and sysconfig entries from jhonce

* Tue Sep 23 2014 Adam Miller <admiller@redhat.com> - 0-0.0.3.git6d9f1a9
- Update to latest upstream.

* Mon Sep 15 2014 Adam Miller <admiller@redhat.com> - 0-0.0.2.git2647df5
- Update to latest upstream.

* Thu Aug 14 2014 Adam Miller <admiller@redhat.com> - 0-0.0.1.gitc3839b8
- First package
