#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%{!?commit:
%global commit 8aabf9c0b23bb02861ca9b2c691569cd79e1d003
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# OpenShift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 4+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.4.2.5-39-g8aabf9c -X github.com/openshift/origin/pkg/version.commitFromGit 8aabf9c -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 8d94c43 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.13.1-dev-641-gf057a25
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
Version:        0.4.3.0
Release:        1%{?dist}
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
Requires:       %{name} = %{version}-%{release}
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description master
%{summary}

%package node
Summary:        OpenShift Node
Requires:       %{name} = %{version}-%{release}
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
Requires:       %{name} = %{version}-%{release}

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

install -d -m 0755 %{buildroot}/etc/%{name}
install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-master.service
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-node.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-master
install -m 0644 rel-eng/openshift-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-node

mkdir -p %{buildroot}%{_sharedstatedir}/%{name}

ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/osc

mkdir -p %{buildroot}%{_libdir}/tuned/openshift-node
install -m 0644 -t %{buildroot}%{_libdir}/tuned/openshift-node tuned/openshift-node/tuned.conf

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/osc
%{_sharedstatedir}/%{name}
/etc/%{name}

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
%{_libdir}/tuned/openshift-node

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
* Thu Apr 09 2015 Scott Dodson <sdodson@redhat.com> 0.4.3.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1631 from liggitt/openid
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1632 from smarterclayton/etcd2
  (dmcphers+openshiftbot@redhat.com)
- Add OpenID identity provider (jliggitt@redhat.com)
- Refactor OAuth config (jliggitt@redhat.com)
- Widen the OpenShift CIDR to 172.30.0.0/16 (ccoleman@redhat.com)
- Add running builds and deployments to status output (ccoleman@redhat.com)
- Separate project status loggers (ccoleman@redhat.com)
- Add a generic injectable test client (ccoleman@redhat.com)
- UPSTREAM: Support a pluggable fake (ccoleman@redhat.com)
- Upgrade from etcd 0.4.6 -> 2.0.9 (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):02697ca725e5c790cc1f9d0918ff22fad84cb4c5
  (ccoleman@redhat.com)
- Godeps.json formatting is wrong (ccoleman@redhat.com)
- Merge pull request #1654 from ncdc/fix-flakey-exec
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1653 from deads2k/deads-stop-swallowing-error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1640 from fabianofranz/bump_cobra
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1599 from deads2k/deads-allow-project-to-change-context
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1472 from pravisankar/v2registry-changes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1638 from csrwng/web_console_imgstreams
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- UPSTREAM: Don't use command pipes for exec/port forward (agoldste@redhat.com)
- stop swallowing errors (deads@redhat.com)
- Adjust help template to latest version of Cobra (contact@fabianofranz.com)
- bump(github.com/inconshreveable/mousetrap):
  76626ae9c91c4f2a10f34cad8ce83ea42c93bb75C (contact@fabianofranz.com)
- bump(github.com/spf13/cobra): b78326bb16338c597567474a3ff35d76b75b804e
  (contact@fabianofranz.com)
- bump(github.com/spf13/pflag): 18d831e92d67eafd1b0db8af9ffddbd04f7ae1f4
  (contact@fabianofranz.com)
- UPSTREAM: make .Stream handle error status codes (deads@redhat.com)
- Test switching contexts with osc project (deads@redhat.com)
- set context namespace in generated .kubeconfig (deads@redhat.com)
- Add image streams to the types handled by web console. (cewong@redhat.com)
- perform initial commit to write config (pweil@redhat.com)
- OpenShift auth handler for v2 docker registry (rpenta@redhat.com)
- Merge pull request #1540 from liggitt/oauth_secret_config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1556 from smarterclayton/cleanup_new_app_resolution
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- OAuth secret config (jliggitt@redhat.com)
- new-app should treat image repositories as more specific than docker images
  (ccoleman@redhat.com)

* Tue Apr 07 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.5
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Project life-cycle updates (decarr@redhat.com)
- Merge pull request #1628 from ncdc/imagestream-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1622 from soltysh/post_1525
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1620 from derekwaynecarr/register_both_cases
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1417 from sdodson/service-dependencies
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1610 from
  derekwaynecarr/enable_namespace_lifecycle_handler
  (dmcphers+openshiftbot@redhat.com)
- Fix nil ImageStreamGetter (agoldste@redhat.com)
- Image stream validations (agoldste@redhat.com)
- Enable namespace exists and namespace lifecycle admission controllers
  (decarr@redhat.com)
- Merge pull request #1626 from sdodson/issue1614
  (dmcphers+openshiftbot@redhat.com)
- More uniform use of %%{name} macro (sdodson@redhat.com)
- Add /etc/openshift to rpm packaging (sdodson@redhat.com)
- Updated #1525 leftovers in sample-app (maszulik@redhat.com)
- Bug 1206109: Handle specified tags that don't exist in the specified image
  repository (kargakis@users.noreply.github.com)
- Register lower and camelCase for v1beta1 (decarr@redhat.com)
- Make service dependencies considerably more strict (sdodson@redhat.com)

* Tue Apr 07 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.4
- Bump

* Tue Apr 07 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.3
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1570 from derekwaynecarr/cherry_pick_ns_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1609 from bparees/build_desc
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1617 from
  fabianofranz/sample_app_readme_needs_cluster_admin_in_build_logs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1611 from deads2k/deads-fix-kubeconfig-validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1608 from liggitt/oauth_csrf_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1585 from liggitt/basic_auth_url_ca
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1578 from pweil-/reaper-typo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1605 from kargakis/appropriate-refactor
  (dmcphers+openshiftbot@redhat.com)
- The build-logs command needs a cluster admin to run, make it clear in README
  (contact@fabianofranz.com)
- Merge pull request #1600 from deads2k/deads-fix-display-of-custom-policy
  (dmcphers+openshiftbot@redhat.com)
