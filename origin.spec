#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet
%global sdn_import_path github.com/openshift/openshift-sdn

# docker_version is the version of docker requires by packages
%global docker_version 1.6.2
# tuned_version is the version of tuned requires by packages
%global tuned_version  2.3
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.3.1
# %commit and %ldflags are intended to be set by tito custom builders provided
# in the rel-eng directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 45f2d27043b3605ad391c2b609624d6d98c5570c
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 1 -X github.com/openshift/origin/pkg/version.minorFromGit 0+ -X github.com/openshift/origin/pkg/version.versionFromGit v1.0.4-1048-g45f2d27 -X github.com/openshift/origin/pkg/version.commitFromGit 45f2d27 -X k8s.io/kubernetes/pkg/version.gitCommit 44c91b1 -X k8s.io/kubernetes/pkg/version.gitVersion v1.1.0-alpha.0-1605-g44c91b1
}

%if "%{dist}" == ".el7aos"
%global package_name atomic-openshift
%global product_name Atomic OpenShift
%else
%global package_name origin
%global product_name Origin
%endif

Name:           atomic-openshift
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the rel-eng directory of this project
Version:        3.0.1.901
Release:        0%{?dist}
Summary:        Open Source Container Management by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz
BuildRequires:  systemd
BuildRequires:  golang >= 1.4

%description
%{summary}

%package master
Summary:        %{product_name} Master
Requires:       %{name} = %{version}-%{release}
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description master
%{summary}

%package node
Summary:        %{product_name} Node
Requires:       %{name} = %{version}-%{release}
Requires:       docker-io >= %{docker_version}
Requires:       tuned-profiles-%{name}-node = %{version}-%{release}
Requires:       util-linux
Requires:       socat
Requires:       nfs-utils
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description node
%{summary}

%package -n tuned-profiles-%{name}-node
Summary:        Tuned profiles for %{product_name} Node hosts
Requires:       tuned >= %{tuned_version}

%description -n tuned-profiles-%{name}-node
%{summary}

%package clients
Summary:      %{product_name} Client binaries for Linux, Mac OSX, and Windows
BuildRequires: golang-pkg-darwin-amd64
BuildRequires: golang-pkg-windows-386

%description clients
%{summary}

%package dockerregistry
Summary:        Docker Registry v2 for %{product_name}
Requires:       %{name} = %{version}-%{release}

%description dockerregistry
%{summary}

%package pod
Summary:        %{product_name} Pod
Requires:       %{name} = %{version}-%{release}

%description pod
%{summary}

%package sdn-ovs
Summary:          %{product_name} SDN Plugin for Open vSwitch
Requires:         openvswitch >= %{openvswitch_version}
Requires:         %{name}-node = %{version}-%{release}
Requires:         bridge-utils
Requires:         ethtool

%description sdn-ovs
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
# Build all linux components we care about
for cmd in openshift dockerregistry
do
        go install -ldflags "%{ldflags}" %{import_path}/cmd/${cmd}
done

# Build clients for other platforms
# TODO: build cmd/oc instead of openshift
GOOS=windows GOARCH=386 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift
GOOS=darwin GOARCH=amd64 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift

#Build our pod
pushd images/pod/
    go build -ldflags "%{ldflags}" pod.go
popd

%install

install -d %{buildroot}%{_bindir}

# Install linux components
for bin in openshift dockerregistry
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _build/bin/${bin} %{buildroot}%{_bindir}/${bin}
done

# Install client executable for windows and mac
install -d %{buildroot}%{_datadir}/%{name}/{linux,macosx,windows}
install -p -m 755 _build/bin/openshift %{buildroot}%{_datadir}/%{name}/linux/oc
install -p -m 755 _build/bin/darwin_amd64/openshift %{buildroot}/%{_datadir}/%{name}/macosx/oc
install -p -m 755 _build/bin/windows_386/openshift.exe %{buildroot}/%{_datadir}/%{name}/windows/oc.exe

#Install pod
install -p -m 755 images/pod/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}%{_unitdir}

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig

