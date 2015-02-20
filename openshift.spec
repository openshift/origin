#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%{!?commit:
%global commit e7765f65e3a458475117bf1f41e05fba24ccaa9d
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# Openshift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 2+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.2.2-134-gc9e7c25aaf0e61-dirty -X github.com/openshift/origin/pkg/version.commitFromGit c9e7c25 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 72ad4f1 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.10.0-46-g72ad4f1
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
Version:        0.3.1
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