- print builds as part of buildconfig description (bparees@redhat.com)
- Merge pull request #1580 from deads2k/deads-subresources
  (dmcphers+openshiftbot@redhat.com)
- only validate start args when NOT using config (deads@redhat.com)
- Merge pull request #1606 from bparees/buildcfg_ref
  (dmcphers+openshiftbot@redhat.com)
- Validate CSRF in external oauth flow (jliggitt@redhat.com)
- Move labeling functions to util package (kargakis@users.noreply.github.com)
- Merge pull request #1561 from bparees/tight_loops
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1604 from liggitt/windows_commands
  (dmcphers+openshiftbot@redhat.com)
- Plumb CA and client cert options into basic auth IDP (jliggitt@redhat.com)
- add buildconfig reference field to builds (bparees@redhat.com)
- support subresources (deads@redhat.com)
- UPSTREAM: support subresources in api info resolver (deads@redhat.com)
- Move hostname detection warning to runtime (jliggitt@redhat.com)
- Make osc.exe work on Windows (jliggitt@redhat.com)
- add delays when handling retries so we don't tight loop (bparees@redhat.com)
- UPSTREAM: add a blocking accept method to RateLimiter
  https://github.com/GoogleCloudPlatform/kubernetes/pull/6314
  (bparees@redhat.com)
- display pretty strings for policy rule extensions (deads@redhat.com)
- UPSTREAM Client must specify a resource version on finalize
  (decarr@redhat.com)
- fix typo (pweil@redhat.com)

* Mon Apr 06 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.2
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1587 from liggitt/image_format_node
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1554 from deads2k/deads-fix-start-node
  (dmcphers+openshiftbot@redhat.com)
- remove dead parameters from openshift start (deads@redhat.com)
- Merge pull request #1586 from liggitt/config_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1314 from smarterclayton/status
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Do not log errors where the event already exists
  (ccoleman@redhat.com)
- Implement an `osc status` command that groups resources by their
  relationships (ccoleman@redhat.com)
- Merge pull request #1594 from kargakis/wrap-proxy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1593 from kargakis/vet-test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1588 from nak3/add_option_description
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/gonum/graph):f6ac2b0f80f5a28ee70af78ce415393b37bcd6c1
  (ccoleman@redhat.com)
- Wrap proxy command (kargakis@users.noreply.github.com)
- go vet test (kargakis@users.noreply.github.com)
- Update config arg in Readme to be after command (ccoleman@redhat.com)
- Fix broken cli commands in README (ccoleman@redhat.com)
- Merge pull request #1583 from liggitt/oauth_redirect
  (dmcphers+openshiftbot@redhat.com)
- Add description for --master option of openshift-master start
  (nakayamakenjiro@gmail.com)
- Use imageConfig in node (jliggitt@redhat.com)
- Output config errors better (jliggitt@redhat.com)
- Merge pull request #1584 from fabianofranz/one_help_template_to_rule_them_all
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1507 from smarterclayton/template_label_position
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1581 from liggitt/login_error
  (dmcphers+openshiftbot@redhat.com)
- Integrating help and usage templates in all commands
  (contact@fabianofranz.com)
- Merge pull request #1525 from ncdc/image-stream
  (dmcphers+openshiftbot@redhat.com)
- Fix OAuth redirect (jliggitt@redhat.com)
- Show error on login redirect without token (jliggitt@redhat.com)
- Merge pull request #1533 from fabianofranz/issues_1529
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1573 from derekwaynecarr/dependency_namespace_controller
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1571 from jwforres/log_error_when_user_store_unavailable
  (dmcphers+openshiftbot@redhat.com)
- Deprecate ImageRepository, add ImageStream (agoldste@redhat.com)
- UPSTREAM: Pass ctx to Validate, ValidateUpdate (agoldste@redhat.com)
- Issue 1529 - fix regression in help templates (contact@fabianofranz.com)
- Merge pull request #1566 from smarterclayton/better_errors
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1553 from fabianofranz/sample_app_readme_with_login
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1548 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1532 from liggitt/unnest_idp_usage
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1535 from liggitt/verify_identity_reference
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1511 from kargakis/another-literal-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1531 from smarterclayton/disable_cadvisor_on_mac
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Remove the use of checkErr in our codebase (ccoleman@redhat.com)
- Add compile dependency on Kubernetes namespace controller (decarr@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes/pkg/namespace:8d94c43e705824f2
  3791b66ad5de4ea095d5bb32) (decarr@redhat.com)
- Adjust default log levels for web console and log errors when the local
  storage user store is unavailable (jforrest@redhat.com)
- Update sample app README to use login and project commands
  (contact@fabianofranz.com)
- Merge pull request #1549 from soltysh/update_sample_app_readme
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1515 from mfojtik/build_secret_extended
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1528 from bparees/fetchonupdate
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1433 from ironcladlou/deploy-trigger-test-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1538 from soltysh/sti_rebase
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1539 from liggitt/node_config_yaml
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1524 from bparees/fromref_bug
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1534 from liggitt/1207952_vagrant_cert_creation
  (dmcphers@redhat.com)
- Updated sample-app output to match current state of work
  (maszulik@redhat.com)
- Test PushSecretName via extended tests (mfojtik@redhat.com)
- Separate e2e-user config file in end to end (jliggitt@redhat.com)
- Merge pull request #1560 from
  smarterclayton/allow_project_change_with_empty_namespcae
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1559 from bparees/add_image_repos
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1551 from pmorie/birthcry
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1543 from tomgross/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1318 from kargakis/replace-import
  (dmcphers+openshiftbot@redhat.com)
- Uncommenting test for headless services (abhgupta@redhat.com)
- Replacing "None" for service PortalIP with constant (abhgupta@redhat.com)
- Merge pull request #1552 from liggitt/fix_assets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1479 from kargakis/test-cmd-build-chain
  (dmcphers+openshiftbot@redhat.com)