for cmd in oc openshift-router openshift-deploy openshift-sti-build openshift-docker-build origin atomic-enterprise \
  oadm kubectl kubernetes kubelet kube-proxy kube-apiserver kube-controller-manager kube-scheduler ; do
    ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/$cmd
done

install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/{master,node}

# different service for origin vs aos
install -m 0644 rel-eng/%{name}-master.service %{buildroot}%{_unitdir}/%{name}-master.service
install -m 0644 rel-eng/%{name}-node.service %{buildroot}%{_unitdir}/%{name}-node.service
# same sysconfig files for origin vs aos
install -m 0644 rel-eng/origin-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-master
install -m 0644 rel-eng/origin-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-node
install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/%{name}-node-{guest,host}
install -m 0644 tuned/origin-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-guest/tuned.conf
install -m 0644 tuned/origin-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-host/tuned.conf
install -d -m 0755 %{buildroot}%{_mandir}/man7
install -m 0644 tuned/man/tuned-profiles-origin-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-%{name}-node.7

mkdir -p %{buildroot}%{_sharedstatedir}/origin


# Install sdn scripts
install -d -m 0755 %{buildroot}%{kube_plugin_path}
install -d -m 0755 %{buildroot}%{_unitdir}/docker.service.d
install -p -m 0644 rel-eng/docker-sdn-ovs.conf %{buildroot}%{_unitdir}/docker.service.d/
pushd _thirdpartyhacks/src/%{sdn_import_path}/ovssubnet/controller/kube/bin
   install -p -m 755 openshift-ovs-subnet %{buildroot}%{kube_plugin_path}/openshift-ovs-subnet
   install -p -m 755 openshift-sdn-kube-subnet-setup.sh %{buildroot}%{_bindir}/openshift-sdn-kube-subnet-setup.sh
popd
pushd _thirdpartyhacks/src/%{sdn_import_path}/ovssubnet/controller/multitenant/bin
   install -p -m 755 openshift-ovs-multitenant %{buildroot}%{_bindir}/openshift-ovs-multitenant
   install -p -m 755 openshift-sdn-multitenant-setup.sh %{buildroot}%{_bindir}/openshift-sdn-multitenant-setup.sh
popd
install -d -m 0755 %{buildroot}%{_unitdir}/%{name}-node.service.d
install -p -m 0644 rel-eng/openshift-sdn-ovs.conf %{buildroot}%{_unitdir}/%{name}-node.service.d/openshift-sdn-ovs.conf