- return updated build+buildconfig objects on update (bparees@redhat.com)
- Changing projects with empty namespace should succeed (ccoleman@redhat.com)
- add db images to sample repos (bparees@redhat.com)
- UPSTREAM: Encode binary assets in ASCII only (jliggitt@redhat.com)
- Restore asset tests on Jenkins (jliggitt@redhat.com)
- Rewrite deployment ImageRepo handling (ironcladlou@gmail.com)
- Send a birthcry event when openshift node starts (pmorie@gmail.com)
- Set unix LF EOL for shell scripts (itconsense@gmail.com)
- Updated STI binary (maszulik@redhat.com)
- Tweak OAuth config (jliggitt@redhat.com)
- Verify identity reference (jliggitt@redhat.com)
- Fix node yaml config serialization, unify config reading helper methods
  (jliggitt@redhat.com)
- Use struct literal to build a new pipeline
  (kargakis@users.noreply.github.com)
- bump(github.com/openshift/source-to-
  image):957c66bdbe15daca7b3af41f2c311af160473796 (maszulik@redhat.com)
- Remove osin example (kargakis@users.noreply.github.com)
- bump(golang.org/x/oauth2):c4932a9b59a60daa02a28db1bb7be39d6ec2e542
  (kargakis@users.noreply.github.com)
- Remove duplicate oauth2 library (kargakis@users.noreply.github.com)
- Update vagrant cluster commands (jliggitt@redhat.com)
- Use the Fake cAdvisor interface on Macs (ccoleman@redhat.com)
- resolve from references when creating builds
  https://bugzilla.redhat.com/show_bug.cgi?id=1206052 (bparees@redhat.com)
- Move common template labels out of the objects description field
  (ccoleman@redhat.com)
- Test build-chain in hack/test-cmd (kargakis@users.noreply.github.com)

* Wed Apr 01 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.1
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Temporarily remove asset build failures from Jenkins (jliggitt@redhat.com)
- Merge pull request #1527 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Add helper for p12 cert creation (jliggitt@redhat.com)
- Update vagrant cert wiring (jliggitt@redhat.com)
- Fix build logs with authenticated node (jliggitt@redhat.com)
- Serve node/etcd over https (jliggitt@redhat.com)
- Merge pull request #1523 from pweil-/fix-resetbefore-methods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1343 from fabianofranz/test_e2e_with_login
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1496 from kargakis/minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1513 from kargakis/describe-help
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1451 from ironcladlou/post-deployment-hook-proposal
  (dmcphers+openshiftbot@redhat.com)
- fix ResetBefore* methods (pweil@redhat.com)
- Merge pull request #1526 from pweil-/fix-godep-hash (ccoleman@redhat.com)
- UPSTREAM: Fixing accidental hardcoding of priority function weight
  (abhgupta@redhat.com)
- UPSTREAM: Removing EqualPriority from the list of default priorities
  (abhgupta@redhat.com)
- UPSTREAM: Remove pods from the assumed pod list when they are deleted
  (abhgupta@redhat.com)
- UPSTREAM: Updating priority function weight based on specified configuration
  (abhgupta@redhat.com)
- Fix switching between contexts that don't have a namespace explicit
  (contact@fabianofranz.com)
- osc login must check if cert data were provided through flags before saving
  (contact@fabianofranz.com)
- Make test-end-to-end use osc login|project (contact@fabianofranz.com)
- UPSTREAM: fix godep hash from bad restore (pweil@redhat.com)
- Merge pull request #1508 from smarterclayton/fix_e2e_message
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1426 from deads2k/deads-expose-auth-config
  (dmcphers+openshiftbot@redhat.com)
- Add deprecation warnings for OAUTH envvars (jliggitt@redhat.com)
- expose oauth config in config (deads@redhat.com)
- new-app: Avoid extra declaration (kargakis@users.noreply.github.com)
- Wrap describe command (kargakis@users.noreply.github.com)
- Console now on /console (ccoleman@redhat.com)
- Expand validation and status spec (ironcladlou@gmail.com)

* Tue Mar 31 2015 Scott Dodson <sdodson@redhat.com> 0.4.2.0
- Merge remote-tracking branch 'upstream/pr/1309' (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1506 from bparees/build_ui
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1519 from pweil-/rebase_final
  (dmcphers+openshiftbot@redhat.com)
- print from repo info for sti builds (bparees@redhat.com)
- Merge pull request #1487 from kargakis/bug-fix
  (dmcphers+openshiftbot@redhat.com)
- rebase refactoring (pweil@redhat.com)
- UPSTREAM: eliminate fallback to root command when another command is
  explicitly requested (deads@redhat.com)
- UPSTREAM: Tone down logging in Kubelet for cAdvisor being dead
  (ccoleman@redhat.com)
- UPSTREAM: Make setSelfLink not bail out (ccoleman@redhat.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- UPSTREAM: Remove cadvisor_mock.go (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):f057a25b5d37a496c1ce25fbe1dc1
  b1971266240 (pweil@redhat.com)
- Merge pull request #1505 from jwforres/upstream_label_filter
  (dmcphers+openshiftbot@redhat.com)
- Bug 1206109 - build-chain: Set tags correctly
  (kargakis@users.noreply.github.com)
- Merge pull request #1510 from smarterclayton/support_headless_dns
  (dmcphers+openshiftbot@redhat.com)
- Moved LabelSelector and LabelFilter to their own bower component
  (jforrest@redhat.com)
- Merge pull request #1492 from bparees/describer
  (dmcphers+openshiftbot@redhat.com)
- Add extra tests for DNS and prepare for Headless services
  (ccoleman@redhat.com)
- Merge pull request #1498 from soltysh/quiet_push
  (dmcphers+openshiftbot@redhat.com)
- DRYed test cases: clarified recursion check (somalley@redhat.com)
- Support headless services via DNS, fix base queries (ccoleman@redhat.com)
- Merge pull request #1493 from mfojtik/fix_cm_panic
  (dmcphers+openshiftbot@redhat.com)
- add tag info to buildparameter output (bparees@redhat.com)
- Merge pull request #1491 from kargakis/typo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1502 from smarterclayton/handle_server_auth_errors
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1503 from fabianofranz/issues_1200
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1450 from liggitt/split_identity
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1500 from deads2k/deads-eliminate-security-on-docker-
  startup (dmcphers+openshiftbot@redhat.com)
- Make docker push quiet in builders (maszulik@redhat.com)
- Handle unexpected server errors more thoroughly in RequestToken
  (ccoleman@redhat.com)
- Merge pull request #1495 from deads2k/deads-fix-relative-paths
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1454 from deads2k/deads-cleanup-admin-commands
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1486 from codnee/master
  (dmcphers+openshiftbot@redhat.com)
- Issue 1200: use single quotes when printing usage for string flags
  (contact@fabianofranz.com)
- Comments (jliggitt@redhat.com)
- Lock down *min dependencies (jliggitt@redhat.com)
- Split identity and user objects (jliggitt@redhat.com)
- add how to disable security to the readme (deads@redhat.com)
- UPSTREAM: Fix namespace on delete (jliggitt@redhat.com)
- UPSTREAM: Ensure no namespace on create/update root scope types
  (jliggitt@redhat.com)
- relativize paths for master config (deads@redhat.com)
- clean up admin commands (deads@redhat.com)
- Fix panic when Source is not specified in CustomBuild (mfojtik@redhat.com)
- new-app: Fix typo (kargakis@users.noreply.github.com)
- Make 'osc project' use the provided io.Writer (d4rkn35t0@gmail.com)
- Remove hard-coded padding (jliggitt@redhat.com)
- Merge pull request #1476 from derekwaynecarr/cache_updates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1470 from derekwaynecarr/ns_delete
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1463 from pmorie/typo (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1474 from bparees/builddescribe
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1275 from soltysh/build_rework
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1478 from kargakis/bug-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1473 from jwforres/warning_terminating_projects
  (dmcphers+openshiftbot@redhat.com)
- Show warning on terminating projects and disable Create button
  (jforrest@redhat.com)
- Bug 1206419 - Handle empty namespace slice plus message fix
  (kargakis@users.noreply.github.com)
- Adding the the kubernetes and (future) openshift services to the list of
  Master server names (bleanhar@redhat.com)
- Fixed problems with deployment tests (maszulik@redhat.com)
- Move build generation logic to new endpoints (maszulik@redhat.com)
- Copying Build Labels into build Pod. (maszulik@redhat.com)
- Issue #333, #528 - add number to builds (maszulik@redhat.com)
- Issue #408 - Remove PodName from Build object (maszulik@redhat.com)
- Merge pull request #1257 from deads2k/deads-eliminate-cobra-fallback-to-root
  (dmcphers+openshiftbot@redhat.com)
- add-user became add-role-to-user (ccoleman@redhat.com)
- Move to upstream list/watch (decarr@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1471 from jwforres/update_readmes
  (dmcphers+openshiftbot@redhat.com)
- show buildconfig in describe output (bparees@redhat.com)
- Update all references to the console on port 8444 (jforrest@redhat.com)
- Merge pull request #1459 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1458 from jwforres/unify_assets
  (dmcphers+openshiftbot@redhat.com)
- Implement scaffolding to purge project content on deletion
  (decarr@redhat.com)
- Merge pull request #1449 from deads2k/deads-absolutize-backstep-references
  (dmcphers+openshiftbot@redhat.com)
- Asset changes needed to support a non / context root.  Global redirect to
  asset server or dump of api paths when / is requested. (jforrest@redhat.com)
- Initial changes to support console being served from same port as API or as
  its own server on a separate port (jliggitt@redhat.com)
- Merge pull request #1464 from kargakis/ict-validation-fix
  (dmcphers+openshiftbot@redhat.com)
- Drop From field validation check (kargakis@users.noreply.github.com)
- Make overwrite-policy require confirmation (ccoleman@redhat.com)
- Add an official 'openshift admin' command (ccoleman@redhat.com)
- Merge pull request #1466 from mfojtik/build_secret_test
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: eliminate fallback to root command when another command is
  explicitly requested (deads@redhat.com)
- Add unit tests for PushSecretName (mfojtik@redhat.com)
- Bug 1206109 - Default empty tag slice to 'latest'
  (kargakis@users.noreply.github.com)
- Merge pull request #1436 from smarterclayton/no_session_fail
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1413 from kargakis/fix-default-namespace
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1448 from derekwaynecarr/ns_delete
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Fix typo (pmorie@gmail.com)
- Specifying the correct kubeconfig file for the minions in a cluster
  (abhgupta@redhat.com)
- Merge pull request #1437 from deads2k/deads-fix-missing-serialization
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1455 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- fix missing serialization tag (deads@redhat.com)
- master and node config files should not contain backsteps (deads@redhat.com)
- Merge pull request #1452 from deads2k/deads-return-forbidden-api-object
  (dmcphers+openshiftbot@redhat.com)
- Add a client cache and remove log.Fatal behavior from session ended
  (ccoleman@redhat.com)
- UPSTREAM: Tone down logging in Kubelet for cAdvisor being dead
  (ccoleman@redhat.com)
- hack/test-cmd.sh is flaky on macs (ccoleman@redhat.com)
- Merge pull request #1453 from mfojtik/build_secret_followup
  (dmcphers+openshiftbot@redhat.com)
- build-chain: Fix default namespace setup (kargakis@users.noreply.github.com)
- Removing --master flag from the openshift-node service (abhgupta@redhat.com)
- Inherit LOGLEVEL in Build pods from OpenShift (mfojtik@redhat.com)
- Require to have 'dockercfg' Secret data for PushSecretName
  (mfojtik@redhat.com)
- Merge pull request #1411 from mfojtik/build_secrets
  (dmcphers+openshiftbot@redhat.com)
- return a forbidden api status object (deads@redhat.com)
- Merge pull request #1422 from smarterclayton/fix_login_messages
  (dmcphers+openshiftbot@redhat.com)
- tidy up MasterConfig getter location (deads@redhat.com)
- Allow to specify PushSecretName in BuildConfig Output (mfojtik@redhat.com)
- Issue 1349 - osc project supports switching by context name
  (contact@fabianofranz.com)
- Expose project status from underlying namespace (decarr@redhat.com)
- Merge pull request #1439 from smarterclayton/test_pod_describe
  (dmcphers+openshiftbot@redhat.com)
- Test pod describe in hack/test-cmd.sh (ccoleman@redhat.com)
- Merge pull request #1432 from bparees/imagerefs2
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1440 from pweil-/missing-policy-json
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1438 from mrunalp/reaper_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1427 from pmorie/cleanup
  (dmcphers+openshiftbot@redhat.com)
- Improve the general message flow for login (ccoleman@redhat.com)
- add missing json def (pweil@redhat.com)
- Merge pull request #1435 from deads2k/deads-remove-stupid-default
  (dmcphers+openshiftbot@redhat.com)
- reaper: Use NOHANG pattern instead of waiting for all children.
  (mrunalp@gmail.com)
- Exposing the scheduler config file option (abhgupta@redhat.com)
- eliminate bad defaults in create-kubeconfig (deads@redhat.com)
- Added image repository reference From field to STIBuildStrategy
  (yinotaurus@gmail.com)
- Merge pull request #1423 from sspeiche/bug1196138
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/pr/1417' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1398 from deads2k/deads-create-node-config-command
  (dmcphers+openshiftbot@redhat.com)
- Revert "Added image repository reference From field to STIBuildStrategy"
  (ccoleman@redhat.com)
- create-node-config command (deads@redhat.com)
- Remove old geard docs (pmorie@gmail.com)
- Added image repository reference From field to STIBuildStrategy
  (yinotaurus@gmail.com)
- Bug 1196138 add newline char for: openshift ex router and registry, output
  (sspeiche@redhat.com)
- Make service dependencies considerably more strict (sdodson@redhat.com)
- Merge pull request #1224 from ironcladlou/post-deployment-hook-proposal
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Introduce deployment hook proposal (ironcladlou@gmail.com)
- Merge pull request #1414 from deads2k/deads-fix-infinite-recursion
  (dmcphers+openshiftbot@redhat.com)
- delegate to kube method for unknown resource kinds (deads@redhat.com)
- Merge pull request #1403 from ncdc/add-virtual-image-resources
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1408 from kargakis/vet-pkg
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1407 from smarterclayton/fix_profiling
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1384 from ncdc/copy-IR-annotations-to-images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1400 from ncdc/fix-v2-registry-secure
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1229 from kargakis/output-dep
  (dmcphers+openshiftbot@redhat.com)
- Output build dependencies of a specific image repository
  (kargakis@users.noreply.github.com)
- add rule to allow self-subject access reviews (deads@redhat.com)
- go vet pkg (kargakis@users.noreply.github.com)
- bump(github.com/awalterschulze/gographviz):20d1f693416d9be045340150094aa42035
  a41c9e (kargakis@users.noreply.github.com)
- Merge pull request #1409 from smarterclayton/fix_travis_build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1376 from kargakis/godeps
  (dmcphers+openshiftbot@redhat.com)
- Remove GOFLAGS arguments from Makefile (ccoleman@redhat.com)
- Force OPENSHIFT_PROFILE to be tested (ccoleman@redhat.com)
- Fix web profiling (ccoleman@redhat.com)
- Merge pull request #1401 from bparees/nonrun_builds
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1330 from sg00dwin/testing-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1385 from jwforres/build_duration
  (dmcphers+openshiftbot@redhat.com)
- ImageStreamImage/ImageRepositoryTag virtual rsrcs (agoldste@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1397 from deads2k/deads-fix-node-kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- handle case where we have no build start time (bparees@redhat.com)
- add validation calls when we delegate cert commands (deads@redhat.com)
- pass master and public-master urls to create-all-certs (deads@redhat.com)
- make test-cmd more reliable on travis by pre-minting certs (deads@redhat.com)
- Use semantic deep equal for comparison (ccoleman@redhat.com)
- Refactor 12 (ccoleman@redhat.com)
- Fixed buildlogs (maszulik@redhat.com)
- Add info messages for end-to-end (ccoleman@redhat.com)
- Convert travis to make check-test (ccoleman@redhat.com)
- Refactor 11 (ccoleman@redhat.com)
- Kubelet health check fails when not sent to hostname (ccoleman@redhat.com)
- Ensure integration tests run when Docker is not available
  (ccoleman@redhat.com)
- Add check-test in between check and test (ccoleman@redhat.com)
- Remove debug logging from image controller (ccoleman@redhat.com)
- Refactor 10 (ccoleman@redhat.com)
- Refactor 9 (ccoleman@redhat.com)
- Refactor 8 (ccoleman@redhat.com)
- Refactor 7 (ccoleman@redhat.com)
- Refactor 6 (ccoleman@redhat.com)
- Refactor 5 (ccoleman@redhat.com)
- Refactor 4 (ccoleman@redhat.com)
- Refactor 3 (ccoleman@redhat.com)
- Refactor 2 (ccoleman@redhat.com)
- Refactor 1 (ccoleman@redhat.com)
- UPSTREAM: Remove global map from healthz (ccoleman@redhat.com)
- UPSTREAM: Lazily init systemd for code that includes cadvisor but doesn't use
  it (ccoleman@redhat.com)
- UPSTREAM: Make setSelfLink not bail out (ccoleman@redhat.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- UPSTREAM: Remove cadvisor_mock.go (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):c5f73516b677434d9cce7d07e460b
  b712c85e00b (ccoleman@redhat.com)
- Fix how OPENSHIFT_INSECURE is parsed (agoldste@redhat.com)
- Unset KUBECONFIG prior to hack/test-cmd.sh (ccoleman@redhat.com)
- Add build duration to web console (jforrest@redhat.com)
- Merge pull request #1396 from mfojtik/testutil
  (dmcphers+openshiftbot@redhat.com)
- Group all build extended tests into one file (mfojtik@redhat.com)
- Unify 'testutil' as package name in tests (mfojtik@redhat.com)
- Add test-extended.sh into Makefile (mfojtik@redhat.com)
- Merge pull request #1344 from ncdc/v2-registry
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1389 from deads2k/deads-allow-leading-slash
  (dmcphers+openshiftbot@redhat.com)
- Add v2 registry (agoldste@redhat.com)
- UPSTREAM: use docker's ParseRepositoryTag when pulling images
  (agoldste@redhat.com)
- bump(docker/docker):c1639a7e4e4667e25dd8c39eeccb30b8c8fc6357
  (agoldste@redhat.com)
- bump(docker/distribution):70560cceaf3ca9f99bfb2d6e84312e05c323df8b
  (agoldste@redhat.com)
- bump(Sirupsen/logrus):2cea0f0d141f56fae06df5b813ec4119d1c8ccbd
  (agoldste@redhat.com)
- Merge pull request #1391 from mfojtik/prepush_extended_image
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1387 from bparees/canceled_duration
  (dmcphers+openshiftbot@redhat.com)
- Pre-push ruby image for extended tests (mfojtik@redhat.com)
- handle trailing slashes (deads@redhat.com)
- Rework ui presentation and markup for builds view. Inclusion of noscript
  messages. Fix flex mixin which had ie 10 issue (sgoodwin@redhat.com)
- describe canceled build duration (bparees@redhat.com)
- Merge pull request #1369 from deads2k/deads-create-more-certs
  (dmcphers+openshiftbot@redhat.com)
- Copy ImageRepository annotations to image (agoldste@redhat.com)
- add create-client command (deads@redhat.com)
- add identities for router and registry (deads@redhat.com)
- Merge pull request #1378 from deads2k/deads-fix-login-test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1381 from mfojtik/fix_extended
  (dmcphers+openshiftbot@redhat.com)
- make login test avoid default kubeconfig chain (deads@redhat.com)
- Merge pull request #1366 from
  fabianofranz/rebase_kube_loader_with_multiple_config_files
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1382 from csrwng/custombuild_imagerepo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1298 from sdodson/increase-master-startup
  (dmcphers+openshiftbot@redhat.com)
- Fix typo in extended docker test (mfojtik@redhat.com)
- Add dockerImageRepository to sample custom builder image repo
  (cewong@redhat.com)
- Removed copies of util.DefaultClientConfig (contact@fabianofranz.com)
- Fix panic during timeout (ironcladlou@gmail.com)
- Rebasing upstream to allow any number of kubeconfig files
  (contact@fabianofranz.com)
- UPSTREAM: allow any number of kubeconfig files (contact@fabianofranz.com)
- Reworked integration tests and added extended tests (mfojtik@redhat.com)
- Add ./hack/test-extended.sh (mfojtik@redhat.com)
- bump(github.com/matttproud/golang_protobuf_extensions/ext):ba7d65ac66e9da93a7
  14ca18f6d1bc7a0c09100c (kargakis@users.noreply.github.com)
- auto-provision policy bindings for bootstrapping (deads@redhat.com)
- properly handle missing policy document (deads@redhat.com)
- Merge pull request #1326 from bparees/build_completion_time
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1372 from fabianofranz/issues_1348
  (dmcphers+openshiftbot@redhat.com)
- Issue 1348 - add support to expose persistent flags in help
  (contact@fabianofranz.com)
- Merge pull request #1328 from mnagy/e2e_database_test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1365 from jwforres/logger
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1362 from deads2k/deads-use-master-constant
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1329 from mnagy/sample_app_mysql
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1367 from liggitt/master_ip
  (dmcphers+openshiftbot@redhat.com)
- Query the sample app during e2e to make sure MySQL responds
  (nagy.martin@gmail.com)
- Merge pull request #1354 from csrwng/fix_readme
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1322 from sallyom/addtesting
  (dmcphers+openshiftbot@redhat.com)
- Make console logging be enabled/disabled with log levels and scoped loggers
  (jforrest@redhat.com)
- Merge pull request #1368 from TomasTomecek/docs-describe-user-creation
  (dmcphers+openshiftbot@redhat.com)
- use bootstrap policy constants for namespace and role default
  (deads@redhat.com)
- Set master IP correctly when starting kubernetes (jliggitt@redhat.com)
- Merge pull request #1355 from deads2k/deads-write-bootstrap-config
  (dmcphers+openshiftbot@redhat.com)
- added test case for empty name argument (somalley@redhat.com)
- examples, docs: describe user creation thoroughly (ttomecek@redhat.com)
- separate out bootstrap policy (deads@redhat.com)
- Merge pull request #1265 from liggitt/remove_docker_ip
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1360 from fabianofranz/issues_1356
  (dmcphers+openshiftbot@redhat.com)
- Remove docker IP from certs (jliggitt@redhat.com)
- Issue 1356 - setup should either save cert file or data
  (contact@fabianofranz.com)
- add completion time field to builds (bparees@redhat.com)
- Fix create-server-cert --signer-signer-{cert,key,serial} stutter
  (sdodson@redhat.com)
- Use new openshift/mysql-55-centos7 image for sample-app
  (nagy.martin@gmail.com)
- Change BindAddrArg to ListenArg (jliggitt@redhat.com)
- Fix grammar in README (cewong@redhat.com)
- Restore ability to run in http (jliggitt@redhat.com)
- Merge pull request #1340 from liggitt/config
  (dmcphers+openshiftbot@redhat.com)
- Group node certs under a single directory (jliggitt@redhat.com)
- Initial config validation (jliggitt@redhat.com)
- Merge pull request #1347 from fabianofranz/bugs_1202672
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1346 from ncdc/master (dmcphers+openshiftbot@redhat.com)
- Bug 1202672 - handle osc project without argument and no namespace set
  (contact@fabianofranz.com)
- Merge pull request #1303 from smarterclayton/add_docker_controller
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: remove exec ARGS log message (agoldste@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1280 from ncdc/exec-port-forward
  (dmcphers+openshiftbot@redhat.com)
- Call sdNotify as soon as we've started OpenShift API server or node
  (sdodson@redhat.com)
- Make file references in config relative to config files (jliggitt@redhat.com)
- Merge pull request #1333 from soltysh/issue1317
  (dmcphers+openshiftbot@redhat.com)
- Preserve tag when v1 pull by id is possible (agoldste@redhat.com)
- Change how tags and status events are recorded (ccoleman@redhat.com)
- Use the dockerImageReference tag when pushed (ccoleman@redhat.com)
- Add an initial importer for Image Repositories (ccoleman@redhat.com)
- Add osc exec and port-forward commands (agoldste@redhat.com)
- Issue1317 - Added error logging to webhook controller (maszulik@redhat.com)
- Merge pull request #1247 from deads2k/deads-intermediate-config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1334 from fabianofranz/bugs_1202686
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1332 from jwforres/temp_fix_catalog
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1325 from jwforres/error_handling
  (dmcphers+openshiftbot@redhat.com)
- Bug 1202686 - fixes forbidden error detection (contact@fabianofranz.com)
- add serializeable start config (deads@redhat.com)
- Error handling for web console, adds notification service and limits
  websocket re-connection retries (jforrest@redhat.com)
- Remove unwanted ng-if check from template catalog (jforrest@redhat.com)
- Minor message improvement (contact@fabianofranz.com)
- Add a forced version tagger for --use-version (sdodson@redhat.com)
- Teach our tito tagger to use vX.Y.Z tags (sdodson@redhat.com)
- Update specfile to 0.4.1 (sdodson@redhat.com)
- Generation command tests (cewong@redhat.com)
- Project is not required for a successful login (contact@fabianofranz.com)
- Introducing client login and setup - 'osc login' (contact@fabianofranz.com)
- Introducing client projects switching - 'osc project'
  (contact@fabianofranz.com)
- UPSTREAM: loader allows multiple sets of loading rules
  (contact@fabianofranz.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1319 from sdodson/bump-timeoutstartsec-300s
  (dmcphers+openshiftbot@redhat.com)
- Temporary fix to bump timeout to 300s (sdodson@redhat.com)
- Speed up installation of etcd (mfojtik@redhat.com)
- Merge pull request #1307 from smarterclayton/resolve_wildcard
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1313 from kargakis/remove-import-comment
  (dmcphers+openshiftbot@redhat.com)
- Handle wildcard resolution of services (ccoleman@redhat.com)
- Remove commented imports (kargakis@users.noreply.github.com)
- Merge pull request #1311 from kargakis/better-tests
  (dmcphers+openshiftbot@redhat.com)
- Make tests for the Docker parser more robust
  (kargakis@users.noreply.github.com)
- Allow test of only master to start successfully (ccoleman@redhat.com)
- Remove excessive logging (ccoleman@redhat.com)
- Tease apart separate concerns in RetryController (ccoleman@redhat.com)
- UPSTREAM: Don't hang when registering zero nodes (ccoleman@redhat.com)
- UPSTREAM: Temporarily relax annotations further (ccoleman@redhat.com)
- Merge pull request #1308 from smarterclayton/fix_dns_integration_test
  (dmcphers+openshiftbot@redhat.com)
- Always use port 8053 in integration tests (ccoleman@redhat.com)
- Merge pull request #1235 from pweil-/router-paths
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1294 from ncdc/use-image-repo-image-refs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1306 from kargakis/add-client-method
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1304 from soltysh/import_cleaning
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1263 from smarterclayton/fix_location_of_dns_default
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1295 from smarterclayton/run_without_address
  (dmcphers+openshiftbot@redhat.com)
- Add DockerImageReference type (agoldste@redhat.com)
- cancel-build: Use BuildLogs client method (kargakis@users.noreply.github.com)
- ose-haproxy-router: Run yum clean all to keep image smaller
  (sdodson@redhat.com)
- Removed extra tools imports (soltysh@gmail.com)
- Consolidate image reference generation/lookup (agoldste@redhat.com)
- path based ACLs (pweil@redhat.com)
- Merge pull request #1256 from smarterclayton/extra_provision
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1291 from ramr/hostgen (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1266 from csrwng/bug_119409
  (dmcphers+openshiftbot@redhat.com)
- Add additional items to vm-provision-full (ccoleman@redhat.com)
- Switch default route dns suffix to "router.default.local" (smitram@gmail.com)
- Fixes to route allocator plugin PR as per @smarterclayton comments.
  (smitram@gmail.com)
- Add simple shard allocator plugin to autogenerate host names for routes based
  on service and namespace and hook it into the route processing [GOFM].
  (smitram@gmail.com)
- Merge pull request #1290 from csrwng/fix_newapp_nodejs_builder
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1223 from ncdc/image-repo-tag-history
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1292 from bparees/check_template
  (dmcphers+openshiftbot@redhat.com)
- Various image updates (agoldste@redhat.com)
- Merge pull request #1299 from deads2k/deads-fix-build-follow
  (dmcphers+openshiftbot@redhat.com)
- fix start-build follow to stop following eventually (deads@redhat.com)
- Need more detail for contributing to v3 web console, started an architecture
  section (jforrest@redhat.com)
- Merge pull request #1276 from kargakis/minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Add save-images.sh that saves docker images to your home directory
  (sdodson@redhat.com)
- Docker 1.4.1 will not overwrite a tag without the -f flag
  (sdodson@redhat.com)
- Merge pull request #1281 from deads2k/deads-add-openshift-image-role
  (dmcphers+openshiftbot@redhat.com)
- Add more visual separation between builds, add copy to clipboard for webhook
  URLs (jforrest@redhat.com)
- integration test (deads@redhat.com)
- comments 2 (deads@redhat.com)
- check if template exists (bparees@redhat.com)
- Fix temporary platform builder image names for generation tools
  (cewong@redhat.com)
- Merge pull request #1264 from mrunalp/reaper
  (dmcphers+openshiftbot@redhat.com)
- openshift-0.4.1-0 (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- comments 1 (deads@redhat.com)
- allow bootstrap policy to span namespaces (deads@redhat.com)
- Merge pull request #1277 from
  jwforres/bug_1200346_include_multiplier_in_quota_comp
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1278 from deads2k/deads-make-forbidden-more-useful
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1187 from deads2k/deads-add-gets-lists
  (dmcphers+openshiftbot@redhat.com)
- Fix bindata for rework page structure to use flexbox so that sidebar columns
  extend (jforrest@redhat.com)
- Bug 1200346 - need to convert quota values including SI prefixes for
  comparisions (jforrest@redhat.com)
- role and rolebinding printers and describers (deads@redhat.com)
- add role and rolebinding gets and lists (deads@redhat.com)
- Bug 1200684: Retrieve logs from failed builds
  (kargakis@users.noreply.github.com)
- add detail to forbidden message (deads@redhat.com)
- Rework page structure to use flexbox so that sidebar columns extend without
  dynamically setting height Adjustments to project-nav, primarily the label-
  selector so that it doesn't wrap and tighten up the look.
  (sgoodwin@redhat.com)
- Adds reaping capability to openshift. (mrunalp@gmail.com)
- Allow OpenShift to start on an airplane (ccoleman@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- Merge pull request #1259 from sallyom/sample_app_doc_update
  (dmcphers+openshiftbot@redhat.com)
- Remove self closing tags (jliggitt@redhat.com)
- Merge pull request #1269 from mfojtik/process_stored_template
  (dmcphers+openshiftbot@redhat.com)
- added notes for Vagrant users in sample-app README doc (somalley@redhat.com)
- Merge pull request #1267 from soltysh/conversion_fix
  (dmcphers+openshiftbot@redhat.com)
- Allow stored templates to be referenced from osc process (mfojtik@redhat.com)
- DNS default check should not be in server.Config (ccoleman@redhat.com)
- Move glog.Fatal to error propagation (jliggitt@redhat.com)
- Add cert validation options to requestheader (jliggitt@redhat.com)
- Make sure we don't swallow errors from inner Convert calls
  (maszulik@redhat.com)
- Bug 119409 - fix source URI generated by new-app (cewong@redhat.com)
- Merge pull request #1258 from pweil-/make-integration
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1219 from sdodson/bump-sti-image-builder
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1254 from smarterclayton/dns
  (dmcphers+openshiftbot@redhat.com)
- Management Console - Create from template (cewong@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1240 from soltysh/issue941
  (dmcphers+openshiftbot@redhat.com)
- remove test-integration.sh from make test.  Resolves #1255 (pweil@redhat.com)
- Fixed URLs for webhooks presented in osc describe (maszulik@redhat.com)
- Add DNS support to OpenShift (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- bump(github.com/skynetservices/skydns):f18bd625a71b5d013b6e6288d1c7ec8796a801
  88 (ccoleman@redhat.com)
- Make it easy to export certs to curl (ccoleman@redhat.com)
- Ensure that we get the latest tags before we build (sdodson@redhat.com)
- Only push latest and the current tag (sdodson@redhat.com)
- Bump sti-image-builder to STI v0.2 (sdodson@redhat.com)
- Bump openshift-0.4-0 (sdodson@redhat.com)
- Merge tag 'v0.4' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.3.4-0].
  (sdodson@redhat.com)
- Merge tag 'v0.3.4' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge tag 'v0.3.3' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.3.3-0].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Cleanup dangling images from cache (sdodson@redhat.com)
- Add ose-docker-registry to ose-build script (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.3.2-0].
  (sdodson@redhat.com)
- Merge tag 'v0.3.2' (sdodson@redhat.com)
- First hack at docker builds for OSE (bleanhar@redhat.com)
- Merge remote-tracking branch 'upstream/master' (bleanhar@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Bump specfile 0.3.1 (sdodson@redhat.com)
- Merge tag 'v0.3.1' (sdodson@redhat.com)
- Revert "Drop the version variable from --images for now" (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-sdodson/set-images-format'
  (sdodson@redhat.com)
- Fix .el7dist string (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-sdodson/set-images-format'
  (sdodson@redhat.com)
- Merge tag 'v0.3' (sdodson@redhat.com)
- Update the custom tagger and builder to provide OpenShift ldflags
  (sdodson@redhat.com)
- Drop the version variable from --images for now (sdodson@redhat.com)
- Attempt to manipulate images path conditionally (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.2.2-0].
  (sdodson@redhat.com)
- Merge tag 'v0.2.2' (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.2.1-4].
  (sdodson@redhat.com)

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