# Install bash completions
install -d -m 755 %{buildroot}%{_sysconfdir}/bash_completion.d/
install -p -m 644 rel-eng/completions/bash/* %{buildroot}%{_sysconfdir}/bash_completion.d/
# Generate atomic-enterprise bash completions
%{__sed} -e "s|openshift|atomic-enterprise|g" rel-eng/completions/bash/openshift > %{buildroot}%{_sysconfdir}/bash_completion.d/atomic-enterprise

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/oc
%{_bindir}/openshift
%{_bindir}/openshift-router
%{_bindir}/openshift-deploy
%{_bindir}/openshift-sti-build
%{_bindir}/openshift-docker-build
%{_bindir}/origin
%{_bindir}/atomic-enterprise
%{_bindir}/oadm
%{_bindir}/kubectl
%{_bindir}/kubernetes
%{_bindir}/kubelet
%{_bindir}/kube-proxy
%{_bindir}/kube-apiserver
%{_bindir}/kube-controller-manager
%{_bindir}/kube-scheduler
%{_sharedstatedir}/origin
%{_sysconfdir}/bash_completion.d/*
%dir %config(noreplace) %{_sysconfdir}/origin

%pre
# If /etc/openshift exists and /etc/origin doesn't, symlink it to /etc/origin
if [ -d "%{_sysconfdir}/openshift" ]; then
  if ! [ -d "%{_sysconfdir}/origin"  ]; then
    ln -s %{_sysconfdir}/openshift %{_sysconfdir}/origin
  fi
fi

%files master
%defattr(-,root,root,-)
%{_unitdir}/%{name}-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-master
%config(noreplace) %{_sysconfdir}/origin/master
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.key
%ghost %config(noreplace) %{_sysconfdir}/origin/admin.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.key
%ghost %config(noreplace) %{_sysconfdir}/origin/ca.serial.txt
%ghost %config(noreplace) %{_sysconfdir}/origin/etcd.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/etcd.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master-config.yaml
%ghost %config(noreplace) %{_sysconfdir}/origin/master.etcd-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.etcd-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master.kubelet-client.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.kubelet-client.key
%ghost %config(noreplace) %{_sysconfdir}/origin/master.server.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/master.server.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-master.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-registry.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.crt
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.key
%ghost %config(noreplace) %{_sysconfdir}/origin/openshift-router.kubeconfig
%ghost %config(noreplace) %{_sysconfdir}/origin/policy.json
%ghost %config(noreplace) %{_sysconfdir}/origin/serviceaccounts.private.key
%ghost %config(noreplace) %{_sysconfdir}/origin/serviceaccounts.public.key

%post master
%systemd_post %{basename:%{name}-master.service}
# Create master config and certs if both do not exist
if [[ ! -e %{_sysconfdir}/origin/master/master-config.yaml &&
     ! -e %{_sysconfdir}/origin/master/ca.crt ]]; then
%if "%{dist}" == ".el7aos"
  %{_bindir}/atomic-enterprise start master --write-config=%{_sysconfdir}/origin/master
%else
  %{_bindir}/openshift start master --write-config=%{_sysconfdir}/origin/master
%endif
  # Create node configs if they do not already exist
  if ! find %{_sysconfdir}/origin/ -type f -name "node-config.yaml" | grep -E "node-config.yaml"; then
    %{_bindir}/oadm create-node-config --node-dir=%{_sysconfdir}/origin/node/ --node=localhost --hostnames=localhost,127.0.0.1 --node-client-certificate-authority=%{_sysconfdir}/origin/master/ca.crt --signer-cert=%{_sysconfdir}/origin/master/ca.crt --signer-key=%{_sysconfdir}/origin/master/ca.key --signer-serial=%{_sysconfdir}/origin/master/ca.serial.txt --certificate-authority=%{_sysconfdir}/origin/master/ca.crt
  fi
  # Generate a marker file that indicates config and certs were RPM generated
  echo "# Config generated by RPM at "`date -u` > %{_sysconfdir}/origin/.config_managed
fi


%preun master
%systemd_preun %{basename:%{name}-master.service}

%postun master
%systemd_postun

%files node
%defattr(-,root,root,-)
%{_unitdir}/%{name}-node.service
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}-node
%config(noreplace) %{_sysconfdir}/origin/node

%post node
%systemd_post %{basename:%{name}-node.service}

%preun node
%systemd_preun %{basename:%{name}-node.service}

%postun node
%systemd_postun

%files sdn-ovs
%defattr(-,root,root,-)
%{_bindir}/openshift-sdn-kube-subnet-setup.sh
%{_bindir}/openshift-ovs-multitenant
%{_bindir}/openshift-sdn-multitenant-setup.sh
%{kube_plugin_path}/openshift-ovs-subnet
%{_unitdir}/%{name}-node.service.d/openshift-sdn-ovs.conf
%{_unitdir}/docker.service.d/docker-sdn-ovs.conf

%files -n tuned-profiles-%{name}-node
%defattr(-,root,root,-)
%{_prefix}/lib/tuned/%{name}-node-host
%{_prefix}/lib/tuned/%{name}-node-guest
%{_mandir}/man7/tuned-profiles-%{name}-node.7*

%post -n tuned-profiles-%{name}-node
recommended=`/usr/sbin/tuned-adm recommend`
if [[ "${recommended}" =~ guest ]] ; then
  /usr/sbin/tuned-adm profile %{name}-node-guest > /dev/null 2>&1
else
  /usr/sbin/tuned-adm profile %{name}-node-host > /dev/null 2>&1
fi

%preun -n tuned-profiles-%{name}-node
# reset the tuned profile to the recommended profile
# $1 = 0 when we're being removed > 0 during upgrades
if [ "$1" = 0 ]; then
  recommended=`/usr/sbin/tuned-adm recommend`
  /usr/sbin/tuned-adm profile $recommended > /dev/null 2>&1
fi

%files clients
%{_datadir}/%{name}/linux/oc
%{_datadir}/%{name}/macosx/oc
%{_datadir}/%{name}/windows/oc.exe

%files dockerregistry
%defattr(-,root,root,-)
%{_bindir}/dockerregistry

%files pod
%defattr(-,root,root,-)
%{_bindir}/pod


%changelog
* Tue Sep 08 2015 Scott Dodson <sdodson@redhat.com> 3.0.1.901
- Bump 3.0.1.901 Early Access 3 RC (sdodson@redhat.com)
- Make unknown trigger types to be warning not error (rhcarvalho@gmail.com)
- Add volume size option (dmcphers@redhat.com)
- don't test the git connection when a proxy is set (bparees@redhat.com)
- split extended tests into functional areas (deads@redhat.com)
- Filter routes by namespace or project labels (ccoleman@redhat.com)
- Tests for adding envVars to buildConfig via new-build (jhadvig@redhat.com)
- Add --env flag to the new-build (jhadvig@redhat.com)
- Fix broken SDN on multinode vagrant environment (rpenta@redhat.com)
- Fix docs of oc (prukner.jan@seznam.cz)
- Handle multiple paths together (ccoleman@redhat.com)
- Add AuthService.withUser() call to CreateProjectController
  (spadgett@redhat.com)
- fix process kill in old e2e.sh (deads@redhat.com)
- UPSTREAM: 13322: Various exec fixes (jliggitt@redhat.com)
- bump(github.com/fsouza/go-dockerclient):
  76fd6c68cf24c48ee6a2b25def997182a29f940e (jliggitt@redhat.com)
- make impersonateSAR with empty token illegal (deads@redhat.com)
- improve nodeconfig validation (deads@redhat.com)
- Check pointer before using it (rhcarvalho@gmail.com)
- Issue 2683 - deprecation warning from moment.js (jforrest@redhat.com)
- Fix manual deployment (ironcladlou@gmail.com)
- Fix copy-paste in test (rhcarvalho@gmail.com)
- UPSTREAM(docker/distribution): manifest deletions (agoldste@redhat.com)
- UPSTREAM(docker/distribution): custom routes/auth (agoldste@redhat.com)
- UPSTREAM(docker/distribution): add BlobService (agoldste@redhat.com)
- UPSTREAM(docker/distribution): add layer unlinking (agoldste@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- bump(github.com/AdRoll/goamz/{aws,s3}):cc210f45dcb9889c2769a274522be2bf70edfb
  99 (mkargaki@redhat.com)
- switch integration tests to non-default ports (deads@redhat.com)
- new-app: Fix oc expose recommendation (mkargaki@redhat.com)
- Allow customization of login page (spadgett@redhat.com)
- [RPMs] Fix requirements between node and tuned profiles (sdodson@redhat.com)
- Fix token display page (jliggitt@redhat.com)
- integration/diag_nodes_test.go: fix test flake issue 4499 (lmeyer@redhat.com)
- Remove background-size and include no-repeat (sgoodwin@redhat.com)
- enable docker registry wait in forcepull bucket (skuznets@redhat.com)
- bump(github.com/docker/distribution):1341222284b3a6b4e77fb64571ad423ed58b0d34
  (mkargaki@redhat.com)
- Secret help - creating .dockercfg from file should be done via 'oc secrets
  new' (jhadvig@redhat.com)
- Add bcrypt to htpasswd auth (jimmidyson@gmail.com)
- Renaming osc to oc in cmd code (jhadvig@redhat.com)
- Prevent duplicate routes from being exposed (ccoleman@redhat.com)
- use buildlog_level field for docker build log level (bparees@redhat.com)
- Fix for nav bar issues on resize.
  https://github.com/openshift/origin/issues/4404 - Styling details of filter,
  add to project, and toggle menu - Use of flex for positioning
  (sgoodwin@redhat.com)
- Router can filter on namespace, label, field (ccoleman@redhat.com)
- Added --generator to expose command as receive error message. Added used of
  v2 tagged image and different color to further emphasize the difference.
  (sspeiche@redhat.com)
- UPSTREAM: 9870: PV Recycler config (mturansk@redhat.com)
- UPSTREAM: 12603: Expanded volume.Spec (mturansk@redhat.com)
- UPSTREAM: 13310: Added VolumeConfig to Volumes (mturansk@redhat.com)
- UPSTREAM: revert 9c1056e: 12603: Expanded volume.Spec to full Volume and PV
  (mturansk@redhat.com)
- UPSTREAM: revert 3b01c2c: 9870: configurable pv recyclers
  (mturansk@redhat.com)
- implementation of deleting from settings using modal
  (gabriel_ruiz@symantec.com)
- better build-logs error messages (bparees@redhat.com)
- expose project creation failures (deads@redhat.com)
- diagnostics: fix, make tests happy (lmeyer@redhat.com)
- diagnostics: remove log message format helpers (lmeyer@redhat.com)
- diagnostics: remove machine-readable output formats (lmeyer@redhat.com)
- diagnostics: revise per code reviews (lmeyer@redhat.com)
- diagnostics: k8s repackaged (lmeyer@redhat.com)
- diagnostics: add registry and router diagnostics (lmeyer@redhat.com)
- diagnostics: complete refactor (lmeyer@redhat.com)
- diagnostics: begin large refactor (deads@redhat.com)
- introduce `openshift ex diagnostics` (lmeyer@redhat.com)
- Only set masterIP with valid IPs, avoid calling OverrideConfig when writing
  config (jliggitt@redhat.com)
- refactored ldap utils (skuznets@redhat.com)
- Triggers should not be omit empty (ccoleman@redhat.com)
- If Vagrant sets a hostname that doesn't resolve it breaks containerized
  installs (bleanhar@redhat.com)
- enable extended test for old config (deads@redhat.com)
- Allow RequestHeaderIdentityProvider to redirect to UI login or challenging
  URL (jliggitt@redhat.com)
- Simplify messages (ccoleman@redhat.com)
- Update to etcd v2.1.2 (ccoleman@redhat.com)
- bump(github.com/coreos/etcd):v2.1.2 (ccoleman@redhat.com)
- Making the regex in hack/test-cmd.sh a little more flexible for downstream
  (bleanhar@redhat.com)
- Node IP can be passed as node config option (rpenta@redhat.com)
- Set console logo with css instead of within markup so that it can be
  customized. ref: https://github.com/openshift/origin/issues/4148
  (sgoodwin@redhat.com)
- Load UI extensions outside the OpenShift binary (spadgett@redhat.com)
- More UI integration test fixes (ffranz@redhat.com)
- Fix for issue #4437 - restarting the haproxy router still dispatches
  connections to a downed backend. (smitram@gmail.com)
- fix TestUnprivilegedNewProjectDenied flake (deads@redhat.com)
- bump(github.com/docker/spdystream):b2c3287 (ccoleman@redhat.com)
- make sure we don't accidentally drop admission plugins (deads@redhat.com)
- Allow origin clientcmd to use kubeconfig (ccoleman@redhat.com)
- Add project service, refactor projectNav (admin@benjaminapetersen.me)
- Disable complex console integration tests (ffranz@redhat.com)
- Do not print out the which error for etcd (ccoleman@redhat.com)
- Port Route to genericetcd (ccoleman@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  4fc1cd198cd990b2c5120bd03304ef207b5ee1bc (rpenta@redhat.com)
- Reuse existing sdn GetNodeIP() for fetching node IP (rpenta@redhat.com)
- Make OsdnRegistryInterface compatible with openshift-sdn SubnetRegistry
  interface (rpenta@redhat.com)
- Make openshift SDN MTU configurable (rpenta@redhat.com)
- Fix nil panic (jliggitt@redhat.com)
- UPSTREAM: 13317: Recover panics in finishRequest, write correct API response
  (jliggitt@redhat.com)
- Make create flow forms always editable (spadgett@redhat.com)
- Bug 1256319: get: Fix nil timestamp output (mkargaki@redhat.com)
- Precompile Angular templates (spadgett@redhat.com)
- Revert "UPSTREAM: <carry>: implement a generic webhook storage"
  (ccoleman@redhat.com)
- Use a local webhook (ccoleman@redhat.com)
- Preserve permissions during image build copy (ccoleman@redhat.com)
- Add the kubernetes service IP to the cert list (ccoleman@redhat.com)
- Re-enable complex console integration tests (ffranz@redhat.com)
- ux for deleting a project, no api call implemented yet (gabe@ggruiz.me)
- Workaround slow ECDHE in F5 router tests (miciah.masters@gmail.com)
- fix typo in docker_version definition, handle origin pre-existing symlink
  (admiller@redhat.com)
- Convert zookeeper template to v1 (mfojtik@redhat.com)
- Skip second validation in when creating dockercfg secret (jhadvig@redhat.com)
- Fix bugz 1243529 - HAProxy template is overwritten by incoming changes.
  (smitram@gmail.com)
- Revert previous Vagrantfile cleanup (rpenta@redhat.com)
- Add empty state help for projects page (spadgett@redhat.com)
- F5 router implementation (miciah.masters@gmail.com)
- Add additional secrets to custom builds (cewong@redhat.com)
- stop: Add deprecation warning; redirect to delete (mkargaki@redhat.com)
- Remove unnecessary if condition in custom-docker-builder/buid.sh
  (nakayamakenjiro@gmail.com)
- Use os::build::setup_env when building extended test package
  (mfojtik@redhat.com)
- buildchain: Fix resource shortcut (mkargaki@redhat.com)
- oc new-app with no arguments will suggest --search and --list
  (jhadvig@redhat.com)
- bump(github.com/openshift/openshift-sdn)
  5a5c409df14c066f564b6015d474d1bf88da2424 (rpenta@redhat.com)
- Return node IPs in GetNodes() SDN interface (rpenta@redhat.com)
- bump(github.com/openshift/source-to-image)
  00d1cb3cb9224bb59c0a37bb2bdd0100e20e1982 (cewong@redhat.com)
- document why namespaces are stripped (bparees@redhat.com)
- Cleanup Vagrantfile (rpenta@redhat.com)
- rename jenkins version (bparees@redhat.com)
- add extended test for s2i incremental builds using docker auth credentials to
  push and pull (bparees@redhat.com)
- plugins/osdn: multitenant service isolation support (danw@redhat.com)
- Add ServiceNetwork field to ClusterNetwork struct (danw@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  9d342eb61cfdcb1d77045ba69b27745f600385e3 (danw@redhat.com)
- Allow to override the default Jenkins image in example (mfojtik@redhat.com)
- Add support for dind image caching (marun@redhat.com)
- Improve graceful shutdown of dind daemon (marun@redhat.com)
- fix wf81 imagestream (bparees@redhat.com)
- Change default instance type (dmcphers@redhat.com)
- Fixup router test hostnames - good catch @Miciah (smitram@gmail.com)
- Restructure of nav layout and presentation at mobile resolutions to address
  https://github.com/openshift/origin/issues/3149 (sgoodwin@redhat.com)
- Add support for docker-in-docker dev cluster (marun@redhat.com)
- Prevent panic in import-image (ccoleman@redhat.com)
- Remove flakiness in webhook test (cewong@redhat.com)
- Add SOURCE_REF variable to builder container (mfojtik@redhat.com)
- change OpenShift references to Origin (pweil@redhat.com)
- Move documentation to test/extended/README.md (mfojtik@redhat.com)
- ext-tests: CLI interface docs (jhadvig@redhat.com)
- Initial docs about writing extended test (mfojtik@redhat.com)
- Remove sti-image-builder from our build-images flow (mfojtik@redhat.com)
- Add 'displayName' to Template (mfojtik@redhat.com)
- Fix 'pods "hello-openshift" cannot be updated' flake (jliggitt@redhat.com)
- make service targetPort consistent with container port (tangbixuan@gmail.com)
- Refactor vagrant provision scripts for reuse (marun@redhat.com)
- UPSTREAM: 13107: Fix portforward test flake with GOMAXPROCS > 1
  (jliggitt@redhat.com)
- UPSTREAM: 12162: Correctly error when all port forward binds fail
  (jliggitt@redhat.com)
- Minor cleanup (ironcladlou@gmail.com)
- Support prefixed deploymentConfig name (ironcladlou@gmail.com)
- Add vpc option to vagrantfile (dmcphers@redhat.com)
- Wait for the builder service account to get registry secrets in extended
  tests (mfojtik@redhat.com)
- Removing unused conversion tool, which was replaced with
  cmd/genconversion/conversion.go some time ago, already. (maszulik@redhat.com)
- Update k8s repository links and fix docs links (maszulik@redhat.com)
- reconcile-cluster-roles: Support union of default and modified cluster roles
  (mkargaki@redhat.com)
- Fix permission issues in zookeeper example (mfojtik@redhat.com)
- Make output directory symlinks relative links (stefw@redhat.com)
- Cleanup etcd install (ccoleman@redhat.com)
- Make config change triggers a default (ccoleman@redhat.com)
- Support generating DeploymentConfigs from run (ccoleman@redhat.com)
- UPSTREAM: 13011: Make run support other types (ccoleman@redhat.com)
- Add attach, run, and annotate to cli (ccoleman@redhat.com)
- Allow listen address to be overriden on api start (ccoleman@redhat.com)
- Completion generation can't run on Mac (ccoleman@redhat.com)
- Govet doesn't run on Mac (ccoleman@redhat.com)
- Split verify step into its own make task (ccoleman@redhat.com)
- Don't use _tmp or cp -u (ccoleman@redhat.com)
- Don't need to test same stuff twice (ccoleman@redhat.com)
- extended tests for setting forcePull in the 3 strategies; changes stemming
  from Ben's comments; some debug improvements; Michal's comments; address
  merge conflicts; adjust to extended test refactor (gmontero@redhat.com)
- Print line of error (ccoleman@redhat.com)
- Add stack dump to log on sigquit of sti builder (bparees@redhat.com)
- Overwriting a volume claim with --claim-name not working
  (ccoleman@redhat.com)
- change internal representation of rolebindings to use subjects
  (deads@redhat.com)
- remove export --all (deads@redhat.com)
- Tests failing at login, fix name of screenshots to be useful Remove the
  backporting of selenium since we no longer use phantom Remove phantomjs
  protractor config (jforrest@redhat.com)
- Remove double-enabled build controllers (jliggitt@redhat.com)
- add --all-namespaces to export (deads@redhat.com)
- fix --all (bparees@redhat.com)
- dump the namespaces at the end of e2e (bparees@redhat.com)
- Completion (ccoleman@redhat.com)
- OpenShift master setup example (ccoleman@redhat.com)
- Allow master-ip to set when running the IP directly (ccoleman@redhat.com)
- UPSTREAM: 12595 <drop>: Support status.podIP (ccoleman@redhat.com)
- Watch from the latest valid index for leader lease (ccoleman@redhat.com)
- add namespace to cluster SAR (deads@redhat.com)
- rpm: Added simple test case script for rpm builds. (smilner@redhat.com)
- Adding more retriable error types for push retry logic (jhadvig@redhat.com)
- Origin and Atomic OpenShift package refactoring (sdodson@redhat.com)
- Rename openshift.spec origin.spec (sdodson@redhat.com)
- update master for new recycler (mturansk@redhat.com)
- UPSTREAM: 5093+12603: adapt downward api volume to volume changes
  (deads@redhat.com)
- UPSTREAM: 6093+12603: adapt cephfs to volume changes (deads@redhat.com)
- UPSTREAM: 9870: configurable pv recyclers (deads@redhat.com)
- UPSTREAM: 12603: Expanded volume.Spec to full Volume and PV
  (deads@redhat.com)
- UPSTREAM: revert faab6cb: 9870: Allow Recyclers to be configurable
  (deads@redhat.com)
- disable SA secret ref limitting per SA (deads@redhat.com)
- Adding extended-tests for build-label (jhadvig@redhat.com)
- Add Docker labels (jhadvig@redhat.com)
- bump(openshift/source-to-image) a737bdd101de4a013758ad01f4bdd1c8d2f912b3
  (jhadvig@redhat.com)
- Extended test fixtures (jhadvig@redhat.com)
- Fix for issue #4035 - internally generated router keys are not unique.
  (smitram@gmail.com)
- Fix failing integration test expectation - we now return a service
  unavailable error rather than connect to 127.0.0.1:8080 (smitram@gmail.com)
- Include namespace in determining new-app dup objects (cewong@redhat.com)
- Use instance_type param (dmcphers@redhat.com)
- Remove default backend from the mix. In the first case, it returns incorrect
  info if something is serving on port 8080. The second bit is if nothing is
  running on port 8080, the cost to return a 503 is high. If someone wants
  custom 503 messages, they can always add a custom backend or use the
  errorfile 503 /path/to/page directive in a custom template.
  (smitram@gmail.com)
- UPSTREAM: 11827: allow permissive SA secret ref limitting (deads@redhat.com)
- Make the docker registry client loggable (ccoleman@redhat.com)
- bump(github.com/openshift/openshift-sdn):
  9dd0b510146571d42c5c9371b4054eae2dc5f82c (rpenta@redhat.com)
- Rename VindMap to VNIDMap (rpenta@redhat.com)
- Fixing the retry logic (jhadvig@redhat.com)
- Add standard vars to hook pod environment (ironcladlou@gmail.com)
- Run e2e UI test in chrome (jliggitt@redhat.com)
- Remove dot imports from extended tests (mfojtik@redhat.com)
- display the host in 'oc status' (v.behar@free.fr)
- Typo in https proxy debug output (swapdisk@users.noreply.github.com)
- use push auth creds to pull previous image for incremental build
  (bparees@redhat.com)
- Fix sdn api field names to match openshift-sdn repo (rpenta@redhat.com)
- show build context in oc status (deads@redhat.com)
- make oc status build output consistent with deployments (deads@redhat.com)
- Add namespace flag to trigger enable instructions (ironcladlou@gmail.com)
- add jenkins to imagestream definitions (bparees@redhat.com)
- UPSTREAM: 8530: GCEPD mounting on Atomic (pweil@redhat.com)
- bump(openshift/source-to-image) 2e52377338d425a290e74192ba8d53bb22965b0d
  (bparees@redhat.com)
- Fixing UI test reference to Origin (bleanhar@redhat.com)
- Minor tweak to hack/install-assets.sh for the RHEL AMI (bleanhar@redhat.com)
- Making the regex in hack/test-cmd.sh a little more flexible for downstream
  (bleanhar@redhat.com)
- Re-adding the upstream Dockerfiles (bleanhar@redhat.com)

* Wed Aug 12 2015 Steve Milner <smilner@redhat.com> 0.2-8
- Master configs will be generated if none are found when the master is installed.
- Node configs will be generated if none are found when the master is installed.
- Additional notice file added if config is generated by the RPM.
- All-In-One services removed.

* Wed Aug 12 2015 Steve Milner <smilner@redhat.com> 0.2-7
- Added new ovs script(s) to file lists.

* Wed Aug  5 2015 Steve Milner <smilner@redhat.com> 0.2-6
- Using _unitdir instead of _prefix for unit data

* Fri Jul 31 2015 Steve Milner <smilner@redhat.com> 0.2-5
- Configuration location now /etc/origin
- Default configs created upon installation

* Tue Jul 28 2015 Steve Milner <smilner@redhat.com> 0.2-4
- Added AEP packages

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
