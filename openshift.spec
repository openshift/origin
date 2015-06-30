#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%global kube_plugin_path /usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet
%global sdn_import_path github.com/openshift/openshift-sdn

# %commit and %ldflags are intended to be set by tito custom builders provided
# in the rel-eng directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 151be0df375f35539144881ec87b69869e177df3
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# OpenShift specific ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 1 -X github.com/openshift/origin/pkg/version.minorFromGit 0+ -X github.com/openshift/origin/pkg/version.versionFromGit v1.0.0-412-g151be0d -X github.com/openshift/origin/pkg/version.commitFromGit 151be0d -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit 496be63 -X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion v0.17.1-804-g496be63
}

Name:           openshift
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the rel-eng directory of this project
Version:        3.0.1.0
Release:        0%{?dist}
Summary:        Open Source Platform as a Service by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz

BuildRequires:  systemd
BuildRequires:  golang >= 1.4


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
Requires:       docker-io >= 1.6.2
Requires:       tuned-profiles-openshift-node
Requires:       util-linux
Requires:       socat
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

%package clients
Summary:      Openshift Client binaries for Linux, Mac OSX, and Windows
BuildRequires: golang-pkg-darwin-amd64
BuildRequires: golang-pkg-windows-386

%description clients
%{summary}

%package dockerregistry
Summary:        Docker Registry v2 for OpenShift
Requires:       %{name} = %{version}-%{release}

%description dockerregistry
%{summary}

%package pod
Summary:        OpenShift Pod
Requires:       %{name} = %{version}-%{release}

%description pod
%{summary}

%package sdn-ovs
Summary:          OpenShift SDN Plugin for Open vSwitch
Requires:         openvswitch >= 2.3.1
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

# Build only 'openshift' for other platforms
GOOS=windows GOARCH=386 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift
GOOS=darwin GOARCH=amd64 go install -ldflags "%{ldflags}" %{import_path}/cmd/openshift

#Build our pod
pushd images/pod/
    go build -ldflags "%{ldflags}" pod.go
popd

%install

install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_datadir}/%{name}/{linux,macosx,windows}

# Install linux components
for bin in openshift dockerregistry
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _build/bin/${bin} %{buildroot}%{_bindir}/${bin}
done
# Install 'openshift' as client executable for windows and mac
install -p -m 755 _build/bin/openshift %{buildroot}%{_datadir}/%{name}/linux/oc
install -p -m 755 _build/bin/darwin_amd64/openshift %{buildroot}%{_datadir}/%{name}/macosx/oc
install -p -m 755 _build/bin/windows_386/openshift.exe %{buildroot}%{_datadir}/%{name}/windows/oc.exe
#Install openshift pod
install -p -m 755 images/pod/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}/etc/%{name}/{master,node}
install -d -m 0755 %{buildroot}%{_unitdir}
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-master.service
install -m 0644 -t %{buildroot}%{_unitdir} rel-eng/openshift-node.service

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig
install -m 0644 rel-eng/openshift-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-master
install -m 0644 rel-eng/openshift-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/openshift-node

mkdir -p %{buildroot}%{_sharedstatedir}/%{name}

ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/oc
ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/oadm

install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/openshift-node-{guest,host}
install -m 0644 tuned/openshift-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/openshift-node-guest/
install -m 0644 tuned/openshift-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/openshift-node-host/
install -d -m 0755 %{buildroot}%{_mandir}/man7
install -m 0644 tuned/man/tuned-profiles-openshift-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-openshift-node.7

# Install sdn scripts
install -d -m 0755 %{buildroot}%{kube_plugin_path}
pushd _thirdpartyhacks/src/%{sdn_import_path}/ovssubnet/bin
   install -p -m 755 openshift-ovs-subnet %{buildroot}%{kube_plugin_path}/openshift-ovs-subnet
   install -p -m 755 openshift-sdn-kube-subnet-setup.sh %{buildroot}%{_bindir}/
popd
install -d -m 0755 %{buildroot}%{_prefix}/lib/systemd/system/openshift-node.service.d
install -p -m 0644 rel-eng/openshift-sdn-ovs.conf %{buildroot}%{_prefix}/lib/systemd/system/openshift-node.service.d/
install -d -m 0755 %{buildroot}%{_prefix}/lib/systemd/system/docker.service.d
install -p -m 0644 rel-eng/docker-sdn-ovs.conf %{buildroot}%{_prefix}/lib/systemd/system/docker.service.d/

# Install bash completions
install -d -m 755 %{buildroot}/etc/bash_completion.d/
install -p -m 644 rel-eng/completions/bash/* %{buildroot}/etc/bash_completion.d/

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/oc
%{_bindir}/oadm
%{_sharedstatedir}/%{name}
/etc/bash_completion.d/*

%files master
%defattr(-,root,root,-)
%{_unitdir}/openshift-master.service
%config(noreplace) %{_sysconfdir}/sysconfig/openshift-master
%config(noreplace) /etc/%{name}/master

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
%config(noreplace) /etc/%{name}/node

%post node
%systemd_post %{basename:openshift-node.service}

%preun node
%systemd_preun %{basename:openshift-node.service}

%postun node
%systemd_postun

%files sdn-ovs
%defattr(-,root,root,-)
%{_bindir}/openshift-sdn-kube-subnet-setup.sh
%{kube_plugin_path}/openshift-ovs-subnet
%{_prefix}/lib/systemd/system/openshift-node.service.d/openshift-sdn-ovs.conf
%{_prefix}/lib/systemd/system/docker.service.d/docker-sdn-ovs.conf

%files -n tuned-profiles-openshift-node
%defattr(-,root,root,-)
%{_prefix}/lib/tuned/openshift-node-host
%{_prefix}/lib/tuned/openshift-node-guest
%{_mandir}/man7/tuned-profiles-openshift-node.7*

%post -n tuned-profiles-openshift-node
recommended=`/usr/sbin/tuned-adm recommend`
if [[ "${recommended}" =~ guest ]] ; then
  /usr/sbin/tuned-adm profile openshift-node-guest > /dev/null 2>&1
else
  /usr/sbin/tuned-adm profile openshift-node-host > /dev/null 2>&1
fi

%preun -n tuned-profiles-openshift-node
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
* Tue Jun 30 2015 Scott Dodson <sdodson@redhat.com> 3.0.1.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #47 from bparees/sti_path (sdodson@redhat.com)
- Merge pull request #3343 from deads2k/add-old-config-test
  (dmcphers+openshiftbot@redhat.com)
- new-app: Don't swallow error (kargakis@users.noreply.github.com)
- Merge pull request #3499 from deads2k/fix-example-rc
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3478 from spadgett/fromimage-url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3490 from spadgett/deployment-flicker
  (dmcphers+openshiftbot@redhat.com)
- make test-cmd RCs non-conflicting (deads@redhat.com)
- bump(github.com/openshift/source-to-
  image):358cdb59db90b920e90a5f9a952ef7a3e11df3ad (bparees@redhat.com)
- Merge pull request #3488 from ironcladlou/remove-beta1-deployment-mappings
  (dmcphers+openshiftbot@redhat.com)
- OSE help links (jliggitt@redhat.com)
- Merge pull request #3443 from stevekuznetsov/skuznets/reorganize-master
  (dmcphers+openshiftbot@redhat.com)
- Avoid warning icon flicker on browse deployments page (spadgett@redhat.com)
- Remove v1beta1 annotation/label mapping (ironcladlou@gmail.com)
- Merge pull request #3473 from bparees/filter_pods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3477 from mfojtik/fix-template
  (dmcphers+openshiftbot@redhat.com)
- Reorganized master code (steve.kuznetsov@gmail.com)
- Merge pull request #3471 from spadgett/create-page-panels
  (dmcphers+openshiftbot@redhat.com)
- Use relative URIs in create flow links (spadgett@redhat.com)
- filter the pods the build controller looks at (bparees@redhat.com)
- Merge pull request #3472 from bparees/output_event
  (dmcphers+openshiftbot@redhat.com)
- Tweak headings and add separator on create page (spadgett@redhat.com)
- Make \w special character in template expression behave like PCRE
  (mfojtik@redhat.com)
- Merge pull request #3200 from fabianofranz/bugs_1223252
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3469 from spadgett/show-project-name
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3454 from ncdc/fix-registry-auth-for-pruning
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3462 from ironcladlou/test-cmd-deployment-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3378 from csrwng/newapp_multiple_service_ports
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3433 from spadgett/relist-delay
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3325 from fabianofranz/cli_by_example_script
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3465 from spadgett/start-build-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3458 from smarterclayton/summit_demo
  (dmcphers+openshiftbot@redhat.com)
- log event for invalid output error (bparees@redhat.com)
- Bug 1223252: UPSTREAM: label: Invalidate empty or invalid value labels
  (kargakis@users.noreply.github.com)
- Introduces auto-generation of CLI documents (contact@fabianofranz.com)
- Show namespace on project settings page (spadgett@redhat.com)
- Merge pull request #3401 from bparees/whitelist
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3348 from deads2k/build-omitempty-fields
  (dmcphers+openshiftbot@redhat.com)
- Newapp: expose multiple ports in generated services (cewong@redhat.com)
- Add API version and kind to POST request content (spadgett@redhat.com)
- Merge pull request #3455 from ironcladlou/reduce-logging
  (dmcphers+openshiftbot@redhat.com)
- Fix deployment scaling race in test-cmd.sh (ironcladlou@gmail.com)
- allow http proxy env variables to be set in privileged sti container
  (bparees@redhat.com)
- More robust websocket error handling (spadgett@redhat.com)
- Correct registry auth for pruning (agoldste@redhat.com)
- Add image pruning e2e test (agoldste@redhat.com)
- require required fields in build objects (deads@redhat.com)
- remove dead build etcd package (deads@redhat.com)
- Merge pull request #3456 from spadgett/safari-tiles
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3453 from soltysh/issue3391
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3265 from mfojtik/bz-1232694
  (dmcphers+openshiftbot@redhat.com)
- Allow private network to be used locally (ccoleman@redhat.com)
- Adjust examples indentation (contact@fabianofranz.com)
- Adjust 'kubectl config' examples to 'oc config' (contact@fabianofranz.com)
- Updated bash completion files (contact@fabianofranz.com)
- UPSTREAM: fixes kubectl config set-credentials examples
  (contact@fabianofranz.com)
- Merge pull request #3440 from ironcladlou/test-cmd-artifacts
  (dmcphers+openshiftbot@redhat.com)
- Remove `width: 100%%` from tile class (spadgett@redhat.com)
- Merge pull request #3431 from deads2k/security-allocator-warning
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3430 from spadgett/long-words
  (dmcphers+openshiftbot@redhat.com)
- Remove some pointless deployment logging (ironcladlou@gmail.com)
- Merge pull request #3439 from liggitt/verify_generated_content
  (dmcphers+openshiftbot@redhat.com)
- Issue 3391 - allow optional image output for custom builder.
  (maszulik@redhat.com)
- Correct Docker image Config type (agoldste@redhat.com)
- Merge pull request #3312 from ironcladlou/always-canary
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3400 from deads2k/add-contains-relationships
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3419 from stevekuznetsov/skuznets/issue/3375
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3425 from pravisankar/fix-typos
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3445 from deads2k/kill-internal-json-tags
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3434 from ncdc/dockerimage-config-should-be-a-pointer
  (dmcphers+openshiftbot@redhat.com)
- Use a predictable tempdir naming convention (ironcladlou@gmail.com)
- add pod and rc spec nodes (deads@redhat.com)
- Fix typos in the repo (rpenta@redhat.com)
- elminate internal json tags forever (deads@redhat.com)
- Added support for multiple roles in oc secrets add
  (steve.kuznetsov@gmail.com)
- Correct Docker image Config type (agoldste@redhat.com)
- Merge pull request #3397 from jhadvig/rhel_origin-base
  (dmcphers+openshiftbot@redhat.com)
- add warning for missing security allocator (deads@redhat.com)
- Merge pull request #3429 from rhcarvalho/fix-typo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3352 from smarterclayton/not_beta
  (dmcphers+openshiftbot@redhat.com)
- Add verification of generated content to make test (jliggitt@redhat.com)
- Merge pull request #3340 from stevekuznetsov/fix-root-route
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3385 from liggitt/who_can_all_namespaces
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3437 from deads2k/loosen-conversions
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3428 from deads2k/fix-template
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3422 from deads2k/validate-extended-params
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3098 from mnagy/warn_on_emptydir
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3387 from bparees/godeps
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3350 from smarterclayton/more_documentation
  (dmcphers+openshiftbot@redhat.com)
- Fixed routes from root (steve.kuznetsov@gmail.com)
- Merge pull request #3275 from deads2k/secret-usage-errors
  (dmcphers+openshiftbot@redhat.com)
- Deflake TestBasicGroupManipulation (jliggitt@redhat.com)
- Allow who-can to check cluster-level access (jliggitt@redhat.com)
- make conversions convert and validation validate (deads@redhat.com)
- Always do a canary check during deployment (ironcladlou@gmail.com)
- Merge pull request #3398 from ncdc/e2e-docker-selinux
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3412 from csrwng/newapp_dockerfileresolver
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3389 from liggitt/split_asset_package
  (dmcphers+openshiftbot@redhat.com)
- More e2e-docker fixes (agoldste@redhat.com)
- Prevent long unbroken words from extending outside tile boundaries
  (spadgett@redhat.com)
- Fix typo (rhcarvalho@gmail.com)
- wait for imagestreamtags before requesting them (deads@redhat.com)
- validate extended args in config (deads@redhat.com)
- Merge pull request #3293 from spadgett/template-catalog-updates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3289 from pravisankar/volume-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3328 from pravisankar/env-fixes
  (dmcphers+openshiftbot@redhat.com)
- Warn user when using EmptyDir volumes (nagy.martin@gmail.com)
- Add EmptyDir volumes for new-app containers (nagy.martin@gmail.com)
- Clean up residual volume mounts from e2e-docker (agoldste@redhat.com)
- Newapp: Allow Docker FROM in Dockerfile to point to an image stream or
  invalid image (cewong@redhat.com)
- Split java console into separate package (jliggitt@redhat.com)
- bump(github.com/elazarl/go-bindata-assetfs):
  3dcc96556217539f50599357fb481ac0dc7439b9 (jliggitt@redhat.com)
- Merge pull request #3405 from deads2k/be-calm-on-failed-deletes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3384 from pravisankar/whocan-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3201 from csrwng/newapp_dockerfile_ports
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3287 from kargakis/dont-create-spec-tags-on-output-stream
  (dmcphers+openshiftbot@redhat.com)
- Use correct images for test-cmd (agoldste@redhat.com)
- Use correct images for e2e-docker (agoldste@redhat.com)
- Fix volume dir label for e2e-docker (agoldste@redhat.com)
- Merge pull request #3382 from deads2k/refactor-graph
  (dmcphers+openshiftbot@redhat.com)
- split graph package (deads@redhat.com)
- Merge pull request #3379 from liggitt/bash_completion
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3349 from spadgett/build-completion-timestamp
  (dmcphers+openshiftbot@redhat.com)
- Adding rhel7 based origin-base Dockerfile (j.hadvig@gmail.com)
- Merge pull request #3386 from bparees/doc (dmcphers+openshiftbot@redhat.com)
- dockercfg secret controllers shouldn't fail on NotFound deletes
  (deads@redhat.com)
- Merge pull request #3369 from pmorie/selinux-disable
  (dmcphers+openshiftbot@redhat.com)
- Update bash autocompletions (jliggitt@redhat.com)
- bump(github.com/spf13/cobra): a8f7f3dc25e03593330100563f6c392224221899
  (jliggitt@redhat.com)
- bump(github.com/spf13/pflag): 381cb823881391d5673cf2fc41e38feba8a8e49a
  (jliggitt@redhat.com)
- Merge pull request #3188 from mfojtik/fix-etcd
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3388 from liggitt/imagemin
  (dmcphers+openshiftbot@redhat.com)
- Remove imagemin step from asset build (jliggitt@redhat.com)
- Merge pull request #3288 from liggitt/deflake
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/source-to-
  image):358cdb59db90b920e90a5f9a952ef7a3e11df3ad (bparees@redhat.com)
- sti to s2i (bparees@redhat.com)
- Merge pull request #3373 from liggitt/test_assets
  (dmcphers+openshiftbot@redhat.com)
- Show namespace, verb, resource for 'oadm policy who-can' cmd
  (rpenta@redhat.com)
- 'oc volume' test cases and fixes (rpenta@redhat.com)
- Updated 'oc env' help message (rpenta@redhat.com)
- Merge pull request #3258 from liggitt/service_port_validation
  (dmcphers+openshiftbot@redhat.com)
- Add policy cache wait in build admission integration tests
  (jliggitt@redhat.com)
- Template and image catalog updates (spadgett@redhat.com)
- Merge pull request #3380 from ncdc/gh3374 (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3337 from deads2k/protect-request-project
  (dmcphers+openshiftbot@redhat.com)
- defend against new-project racers (deads@redhat.com)
- Remove error when tracking tag target isn't found (agoldste@redhat.com)
- New-app: Expose ports specified in source Dockerfile (cewong@redhat.com)
- Add asset failure debugging (jliggitt@redhat.com)
- Make emptyDir work when SELinux is disabled (pmorie@gmail.com)
- Merge pull request #3341 from bparees/build_event
  (dmcphers+openshiftbot@redhat.com)
- Not beta! (ccoleman@redhat.com)
- fixed bash error (mturansk@redhat.com)
- fixed typo in script name (mturansk@redhat.com)
- fixed plugin init (mturansk@redhat.com)
- Add descriptions to our core objects and fix typos (ccoleman@redhat.com)
- Filter builds by completion rather than creation time on overview page
  (spadgett@redhat.com)
- add test for old config compatibility (deads@redhat.com)
- only handleErr on last retry failure (bparees@redhat.com)
- Now at one dot oh (ccoleman@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- bump(github.com/openshift/openshift-
  sdn/ovssubnet):cdd9955dc602abe8ef2d934a3c39417375c486c6 (rchopra@redhat.com)
- UPSTREAM: Ensure service account does not exist before deleting added/updated
  tokens (jliggitt@redhat.com)
- UPSTREAM: Add logging for invalid JWT tokens (jliggitt@redhat.com)
- use cookies for sticky sessions on http based routes (pweil@redhat.com)
- bump(github.com/openshift/openshift-
  sdn/ovssubnet):2bf8606dd9e0d5c164464f896e2223431f4b5099 (rchopra@redhat.com)
- Minor fixup to profiling instructions (ccoleman@redhat.com)
- Chmod hack/release.sh (ccoleman@redhat.com)
- Update logo and page title in JVM console for OSE (slewis@fusesource.com)
- Merge pull request #3301 from liggitt/debug_tokens
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Ensure service account does not exist before deleting added/updated
  tokens (jliggitt@redhat.com)
- UPSTREAM: Add logging for invalid JWT tokens (jliggitt@redhat.com)
- Merge pull request #3300 from pweil-/router-cookies
  (dmcphers+openshiftbot@redhat.com)
- Make etcd example more resilient to failure (mfojtik@redhat.com)
- new-app: Don't set spec.tags in output streams
  (kargakis@users.noreply.github.com)
- Bug 1232694 - Make the secret volume for push/pull secrets unique
  (mfojtik@redhat.com)
- use cookies for sticky sessions on http based routes (pweil@redhat.com)
- bump(github.com/openshift/openshift-
  sdn/ovssubnet):2bf8606dd9e0d5c164464f896e2223431f4b5099 (rchopra@redhat.com)
- Minor fixup to profiling instructions (ccoleman@redhat.com)
- Chmod hack/release.sh (ccoleman@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3284 from pmorie/emptydir-security-context
  (dmcphers@redhat.com)
- Merge pull request #3278 from smarterclayton/fix_deep_copy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3277 from liggitt/cert_hostnames
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3266 from smarterclayton/improve_perf_debugging
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3187 from markturansky/recyc_image
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3273 from liggitt/redirect_uri_validation
  (dmcphers@redhat.com)
- UPSTREAM: fix emptyDir idempotency bug (pmorie@gmail.com)
- Merge pull request #3276 from ncdc/fix-expired-hub-token
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3271 from csrwng/dockerfile_fix
  (dmcphers+openshiftbot@redhat.com)
- DeepCopy for util.StringSet (ccoleman@redhat.com)
- UPSTREAM: Make util.Empty public (ccoleman@redhat.com)
- UPSTREAM: use api.Scheme.DeepCopy() (ccoleman@redhat.com)
- Merge pull request #3269 from deads2k/stop-casting-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3263 from markturansky/patch_recyc_config
  (dmcphers+openshiftbot@redhat.com)
- Update default hostnames in cert (jliggitt@redhat.com)
- changed Recycler config for OS and added custom script for scrubbing in
  origin image (mturansk@redhat.com)
- Remove cached docker client repo on error (agoldste@redhat.com)
- update secret commands to give usage errors (deads@redhat.com)
- eliminate extra policy casting (deads@redhat.com)
- make policy interfaces (deads@redhat.com)
- switch policy types to map to pointers (deads@redhat.com)
- Prevent local fragment from being sent to a remote server
  (jliggitt@redhat.com)
- Properly handle Dockerfile build with no newline at the end
  (cewong@redhat.com)
- Validate redirect_uri doesn't contain path traversals (jliggitt@redhat.com)
- Add documentation on how to profile OpenShift (ccoleman@redhat.com)
- UPSTREAM: Allow recyclers to be configurable (mturansk@redhat.com)
- Merge pull request #3259 from liggitt/request_timeout_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3238 from liggitt/field_mappings
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3250 from csrwng/newapp_stream_tag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3225 from derekwaynecarr/cherry_pick_9765
  (dmcphers+openshiftbot@redhat.com)
- Re-enable timeout of -1 (no timeout) (jliggitt@redhat.com)
- UPSTREAM: Handle SecurityContext correctly for emptyDir volumes
  (pmorie@gmail.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- UPSTREAM: add client field mappings for v1 (jliggitt@redhat.com)
- UPSTREAM: Validate port protocol case strictly (jliggitt@redhat.com)
- Merge pull request #3251 from deads2k/suppress-check-errs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3252 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3255 from spadgett/uppercase-protocol
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3248 from deads2k/stop-changes-to-build-spec
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3249 from ncdc/exec-infinite-loop
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/openshift-
  sdn/ovssubnet):962bcbc2400f6e66e951e61ba259e81a6036f1a2 (rchopra@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3218 from pweil-/ipfailover-sa
  (dmcphers+openshiftbot@redhat.com)
- Use uppercase protocol when creating from source in Web Console
  (spadgett@redhat.com)
- allow some resources to be created while the namespace is terminating
  (deads@redhat.com)
- Newapp: preserve tag specified in image stream input (cewong@redhat.com)
- Merge pull request #3241 from ironcladlou/strategy-logging-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3240 from spadgett/pod-template-width
  (dmcphers+openshiftbot@redhat.com)
- add buildUpdate validation to protect spec (deads@redhat.com)
- UPSTREAM: fix exec infinite loop (agoldste@redhat.com)
- Merge pull request #3239 from liggitt/registry_rolling
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3151 from csrwng/build_admission
  (dmcphers+openshiftbot@redhat.com)
- Improve deployment strategy logging (ironcladlou@gmail.com)
- Set min-width on pod-template-block (spadgett@redhat.com)
- Merge pull request #3224 from smarterclayton/set_cache_control
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3221 from spadgett/fix-build-deployment-filters
  (dmcphers+openshiftbot@redhat.com)
- Update registry/router to use rolling deployments (jliggitt@redhat.com)
- Add admission controller for build strategy policy check (cewong@redhat.com)
- Add admission controller for build strategy policy check (cewong@redhat.com)
- Merge pull request #3232 from smarterclayton/make_population_parameterizable
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3229 from sg00dwin/word-break
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3228 from deads2k/fix-process-error-handling
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3210 from nak3/proxyfromenvironment
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3204 from abhgupta/agupta-deploy1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3205 from smarterclayton/impose_qps
  (dmcphers+openshiftbot@redhat.com)
- Population tuning should be parameterizable (ccoleman@redhat.com)
- Merge pull request #3213 from spadgett/pods-restart-policy
  (dmcphers+openshiftbot@redhat.com)
- Utility class for word-break. Fix for
  https://github.com/openshift/origin/issues/2560 (sgoodwin@redhat.com)
- prevent panic in oc process error handling (deads@redhat.com)
- UPSTREAM Fix bug where network container could be torn down before other pods
  (decarr@redhat.com)
- Set cache control headers for protected requests (ccoleman@redhat.com)
- Merge pull request #3211 from soltysh/fix_msg
  (dmcphers+openshiftbot@redhat.com)
- Update build and deployment config associations when filter changes
  (spadgett@redhat.com)
- Merge pull request #3207 from pmorie/e2e-fix
  (dmcphers+openshiftbot@redhat.com)
- add service account to ipfailover (pweil@redhat.com)
- Impose a high default QPS and rate limit (ccoleman@redhat.com)
- Show correct restart policy on browse pods page (spadgett@redhat.com)
- Fixed the message about image being pushed with authorization
  (maszulik@redhat.com)
- Set some timeout values explicitly to http.Transport
  (nakayamakenjiro@gmail.com)
- Set http.Transport to get proxy from environment (nakayamakenjiro@gmail.com)
- Merge pull request #3166 from liggitt/registry_auth
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2960 from markturansky/ceph_patch
  (dmcphers+openshiftbot@redhat.com)
- Fix syntax error in e2e (pmorie@gmail.com)
- Adding validatiions for deployment config LatestVersion  - LatestVersion
  cannot be negative  - LatestVersion cannot be decremented  - LatestVersion
  can be incremented by only 1 (abhgupta@redhat.com)
- Deflake TestServiceAccountAuthorization (jliggitt@redhat.com)
- Only challenge for errors that can be fixed by authorizing
  (jliggitt@redhat.com)
- Add registry auth tests, fix short-circuit (jliggitt@redhat.com)
- Clean up orphaned deployers and use pod watches (ironcladlou@gmail.com)
- Merge pull request #3191 from deads2k/fix-new-app-error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3203 from liggitt/create_api_client_basename
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3199 from bparees/readiness
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3195 from bparees/db_service_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3023 from pravisankar/osc-volume
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3194 from smarterclayton/formalize_release
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3189 from gashcrumb/styling-fixes
  (dmcphers+openshiftbot@redhat.com)
- Make base filename configurable for create-api-client-config
  (jliggitt@redhat.com)
- Merge pull request #3186 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Fix ReadinessProbe: seperate readiness and liveness in the code
  (bparees@redhat.com)
- Merge pull request #3185 from sg00dwin/spin-icon-waiting
  (dmcphers+openshiftbot@redhat.com)
- make service name a parameter (bparees@redhat.com)
- fix new-app errors (deads@redhat.com)
- OpenShift CLI cmd for volumes (rpenta@redhat.com)
- Change all references from openshift3_beta to openshift3 (sdodson@redhat.com)
- Create a formal release script (ccoleman@redhat.com)
- Updates to address part of bug 1230483 (slewis@fusesource.com)
- Specify seperate icon for 'pending' state vs 'running' state Add browse
  screenshots for Ashesh (sgoodwin@redhat.com)
- Setting NodeSelector on deployer/hook pods (abhgupta@redhat.com)
- UPSTREAM: Add CephFS volume plugin (mturansk@redhat.com)

* Mon Jun 15 2015 Scott Dodson <sdodson@redhat.com> 0.6.1.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3133 from deads2k/change-cluster-reader
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3095 from derekwaynecarr/cherry_pick_9361
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3113 from derekwaynecarr/data_population
  (dmcphers+openshiftbot@redhat.com)
- make a non-escalating policy resource group (deads@redhat.com)
- Merge pull request #2868 from pweil-/scc-admission
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Don't pretty print by default (decarr@redhat.com)
- Export command to template should let me provide template name
  (decarr@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3156 from derekwaynecarr/data_pop_scripts
  (dmcphers+openshiftbot@redhat.com)
- feedback for nodes group, namespace name check and weights (pweil@redhat.com)
- Merge pull request #3173 from liggitt/configurable_router_suffix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3171 from smarterclayton/test_docker_container
  (dmcphers+openshiftbot@redhat.com)
- Add routingConfig.subdomain to master config (jliggitt@redhat.com)
- Test the origin docker container (ccoleman@redhat.com)
- Merge pull request #3155 from pmorie/upstream-expand
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3168 from smarterclayton/search_nsenter_paths
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3142 from aveshagarwal/master-build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3101 from derekwaynecarr/bug_1230481
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3161 from kargakis/use-already-generated-path
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3164 from kargakis/doc-fixes
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: search for mount binary in hostfs (ccoleman@redhat.com)
- Update readme (ccoleman@redhat.com)
- Merge pull request #3158 from liggitt/admission_error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3160 from liggitt/openshift_client_name
  (dmcphers+openshiftbot@redhat.com)
- doc: Sync with master (kargakis@users.noreply.github.com)
- Change system:openshift-client username to system:master
  (jliggitt@redhat.com)
- config: Use already existing path (kargakis@users.noreply.github.com)
- Make admission errors clearer (jliggitt@redhat.com)
- Simplify createProvidersFromConstraints, add non-mutating test
  (jliggitt@redhat.com)
- Tag and push s2i images to local registry too (sdodson@redhat.com)
- UPSTREAM Rate limit scheduler to bind pods burst qps (decarr@redhat.com)
- More updates to data population scripts (decarr@redhat.com)
- Add non-mutating test (jliggitt@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3148 from smarterclayton/update_readme
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Expand variables in containers' env, cmd, args (pmorie@gmail.com)
- Merge pull request #3147 from spadgett/fix-get-started
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3122 from ironcladlou/rollback-disable-triggers
  (dmcphers+openshiftbot@redhat.com)
- Update readme (ccoleman@redhat.com)
- Merge pull request #3140 from csrwng/fix_expose_msg
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3134 from spadgett/validate-app-name
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3090 from ncdc/custom-build-use-push-secrets
  (dmcphers+openshiftbot@redhat.com)
- Don't show "Get Started" message when project has replication controllers
  (spadgett@redhat.com)
- scc admission (pweil@redhat.com)
- router as restricted uid (pweil@redhat.com)
- UPSTREAM: work with SC copy, error formatting, add GetSCCName to provider
  (pweil@redhat.com)
- Merge pull request #3138 from derekwaynecarr/data_pop_scripts
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3011 from stevekuznetsov/skuznets/issue/2951
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3112 from smarterclayton/make_containerized_kubelet
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3139 from derekwaynecarr/cherry_pick_9716
  (dmcphers+openshiftbot@redhat.com)
- Disable image triggers on rollback (ironcladlou@gmail.com)
- Merge pull request #3057 from csrwng/newapp_code_detect
  (dmcphers+openshiftbot@redhat.com)
- Initial scripts for data population (decarr@redhat.com)
- Added parsing for new secret sources to enable naming source keys.
  (steve.kuznetsov@gmail.com)
- Support a containerized node (ccoleman@redhat.com)
- Merge pull request #3130 from liggitt/omitempty_volumesources
  (dmcphers+openshiftbot@redhat.com)
- Fix command help for expose service (cewong@redhat.com)
- Use stricter name validation creating from source repository
  (spadgett@redhat.com)
- UPSTREAM its bad to spawn a gofunc per quota with large number of quotas
  (decarr@redhat.com)
- UPSTREAM: add simple variable expansion (pmorie@gmail.com)
- UPSTREAM: nsenter path should be relative (ccoleman@redhat.com)
- Build fixes. (avagarwa@redhat.com)
- Newapp: Remove hard coded image names for code detection (cewong@redhat.com)
- Merge pull request #3103 from ncdc/image-policy-changes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3100 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3125 from liggitt/markup_test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3127 from bparees/templates
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Add omitempty to RBD/ISCSI volume sources (jliggitt@redhat.com)
- Merge pull request #3104 from deads2k/change-to-kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3124 from spadgett/no-dep-config
  (dmcphers+openshiftbot@redhat.com)
- update db templates to use imagestreams in openshift project
  (bparees@redhat.com)
- Cleaning up deployments on failure  - The failed deployment is scaled to 0  -
  The last completed deployment is scaled back up  - The cleanup is done before
  updating the deployment status to Failed  - A failure to clean up results in
  a transient error that is retried indefinitely (abhgupta@redhat.com)
- Test for and fix invalid markup nesting (jliggitt@redhat.com)
- Show "no deployments" message on browse page consistently
  (spadgett@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3068 from smarterclayton/better_message
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3117 from liggitt/registry_router_service_account
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3084 from smarterclayton/incorrectnewapp_guard
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3105 from smarterclayton/no_ready_notification
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3094 from fabianofranz/bugs_1229555
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3018 from soltysh/bug1229642
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3115 from pmorie/emptydir-nonroot-2
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3111 from pravisankar/fix-oc-env
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3107 from smarterclayton/topology_div
  (dmcphers+openshiftbot@redhat.com)
- Display start time instead of IP when not present (ccoleman@redhat.com)
- Merge pull request #3078 from markturansky/thread_leak_fix
  (dmcphers+openshiftbot@redhat.com)
- Bug 1229642 - new-build generates ImageStreams along with BuildConfig when
  needed. (maszulik@redhat.com)
- Allow setting service account in oadm registry/router (jliggitt@redhat.com)
- UPSTREAM: validate service account name in pod.spec (jliggitt@redhat.com)
- UPSTREAM: EmptyDir volumes for non-root 2/2 (pmorie@gmail.com)
- Merge pull request #3108 from markturansky/recyc_thread_leak
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3083 from smarterclayton/fix_kubectl_run_container
  (dmcphers+openshiftbot@redhat.com)
- Fix broken --filename for oc env cmd (rpenta@redhat.com)
- Merge pull request #3106 from ironcladlou/initial-deploy-failure-event
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3096 from ironcladlou/deployment-label-correlation
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Added stop channel to prevent thread leak from watch
  (mturansk@redhat.com)
- app: Add element around overview on project.html page (stefw@redhat.com)
- Record events for initial deployment failures (ironcladlou@gmail.com)
- Node was blocking rather than running in goroutine (ccoleman@redhat.com)
- Merge pull request #2867 from csrwng/newapp_dup_comp_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2922 from mnagy/db_empty_dir
  (dmcphers+openshiftbot@redhat.com)
- change .config/openshift to .kube (deads@redhat.com)
- Image push/pull policy updates (agoldste@redhat.com)
- change OPENSHIFTCONFIG to KUBECONFIG (deads@redhat.com)
- Bug 1229555 - fix broken example in oc new-app (contact@fabianofranz.com)
- Fix application template double escaping (decarr@redhat.com)
- Added stop channel to watched pods to prevent thread leak
  (mturansk@redhat.com)
- Use line-height: 1.3 for component headers (spadgett@redhat.com)
- Correlate and retrieve deployments by label (ironcladlou@gmail.com)
- Expose upstream generators and fix kubectl help (ccoleman@redhat.com)
- Support push secrets in the custom builder (agoldste@redhat.com)
- Prevent invalid names in new-app (cewong@redhat.com)
- Add emptyDir volumes to database templates (nagy.martin@gmail.com)
- Component refs are not docker image refs (ccoleman@redhat.com)
- UPSTREAM: Run had invalid arguments (ccoleman@redhat.com)

* Thu Jun 11 2015 Scott Dodson <sdodson@redhat.com> 0.6.0.1
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #3082 from liggitt/namespace
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3006 from spadgett/spec-tags-annotations
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2892 from ironcladlou/rolling-canary
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3080 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Update README with create flow improvements (ccoleman@redhat.com)
- Merge pull request #3043 from jhadvig/image_readme
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: kill namespace defaulting (deads@redhat.com)
- Make tests more explicit (jliggitt@redhat.com)
- Merge pull request #3046 from sdodson/origin-dynamic-docker-network-config
  (dmcphers+openshiftbot@redhat.com)
- fix accessReview integration tests (deads@redhat.com)
- Merge pull request #3077 from smarterclayton/add_ip_to_describe
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3045 from liggitt/conflict_check
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3081 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3079 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Implement deployment canary and fixes (ironcladlou@gmail.com)
- Upstream: node restart should not restart docker; fix node registration in
  vagrant (rchopra@redhat.com)
- Merge pull request #3069 from pweil-/stats-port
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3059 from bparees/update_streams
  (dmcphers+openshiftbot@redhat.com)
- Use better link for web console doc (dmcphers@redhat.com)
- Fix spelling errors (dmcphers@redhat.com)
- UPSTREAM: Add Pod IP to pod describe (ccoleman@redhat.com)
- Merge pull request #2984 from smarterclayton/fix_sdn_init
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3063 from deads2k/add-client-debug
  (dmcphers+openshiftbot@redhat.com)
- expose stats port based on config (pweil@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Upstream: make docker bridge configuration more dynamic (sdodson@redhat.com)
- UPSTREAM: add debug output for client calls (deads@redhat.com)
- Merge pull request #3060 from sdodson/bash-completion-regen
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3055 from smarterclayton/add_token_and_context_to_whoami
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3061 from fabianofranz/bugs_1230066
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3056 from smarterclayton/update_cadvisor
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3058 from smarterclayton/fix_rest_client_metrics_fanout
  (dmcphers+openshiftbot@redhat.com)
- s/openshift3_beta/openshift3 in the OSE build scripts (bleanhar@redhat.com)
- Merge pull request #3028 from jwhonce/wip/dns
  (dmcphers+openshiftbot@redhat.com)
- Bug 1230066 - fixes oc help to use public git repo (contact@fabianofranz.com)
- bump(github.com/google/cadvisor):0.15.0-4-ga36554f (ccoleman@redhat.com)
- update imagestream location for ga (bparees@redhat.com)
- Add .tag* to gitignore, files generated by atom ctag (sdodson@redhat.com)
- Update bash completion for osc/osadm -> oc/oadm (sdodson@redhat.com)
- Merge pull request #3052 from smarterclayton/race_during_crypto_serial
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3050 from gashcrumb/integration-bugfixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2969 from gabemontero/dev/gabemontero/issue/2740
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2958 from deads2k/add-to-sa
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3041 from kargakis/fix-example-in-oc-deploy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3040 from kargakis/minor-log-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3037 from pmorie/upstream-9384
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: rest_client logging is generating too many metrics
  (ccoleman@redhat.com)
- whoami should show the current token and context (ccoleman@redhat.com)
- Merge pull request #3034 from smarterclayton/make_gomaxprocs_precise
  (dmcphers+openshiftbot@redhat.com)
- Cert serial numbers must be unique per execution (ccoleman@redhat.com)
- Merge pull request #2919 from kargakis/fix-new-app-test
  (dmcphers+openshiftbot@redhat.com)
- * Implement input from review (jhonce@redhat.com)
- Update to openshift-jvm 1.0.19 (slewis@fusesource.com)
- Infrastructure - Add and call vm-provision-fixup.sh (jhonce@redhat.com)
- fixes to sample app related instructions (gmontero@redhat.com)
- Handle spec.tags where tag name has a dot (spadgett@redhat.com)
- Merge pull request #3025 from bparees/bad_sample
  (dmcphers+openshiftbot@redhat.com)
- Change error check from IsConflict to IsAlreadyExists (jliggitt@redhat.com)
- add attach secret command (deads@redhat.com)
- deploy: Fix example and usage message (kargakis@users.noreply.github.com)
- Update README for images (j.hadvig@gmail.com)
- new-app: Fix empty tag in logs (kargakis@users.noreply.github.com)
- UPSTREAM: Support emptydir volumes for containers running as non-root
  (pmorie@gmail.com)
- Merge pull request #3033 from smarterclayton/add_parallel_test
  (dmcphers+openshiftbot@redhat.com)
- Fix ose image issues introduced by bad merge resolution (sdodson@redhat.com)
- Deflake integration test for overwrite policy (ccoleman@redhat.com)
- Merge pull request #3015 from stevekuznetsov/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Allow GOMAXPROCS to be customized for openshift (ccoleman@redhat.com)
- Add test case for parallel.Run (ccoleman@redhat.com)
- SDN should signal when it's ready for pods to run (ccoleman@redhat.com)
- UPSTREAM: Allow OSDN to signal ready (ccoleman@redhat.com)
- UPSTREAM: Allow pod start to be delayed in Kubelet (ccoleman@redhat.com)
- Parallelize cert creation and test integration (ccoleman@redhat.com)
- Merge pull request #3027 from smarterclayton/update_go_etcd
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3024 from bparees/custom_builder_error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2777 from liggitt/build_pod_user
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/coreos/go-etcd/etcd):4cceaf7283b76f27c4a732b20730dcdb61053bf5
  (ccoleman@redhat.com)
- Merge pull request #2891 from markturansky/recycler_to_master
  (dmcphers+openshiftbot@redhat.com)
- fix bad custom template (bparees@redhat.com)
- better error message for invalid source uri in sample custom builder
  (bparees@redhat.com)
- changed hard-coded time to flag coming from CLI (mturansk@redhat.com)
- Added PVRecycler controller to master (mturansk@redhat.com)
- Updated example README (steve.kuznetsov@gmail.com)
- Use service accounts for build controller, deployment controller, and
  replication controller (jliggitt@redhat.com)
- new-app: Fix detectSource test (kargakis@users.noreply.github.com)

* Tue Jun 09 2015 Scott Dodson <sdodson@redhat.com> 0.6.0.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2952 from stevekuznetsov/skuznets/issue/1902
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3021 from ncdc/oc-tag-create-target-stream
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3019 from liggitt/deployer
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2963 from derekwaynecarr/cherry_pick_9032
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2714 from csrwng/insecure_registry
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3017 from markturansky/patch_9282
  (dmcphers+openshiftbot@redhat.com)
- 'oc tag': create target stream if not found (agoldste@redhat.com)
- Update e2e test for new get pod output (decarr@redhat.com)
- Merge pull request #3014 from ncdc/resolve-short-isimage-refs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3010 from deads2k/force-update-validation
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Decrease columns and refactor get pods layout (decarr@redhat.com)
- Support insecure registries (cewong@redhat.com)
- Fix deployer log message (jliggitt@redhat.com)
- Merge pull request #3012 from markturansky/patch_8530
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2966 from bparees/rest_errors
  (dmcphers+openshiftbot@redhat.com)
- Expand short isimage id refs (agoldste@redhat.com)
- UPSTREAM: Add available volumes to index when not present
  (mturansk@redhat.com)
- Merge pull request #2866 from deads2k/create-osc-secrets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2967 from ncdc/add-tag-command
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3009 from markturansky/patch_9069
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3004 from markturansky/patch_8732
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2944 from childsb/patch-1
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: GCEPD mounting on Atomic (mturansk@redhat.com)
- Adding a "/ready" endpoint for OpenShift (steve.kuznetsov@gmail.com)
- storage unit test to confirm validation (deads@redhat.com)
- add validateUpdate for API types (deads@redhat.com)
- add secrets subcommand to osc (deads@redhat.com)
- Add tag command (agoldste@redhat.com)
- UPSTREAM: Adds ISCSI to PV and fixes a nil pointer issue
  (mturansk@redhat.com)
- return valid http status objects on rest errors (bparees@redhat.com)
- UPSTREAM: Normalize and fix PV support across volumes (mturansk@redhat.com)
- UPSTREAM: kube: update describer for dockercfg secrets (deads@redhat.com)
- Update CONTRIBUTING.adoc (bchilds@redhat.com)

* Tue Jun 09 2015 Scott Dodson <sdodson@redhat.com> 0.5.4.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2997 from spadgett/obj-describer-1.0.1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2879 from markturansky/pv_recycler
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3003 from liggitt/qps (dmcphers+openshiftbot@redhat.com)
- Merge pull request #3002 from markturansky/patch_nfs
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2998 from deads2k/handle-update-methods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2995 from brenton/master
  (dmcphers+openshiftbot@redhat.com)
- Disable QPS for internal API client (jliggitt@redhat.com)
- UPSTREAM: Added PV support for NFS (mturansk@redhat.com)
- Merge pull request #2779 from kargakis/disable-update-patch-flag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2912 from smarterclayton/configure_controllers
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2953 from soltysh/issue2880
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2539 from stevekuznetsov/dev/skuznets/issue/2494
  (dmcphers+openshiftbot@redhat.com)
- update validation to handle validation argument reordering (deads@redhat.com)
- Merge pull request #2994 from ncdc/fix-prune-images-registry-override
  (dmcphers+openshiftbot@redhat.com)
- Update openshift-object-describer to 1.0.1 (spadgett@redhat.com)
- UPSTREAM: PV Recycling support (mturansk@redhat.com)
- Merge pull request #2988 from gashcrumb/bug-1229595
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2936 from kargakis/flag-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2989 from deads2k/update-policy-groups
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2961 from smarterclayton/show_spec_tags_in_is
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2565 from pweil-/stats-config
  (dmcphers+openshiftbot@redhat.com)
- update: Remove disabled --patch example (kargakis@users.noreply.github.com)
- Allow all Kubernetes controller arguments to be configured
  (ccoleman@redhat.com)
- UPSTREAM: Kubelet should log events at V(3) (ccoleman@redhat.com)
- Bug 1229731 - Updating docker rpm dependency in the specfile
  (bleanhar@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2978 from liggitt/avoid_build_mutation
  (dmcphers+openshiftbot@redhat.com)
- Add registry-url flag to prune images (agoldste@redhat.com)
- UPSTREAM: Improve signature consistency for ValidateObjectMetaUpdate
  (deads@redhat.com)
- Issue 2880 - allow debugging docker push command. (maszulik@redhat.com)
- remove imagerepository policy refs (deads@redhat.com)
- Update to openshift-jvm 1.0.18 (slewis@fusesource.com)
- Don't mutate build.spec when creating build pod (jliggitt@redhat.com)
- check name validation (deads@redhat.com)
- UPSTREAM: kube: expose name validation method (deads@redhat.com)
- make build config and instantiate consistent rest strategy (deads@redhat.com)
- expose: Bump validation and default flag message to routes
  (kargakis@users.noreply.github.com)
- Merge pull request #2934 from kargakis/better-commenting
  (dmcphers+openshiftbot@redhat.com)
- Mostly golinting (kargakis@users.noreply.github.com)
- Merge pull request #2983 from liggitt/fix_webhook_url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2563 from sg00dwin/failed-pod
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2972 from bparees/streamtags
  (dmcphers+openshiftbot@redhat.com)
- Fix webhook URL generation in UI (jliggitt@redhat.com)
- Merge pull request #2863 from pweil-/scc-bootstrap-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2965 from smarterclayton/add_abbrev_for_service_accounts
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2973 from spadgett/tooltip-typos
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2970 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- add version specific imagestream tags (bparees@redhat.com)
- add a default SCC for the cluster (pweil@redhat.com)
- Merge pull request #2911 from smarterclayton/default_kube_api
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2914 from smarterclayton/add_types_command
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2950 from bparees/buildlogs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2750 from rhcarvalho/issue-2594
  (dmcphers+openshiftbot@redhat.com)
- Fix typos in tooltips on create from source page (spadgett@redhat.com)
- Consider existing deployments correctly when creating a new deployment
  (abhgupta@redhat.com)
- Switch the default API to v1 (ccoleman@redhat.com)
- UPSTREAM: v1 Secrets/ServiceAccounts needed a conversion
  (ccoleman@redhat.com)
- UPSTREAM: Alter default Kubernetes API versions (ccoleman@redhat.com)
- Remove trailing periods from help (ccoleman@redhat.com)
- Add types command showing OpenShift concepts (ccoleman@redhat.com)
- bump(github.com/MakeNowJust/heredoc):1d91351acdc1cb2f2c995864674b754134b86ca7
  (ccoleman@redhat.com)
- Pod failed, pending, warning style updates along with several bug fixes,
  inclusion of meta tags, and removal of inline styles and unused classes.
  (sgoodwin@redhat.com)
- Merge pull request #2948 from smarterclayton/fix_nested_command_help
  (dmcphers+openshiftbot@redhat.com)
- Add shortcuts for service accounts (ccoleman@redhat.com)
- osc describe is should show spec tags when status empty (ccoleman@redhat.com)
- Merge pull request #2954 from bparees/missing_tag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2895 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2959 from spadgett/hawtio-patch (jordan@liggitt.net)
- Update bindata.go for modified hawtio-core-navigation 2.0.48
  (spadgett@redhat.com)
- Merge pull request #2920 from csrwng/remove_cors_headers
  (dmcphers+openshiftbot@redhat.com)
- return proper http status failures from get logs (bparees@redhat.com)
- Merge pull request #2942 from soltysh/fix_webhook_url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2930 from derekwaynecarr/cherry_pick_9358
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2908 from smarterclayton/configure_master
  (dmcphers+openshiftbot@redhat.com)
- lower log level of missing tag for image message (bparees@redhat.com)
- Deleting deployment hook pods after a deployment completes
  (abhgupta@redhat.com)
- Subcommands should not display additional commands (ccoleman@redhat.com)
- Improve consistency of build trigger types (rhcarvalho@gmail.com)
- Replace occurrences of Github -> GitHub (rhcarvalho@gmail.com)
- Merge pull request #2751 from deads2k/description-test
  (dmcphers+openshiftbot@redhat.com)
- Fixed webhook URLs (maszulik@redhat.com)
- Merge pull request #2928 from liggitt/node_glusterfs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2925 from mfojtik/secrets-namespace
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2644 from mnagy/db_persistent_volumes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2910 from smarterclayton/export
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2841 from mfojtik/gh-2680
  (dmcphers+openshiftbot@redhat.com)
- allow stats user/port/password to be configured (pweil@redhat.com)
- Merge pull request #2893 from smarterclayton/make_allocator_configurable
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2924 from spadgett/oc-rename
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2856 from pweil-/scc-upstream
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2860 from deads2k/promote-whoami
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2896 from bparees/delete
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2917 from sdminonne/upstream_patch
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Pass memory swap limit -1 by default (decarr@redhat.com)
- Add get endpoints permission to node role for glusterfs (jliggitt@redhat.com)
- Add an export command to oc (ccoleman@redhat.com)
- Add support for processing multiple templates (mfojtik@redhat.com)
- Fix message in extended tests fixture (mfojtik@redhat.com)
- Security Allocator is configurable (ccoleman@redhat.com)
- Implemented an interface to expose the PolicyCache List() and Get() methods
  for policies and bindings on project and cluster level to the project
  authorization cache. (steve.kuznetsov@gmail.com)
- Namespace all build secrets (mfojtik@redhat.com)
- Update `osc` to `oc` in Web Console help text (spadgett@redhat.com)
- updates for feedback (pweil@redhat.com)
- promote whoami to osc (deads@redhat.com)
- add api description unit test (deads@redhat.com)
- UPSTREAM: Remove CORS headers from pod proxy responses (cewong@redhat.com)
- New-app: fix message when language is not detected. (cewong@redhat.com)
- Add persistent volume claims to database example templates
  (nagy.martin@gmail.com)
- Fix time duration formatting in push retry (mfojtik@redhat.com)
- UPSTREAM: Disable --patch for kubectl update
  (kargakis@users.noreply.github.com)
- Add process from stdin to hack/test-cmd.sh (mfojtik@redhat.com)
- Support processing using standard input (mfojtik@redhat.com)
- UPSTREAM: adding downward api volume plugin (salvatore-
  dario.minonne@amadeus.com)
- Must not apply labels to nested objects (contact@fabianofranz.com)
- UPSTREAM: Split resource.AsVersionedObject (ccoleman@redhat.com)
- Merge pull request #2610 from fabianofranz/issues_2412
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: ToJSON must support support forcing a JSON syntax check
  (contact@fabianofranz.com)
- Allow more master options to be configurable (ccoleman@redhat.com)
- Refactor usage and help templates to better catch corner cases
  (ccoleman@redhat.com)
- Merge pull request #2907 from smarterclayton/fix_travis
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2905 from pmorie/registry-tree
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2904 from bparees/hello
  (dmcphers+openshiftbot@redhat.com)
- Fix travis builds (ccoleman@redhat.com)
- Merge pull request #2797 from derekwaynecarr/cherry_pick_9099
  (dmcphers+openshiftbot@redhat.com)
- Add binaries for find and tree to registry image (pmorie@gmail.com)
- build hello-pod statically (bparees@redhat.com)
- Merge pull request #2902 from liggitt/ui_watch_expire
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2901 from smarterclayton/generate_conversions
  (dmcphers+openshiftbot@redhat.com)
- Handle watch window expirations in UI (jliggitt@redhat.com)
- Generated conversions and deep copies (ccoleman@redhat.com)
- Make verify-generated-(deep-copies|conversion).sh required
  (ccoleman@redhat.com)
- Enable deep copy and conversions in OpenShift (ccoleman@redhat.com)
- UPSTREAM: Improve deep copy to work with OpenShift (ccoleman@redhat.com)
- Move conversions out of init() and give them names (ccoleman@redhat.com)
- Group commands in osc for ease of use (ccoleman@redhat.com)
- Merge pull request #2900 from smarterclayton/rename_cli
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2884 from liggitt/limit_secret_refs
  (dmcphers+openshiftbot@redhat.com)
- Rename 'osc' to 'os' and 'osadm' to 'oadm' (ccoleman@redhat.com)
- Merge pull request #2889 from ncdc/image-api-cleanup
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2887 from spadgett/overview-show-replicas
  (dmcphers+openshiftbot@redhat.com)
- Display current and desired replicas on overview page (spadgett@redhat.com)
- Image API cleanup (agoldste@redhat.com)
- Merge pull request #2837 from
  smarterclayton/allow_controllers_to_be_controlled
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2870 from skonzem/fix_doc_typos
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2840 from HyunsooKim1112/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2803 from ncdc/ignore-spec-tags-on-import
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2801 from aveshagarwal/master-spec-fixes
  (dmcphers+openshiftbot@redhat.com)
- never retry pod/build sync delete failures (bparees@redhat.com)
- Controllers should be able to be lazily started (ccoleman@redhat.com)
- Round trip service account correctly in buildconfig/build
  (jliggitt@redhat.com)
- UPSTREAM: Enable LimitSecretReferences in service account admission
  (jliggitt@redhat.com)
- Merge pull request #2830 from smarterclayton/remove_v1beta1
  (dmcphers+openshiftbot@redhat.com)
- Properly resolve tag of build.Output in conversion (ccoleman@redhat.com)
- Merge pull request #2894 from soltysh/fix_webhook
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2747 from soltysh/new-build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2875 from deads2k/osc-status
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2835 from smarterclayton/remove_unnecessary_prints
  (dmcphers+openshiftbot@redhat.com)
- Fixed the error from webhook when secret mismatch (maszulik@redhat.com)
- Merge pull request #2871 from derekwaynecarr/cherry_pick_9080
  (dmcphers+openshiftbot@redhat.com)
- Minor master cleanups (ccoleman@redhat.com)
- Remove v1beta1 (ccoleman@redhat.com)
- Merge pull request #2530 from liggitt/deployer_service_account
  (dmcphers+openshiftbot@redhat.com)
- Renamed --build flag to --strategy and fixed taking that flag into account
  even when repo has Dockerfile. (maszulik@redhat.com)
- Issue 2695 - new-build command creating just BuildConfig from provided source
  and/or image. (maszulik@redhat.com)
- Merge pull request #2878 from jwhonce/wip/volumes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2833 from smarterclayton/move_endpoints
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2787 from spadgett/create-flow-updates
  (dmcphers+openshiftbot@redhat.com)
- have osc status highlight disabled deployment configs (deads@redhat.com)
- Too much logging from cert creation (ccoleman@redhat.com)
- Merge pull request #2873 from pweil-/route-update-bug
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2874 from smarterclayton/upstream_metrics
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2872 from spadgett/settings-display-name
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2799 from deads2k/add-messages-to-remove-user
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2825 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Use deployer service account for deployments (jliggitt@redhat.com)
- UPSTREAM: process new service accounts (jliggitt@redhat.com)
- Merge pull request #2630 from sdodson/bash_completion_rpms
  (dmcphers+openshiftbot@redhat.com)
- Differentiate nodes volumes directories (jhonce@redhat.com)
- ensure route exists under one service only (pweil@redhat.com)
- Merge pull request #2806 from csrwng/fix_newapp_help
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: remove client facet from metrics (ccoleman@redhat.com)
- Merge pull request #2697 from csrwng/image_reference_generation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2839 from liggitt/ui_perf_fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2811 from stevekuznetsov/skuznets/removing-glog
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2847 from deads2k/remove-kube-v1beta1
  (dmcphers+openshiftbot@redhat.com)
- Show correct message on project settings when no display name
  (spadgett@redhat.com)
- UPSTREAM Insert 'svc' into the DNS search paths (decarr@redhat.com)
- add output for what remove-user is doing (deads@redhat.com)
- Fix typos in documentation (konzems@gmail.com)
- Fix vagrant setup so that minions are pre-registered; Update Godeps with
  latest openshift-sdn (rchopra@redhat.com)
- Merge pull request #2846 from deads2k/update-files-for-v1beta3
  (dmcphers+openshiftbot@redhat.com)
- Fix image reference generation for deployment configs (cewong@redhat.com)
- Remove TaskList polling (jliggitt@redhat.com)
- Merge pull request #2850 from ncdc/fix-node-rootdir
  (dmcphers+openshiftbot@redhat.com)
- Updates to create flow forms (spadgett@redhat.com)
- UPSTREAM: security context constraints (pweil@redhat.com)
- Merge pull request #2790 from jimmidyson/template-watches
  (dmcphers+openshiftbot@redhat.com)
- Ensure KubeletConfig's RootDirectory is correct (agoldste@redhat.com)
- Converted glog.Error() -> util.HandleError(errors.New()), glog.Errorf() ->
  util.HandleError(fmt.Errorf()) where code was eating the error.
  (steve.kuznetsov@gmail.com)
- Merge pull request #2836 from liggitt/project_request
  (dmcphers+openshiftbot@redhat.com)
- disable kube v1beta1 and v1beta2 by default (deads@redhat.com)
- update docs to remove v1beta1 (deads@redhat.com)
- Merge pull request #2844 from kargakis/test-fix
  (dmcphers+openshiftbot@redhat.com)
- Don't default displayname/description to project name (jliggitt@redhat.com)
- Support template watch (jimmidyson@gmail.com)
- Merge pull request #2782 from stefwalter/readme-start-developing
  (dmcphers+openshiftbot@redhat.com)
- reaper: Exit tests as soon as actions lengths don't match
  (kargakis@users.noreply.github.com)
- Merge pull request #2784 from kargakis/rem-ex-cmds
  (dmcphers+openshiftbot@redhat.com)
- README.md: Fix up the 'Start Developing' instructions (stefw@redhat.com)
- Update CONTRIBUTING.adoc (hyun.soo.kim1112@gmail.com)
- Merge pull request #2831 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Don't import if a spec.tag is tracking another (agoldste@redhat.com)
- Merge pull request #2829 from pweil-/only-reload-if-changed
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2717 from ncdc/image-prune-override-registry-url
  (dmcphers+openshiftbot@redhat.com)
- Expose _endpoints.<> as a DNS endpoint (ccoleman@redhat.com)
- Merge pull request #2816 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2826 from smarterclayton/update_swagger
  (dmcphers+openshiftbot@redhat.com)
- Fixing console link and adding link to new api doc (dmcphers@redhat.com)
- Merge pull request #2766 from deads2k/change-kubeconfig-keys
  (dmcphers+openshiftbot@redhat.com)
- do not bounce router for endpoint changes that do not change the data
  (pweil@redhat.com)
- Merge pull request #2694 from ironcladlou/deploy-trigger-test-cleanup
  (dmcphers+openshiftbot@redhat.com)
- Fixes to Contributing doc (dmcphers@redhat.com)
- Updated swagger doc (ccoleman@redhat.com)
- Merge pull request #2823 from ncdc/add-image-tracking-test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2796 from ncdc/import-image
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2810 from pmorie/serial-e2e
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2812 from csrwng/newapp_noport_msg
  (dmcphers+openshiftbot@redhat.com)
- Add test for tracking tags when ISMs are posted (agoldste@redhat.com)
- Merge pull request #2808 from gashcrumb/new-proxy-url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2753 from derekwaynecarr/v1_descriptions_oauth
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2804 from ironcladlou/cancel-cli-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2792 from ironcladlou/dc-reaper-reentrancy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2793 from pweil-/vagrant-volume-dir
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2718 from smarterclayton/support_all_kubelet_args
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2733 from kargakis/expose-validation
  (dmcphers+openshiftbot@redhat.com)
- Add 'osc import-image' command (agoldste@redhat.com)
- New-app: more helpful message when not generating a service
  (cewong@redhat.com)
- Merge pull request #2704 from csrwng/support_template_files
  (dmcphers+openshiftbot@redhat.com)
- Update proxy URL path (slewis@fusesource.com)
- Fix new-app help for app-from-image (cewong@redhat.com)
- Merge pull request #2802 from csrwng/fix_proxy_url_slash
  (dmcphers+openshiftbot@redhat.com)
- Add v1 descriptions to oauth (decarr@redhat.com)
- Fix device busy issues on serial e2e runs (pmorie@gmail.com)
- Merge pull request #2800 from deads2k/lower-trace-level
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2794 from derekwaynecarr/cherry_pick_9325
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2791 from sdodson/revert-sdn-kubelet-ordering
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2789 from deads2k/update-version-for-config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2788 from deads2k/fix-output
  (dmcphers+openshiftbot@redhat.com)
- Fix CLI deployment cancellation (ironcladlou@gmail.com)
- Make deployment config reaper reentrant (ironcladlou@gmail.com)
- don't share volumes directory in vagrant (pweil@redhat.com)
- Merge pull request #2744 from kargakis/specify-host-when-exposing
  (dmcphers+openshiftbot@redhat.com)
- Allow overriding registry URL when image pruning (agoldste@redhat.com)
- Minor spec file fixes. (avagarwa@redhat.com)
- UPSTREAM: Fix proxying of URLs that end in "/" in the pod proxy
  (cewong@redhat.com)
- UPSTREAM: make getFromCache more tolerant (deads@redhat.com)
- Allow the kubelet to take the full set of params (ccoleman@redhat.com)
- Adding 'openshift.io/' namespace to 'displayName', 'description' annotations.
  Added annotations to api/types.go and updated references.
  (steve.kuznetsov@gmail.com)
- UPSTREAM: Support kubelet initialization in order (ccoleman@redhat.com)
- UPSTREAM Fix error in quantity code conversion (decarr@redhat.com)
- UPSTREAM Fix namespace controller to tolerate not found items
  (decarr@redhat.com)
- Merge pull request #2768 from ncdc/confirm (dmcphers+openshiftbot@redhat.com)
- New-app: add support for template files (cewong@redhat.com)
- Merge pull request #2105 from pweil-/sticky-sessions
  (dmcphers+openshiftbot@redhat.com)
- Revert SDN and Kubelet initialization ordering (sdodson@redhat.com)
- update origin-version-change for configapi.Config objects (deads@redhat.com)
- fix output messages for api versions (deads@redhat.com)
- Merge pull request #2781 from mfojtik/fix-upstream-keyring
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2742 from kargakis/dc-controller-logging
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2776 from derekwaynecarr/ns_controller_fixup
  (dmcphers+openshiftbot@redhat.com)
- Rename dry-run to confirm for prune commands (agoldste@redhat.com)
- sticky sessions (pweil@redhat.com)
- Actually remove router and registry from experimental commands
  (kargakis@users.noreply.github.com)
- Merge pull request #2703 from ncdc/fix-registry-auth
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2737 from pweil-/reverted-router
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2752 from derekwaynecarr/v1_descriptions_image
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Match the isDefaultRegistryMatch with upstream (mfojtik@redhat.com)
- expose: Allow specifying a hostname when generating routes
  (kargakis@users.noreply.github.com)
- Merge pull request #2599 from kargakis/remove-ex-refs-cmds
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2758 from spadgett/relax-url-validation
  (dmcphers+openshiftbot@redhat.com)
- expose: Fix service validation (kargakis@users.noreply.github.com)
- dcController: Use annotations to list deployments
  (kargakis@users.noreply.github.com)
- Merge pull request #2748 from derekwaynecarr/v1_descriptions_build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2727 from pmorie/mcs-range
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2476 from kargakis/newapp-contextdir
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2719 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2484 from mjisyang/build-fix
  (dmcphers+openshiftbot@redhat.com)
- Make origin project delete controller more fault tolerant to stop O(n) minute
  deletes (decarr@redhat.com)
- Merge pull request #2772 from ironcladlou/dc-stop-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2770 from spadgett/incorrect-mime-types
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2760 from derekwaynecarr/v1_descriptions_user
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2759 from derekwaynecarr/v1_descriptions_template
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2756 from deads2k/fix-overwrite-policy
  (dmcphers+openshiftbot@redhat.com)
- Make default namespace and osadm new-project set up service account roles
  (jliggitt@redhat.com)
- Update e2e to handle registry auth (agoldste@redhat.com)
- Require token in client config for image pruning (agoldste@redhat.com)
- Image policy updates (agoldste@redhat.com)
- Update registry policy (agoldste@redhat.com)
- Allow registry /healthz without auth (agoldste@redhat.com)
- Enable registry authentication (agoldste@redhat.com)
- Remove username validation for registry login (agoldste@redhat.com)
- Merge pull request #2757 from derekwaynecarr/v1_descriptions_sdn
  (dmcphers+openshiftbot@redhat.com)
- Delete deploymentConfig before reaping deployments (ironcladlou@gmail.com)
- Merge pull request #2755 from derekwaynecarr/v1_descriptions_route
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2749 from spadgett/close-websockets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2764 from soltysh/post2699
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2762 from spadgett/update-no-builders-msg
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2754 from derekwaynecarr/v1_descriptions_project
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2528 from pravisankar/project-node-selector-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2745 from kargakis/nit-expose-upstream
  (dmcphers+openshiftbot@redhat.com)
- fix reverted code (pweil@redhat.com)
- Merge pull request #2706 from soltysh/migrate-examples
  (dmcphers+openshiftbot@redhat.com)
- Fix incorrect Content-Type in some Web Console responses
  (spadgett@redhat.com)
- Add v1 descriptions to image api (decarr@redhat.com)
- update generated kubeconfig keys to match osc login (deads@redhat.com)
- Merge pull request #2738 from spadgett/depconfig-error
  (dmcphers+openshiftbot@redhat.com)
- Move registry and router out of experimental commands
  (kargakis@users.noreply.github.com)
- Relax source repository URL validation (spadgett@redhat.com)
- Updated validation for builds per @rhcarvalho comments in #2699: separated
  and hardened test cases and simplified validation for multiple ICTs
  (maszulik@redhat.com)
- Merge pull request #2732 from spadgett/memleak
  (dmcphers+openshiftbot@redhat.com)
- Remove "image repositories" from example osc command (spadgett@redhat.com)
- Add v1 descriptions to user api (decarr@redhat.com)
- Add v1 descriptions to external template api (decarr@redhat.com)
- UPSTREAM: json marshalling error must manifest earlier in resource visitors
  (contact@fabianofranz.com)
- fix overwrite-policy prefix (deads@redhat.com)
- Add v1 descriptions to sdn api (decarr@redhat.com)
- Add v1 descriptions to route api (decarr@redhat.com)
- Add missing description to v1 project api (decarr@redhat.com)
- Merge pull request #2746 from derekwaynecarr/add_descriptions_v1
  (dmcphers+openshiftbot@redhat.com)
- Project node selector fixes (rpenta@redhat.com)
- Don't reopen websockets closed by DataService.unwatch() (spadgett@redhat.com)
- Add v1 descriptions to all build fields (decarr@redhat.com)
- Bug 1224089: UPSTREAM: expose: Better error formatting and generic flag
  message (kargakis@users.noreply.github.com)
- Add descriptions to v1 policy api, remove deprecated field
  (decarr@redhat.com)
- stop: Reap all deployments of a config (kargakis@users.noreply.github.com)
- Update `osc logs` examples (rhcarvalho@gmail.com)
- Guard against null deployment config triggers (spadgett@redhat.com)
- Update hawtio-core-navigation to 2.0.48 (spadgett@redhat.com)
- Merge pull request #2728 from liggitt/request_project
  (dmcphers+openshiftbot@redhat.com)
- Add commit to github links, improve git link display (jliggitt@redhat.com)
- Remove --node-selector from osc new-project (jliggitt@redhat.com)
- Add godoc for mcs.ParseRange (pmorie@gmail.com)
- Merge pull request #2611 from deads2k/really-disable-v1beta1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2700 from bparees/hello
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2711 from bparees/grammar
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2674 from liggitt/client_token_file
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2699 from soltysh/issue2586
  (dmcphers+openshiftbot@redhat.com)
- Fix issue#2655; bz1225410; rebase openshift-sdn Godeps (rchopra@redhat.com)
- Merge pull request #2705 from smarterclayton/swagger_upstream
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2666 from pravisankar/expose-podEvictionTimeout
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2677 from liggitt/honor_links
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2612 from ironcladlou/hook-retry-cancellation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2673 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Switch UI to using legacy API prefix (jliggitt@redhat.com)
- fix grammar of resource create message (bparees@redhat.com)
- Merge pull request #2645 from mrunalp/iptables_restart_doc
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2635 from derekwaynecarr/cherry_pick_8875
  (dmcphers+openshiftbot@redhat.com)
- disable v1beta1 by default (deads@redhat.com)
- UPSTREAM: Backport schema output fixes (ccoleman@redhat.com)
- JSON examples migration to v1beta3 leftovers (maszulik@redhat.com)
- Make pod eviction timeout configurable (rpenta@redhat.com)
- Honor clicks anywhere inside a link (jliggitt@redhat.com)
- Merge pull request #2660 from smarterclayton/fix_api_prefix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2690 from spadgett/parameter-descriptions
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2631 from mfojtik/convert-samples-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2684 from spadgett/v1beta3-follow-on
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2688 from spadgett/template-cancel
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2676 from liggitt/ui_e2e_test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2653 from soltysh/fix_version_changer
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2664 from derekwaynecarr/bug_1224671
  (dmcphers+openshiftbot@redhat.com)
- add tmp volume so scratch image will work (bparees@redhat.com)
- Allow populating bearer token from file contents (jliggitt@redhat.com)
- Merge pull request #2567 from deads2k/non-zero
  (dmcphers+openshiftbot@redhat.com)
- Issue 2586 - allow only one ImageChangeTrigger for BuildConfig.
  (maszulik@redhat.com)
- Generate swagger specs and docs for our API (ccoleman@redhat.com)
- Refactor deploy hook retry policy (ironcladlou@gmail.com)
- Tearing down old deployment before bringing up new one for the recreate
  strategy (abhgupta@redhat.com)
- Merge pull request #2658 from abhgupta/agupta-deploy1
  (dmcphers+openshiftbot@redhat.com)
- Simplify the deploy trigger int test (ironcladlou@gmail.com)
- Merge pull request #2641 from deads2k/fix-new-app
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Add "Info" to go-restful ApiDecl (ccoleman@redhat.com)
- UPSTREAM: Hack date-time format on *util.Time (ccoleman@redhat.com)
- UPSTREAM: Expose OPTIONS but not TRACE (ccoleman@redhat.com)
- UPSTREAM: Patch needs a type for swagger doc (ccoleman@redhat.com)
- Properly support the 'oapi' prefix (ccoleman@redhat.com)
- Show template parameter descriptions on create page (spadgett@redhat.com)
- Merge pull request #2621 from mfojtik/gh-2595
  (dmcphers+openshiftbot@redhat.com)
- Restore comments and remove empty ServiceAccount (mfojtik@redhat.com)
- Migrate YAML files to v1beta3 (mfojtik@redhat.com)
- Fix hack/convert-samples to deal with YAML files (mfojtik@redhat.com)
- Follow-on Web Console fixes for osapi/v1beta3 (spadgett@redhat.com)
- Fix Cancel button error on Web Console create pages (spadgett@redhat.com)
- Add UI e2e tests (jliggitt@redhat.com)
- Rework dockercfg in builder package to use upstream keyring
  (mfojtik@redhat.com)
- Merge pull request #2681 from kargakis/fix-expose-validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2623 from mfojtik/retry-push
  (dmcphers+openshiftbot@redhat.com)
- expose: Fix service validation and help message
  (kargakis@users.noreply.github.com)
- Merge pull request #2679 from liggitt/omitempty_service_account
  (dmcphers+openshiftbot@redhat.com)
- Retry the output image push if failed (mfojtik@redhat.com)
- Merge pull request #2578 from spadgett/osapi-v1beta3
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: fix omitempty on service account in v1beta3 (jliggitt@redhat.com)
- Deleting deployer pods for failed deployment before retrying
  (abhgupta@redhat.com)
- Merge pull request #2671 from thesteve0/patch-4
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2668 from soltysh/sti_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2663 from fabianofranz/bash_completions
  (dmcphers+openshiftbot@redhat.com)
- Update Web Console to use osapi/v1beta3 (spadgett@redhat.com)
- Merge pull request #2654 from bparees/build_race
  (dmcphers+openshiftbot@redhat.com)
- Fixing doc to make clear where the source should be (scitronpousty@gmail.com)
- Merge pull request #2603 from kargakis/service-description
  (dmcphers+openshiftbot@redhat.com)
- Updated code with latest S2I. (maszulik@redhat.com)
- bump(github.com/openshift/source-to-
  image):77e3b722b028b8af94a3606d0dbb76dc377755fd (maszulik@redhat.com)
- Added Lists to origin-version-changer (maszulik@redhat.com)
- fix long help description to remove garbage chars (decarr@redhat.com)
- Merge pull request #2559 from kargakis/reduce-inteval-for-dcs
  (dmcphers+openshiftbot@redhat.com)
- Re-generate bash completions (contact@fabianofranz.com)
- Fixes bash completion gen script (contact@fabianofranz.com)
- Merge pull request #2613 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- validate lasttriggeredimageid hasn't changed before running build
  (bparees@redhat.com)
- create osc policy * for non-cluster admin usage (deads@redhat.com)
- make subcommands return non-zero status on failures (deads@redhat.com)
- Build ImageChangeController shouldn't return early if it encounters an
  Instantiate error (mfojtik@redhat.com)
- Merge pull request #2620 from mfojtik/last-triggered-ref
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2544 from smarterclayton/uid_allocation
  (dmcphers+openshiftbot@redhat.com)
- Adds steps for safely restarting iptables (mrunal@me.com)
- deployconfig requires kind (deads@redhat.com)
- Security allocator and repair logic (ccoleman@redhat.com)
- Create UID and MCS category allocators (ccoleman@redhat.com)
- UPSTREAM: Add contiguous allocator (ccoleman@redhat.com)
- UPSTREAM Force explicit namespace provision (decarr@redhat.com)
- UPSTREAM Do not set container requests in limit ranger for Kube 1.0
  (decarr@redhat.com)
- Merge pull request #2436 from kargakis/expose-bug
  (dmcphers+openshiftbot@redhat.com)
- [RPMs]: Install bash completion files (sdodson@redhat.com)
- Make the TriggeredByImage field ObjectReference (mfojtik@redhat.com)
- UPSTREAM: kube: serialize dockercfg files with auth (deads@redhat.com)
- auto-generate dockercfg secrets for service accounts (deads@redhat.com)
- UPSTREAM: kube: cleanup unused service account tokens (deads@redhat.com)
- Merge pull request #2602 from spadgett/fix-undefined-undefined-message
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2616 from pmorie/preex-deployer-pod
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2486 from mfojtik/migrate-examples
  (dmcphers+openshiftbot@redhat.com)
- Print name of existing pods with deployer-pod names (pmorie@gmail.com)
- Fixed typo in BuildRequest field name (mfojtik@redhat.com)
- Replace custom/flaky pod comparison with DeepEqual (ironcladlou@gmail.com)
- Adding meaningful comments deployments cannot be started/retried
  (abhgupta@redhat.com)
- Merge pull request #2569 from sosiouxme/20150528-skydns-norec
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2608 from spadgett/repo-example
  (dmcphers+openshiftbot@redhat.com)
- Added missing test object and updated DeploymentConfig's ImageChange trigger
  definition to match current validations. (maszulik@redhat.com)
- Merge pull request #2582 from ironcladlou/hook-cancellation
  (dmcphers+openshiftbot@redhat.com)
- Guard against missing error details kind or ID (spadgett@redhat.com)
- Merge pull request #2605 from spadgett/fix-object-object-message
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2570 from ncdc/pluggable-exec
  (dmcphers+openshiftbot@redhat.com)
- Add example Git URL to Create page (spadgett@redhat.com)
- Merge pull request #2547 from ramr/nowatchport0
  (dmcphers+openshiftbot@redhat.com)
- integration/dns_test.go: don't expect recursion (lmeyer@redhat.com)
- SkyDNS: don't recurse requests (lmeyer@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- bump(github.com/skynetservices/skydns):01de2a7562896614c700547f131e240b665468
  0c (lmeyer@redhat.com)
- Show correct error details on browse builds page (spadgett@redhat.com)
- UPSTREAM: Show pods number when describing services
  (kargakis@users.noreply.github.com)
- Use kubectl constants for dc reaper interval/timeout
  (kargakis@users.noreply.github.com)
- Merge pull request #2475 from mfojtik/service-account-push-secrets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2589 from kargakis/resize-to-scale-rename
  (dmcphers+openshiftbot@redhat.com)
- Make node's Docker exec handler configurable (agoldste@redhat.com)
- Merge pull request #2590 from deads2k/redo-upstream
  (dmcphers+openshiftbot@redhat.com)
- Rename BuildRequest.Image to TriggedByImage (mfojtik@redhat.com)
- Support cancelling deployment hooks (etc) (ironcladlou@gmail.com)
- Add more description for the BuildRequest Image field (mfojtik@redhat.com)
- Merge pull request #2573 from deads2k/disable-v1beta1
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: kube: add pull secrets to service accounts (deads@redhat.com)
- Rename osc resize to osc scale (kargakis@users.noreply.github.com)
- make api levels configuratable (deads@redhat.com)
- UPSTREAM: rename resize to scale (kargakis@users.noreply.github.com)
- Tolerate missing BuildStatusNew in webhookgithub integration test
  (mfojtik@redhat.com)
- Fix TestWebhookGithubPushWithImage integration test (mfojtik@redhat.com)
- Resolve ImageStream reference outside loop in resolveImageSecret
  (mfojtik@redhat.com)
- Do not mutate DefaultServiceAccountName in BuildGenerator
  (mfojtik@redhat.com)
- Fix build image change race (agoldste@redhat.com)
- Provide default PushSecret and PullSecret using Service Account
  (mfojtik@redhat.com)
- Auto-wire push secrets into builds (agoldste@redhat.com)
- UPSTREAM: Add 'docker.io' and 'index.docker.io' to default registry
  (mfojtik@redhat.com)
- Merge pull request #2577 from deads2k/fix-registry
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2552 from csrwng/build_deploy_podnames
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2580 from csrwng/fix_webhook_url
  (dmcphers+openshiftbot@redhat.com)
- Fix name collisions between build and deployment pods (cewong@redhat.com)
- Fix structure of webhooks URL display in web console (cewong@redhat.com)
- UPSTREAM: Allowing ActiveDeadlineSeconds to be updated for a pod
  (abhgupta@redhat.com)
- Merge pull request #2576 from fabianofranz/bugs_1220998
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2533 from derekwaynecarr/cleanup_deploymentconfig
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2561 from deads2k/fix-remove-user-again
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2532 from pravisankar/fix-admin-manage-node
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2555 from ironcladlou/deployer-pod-failure-detection
  (dmcphers+openshiftbot@redhat.com)
- Bug 1220998 - improve message in osc env update errors
  (contact@fabianofranz.com)
- make registry and router exit non-zero for failures (deads@redhat.com)
- UPSTREAM: Add support for pluggable Docker exec handlers
  (agoldste@redhat.com)
- Ensure OpenShift content creation results in namespace finalizer
  (decarr@redhat.com)
- Merge pull request #2557 from HyunsooKim1112/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2502 from ncdc/registry-selector
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2554 from mattf/master (dmcphers+openshiftbot@redhat.com)
- remove-from-project and integration tests (deads@redhat.com)
- Merge pull request #2551 from liggitt/config_change_detection
  (dmcphers+openshiftbot@redhat.com)
- Update README.md (hyun.soo.kim1112@gmail.com)
- Deployer pod failure detection improvements (ironcladlou@gmail.com)
- Merge pull request #2543 from spadgett/settings-tooltips
  (dmcphers+openshiftbot@redhat.com)
- update readme - centos7 has docker 1.6 (matt@redhat.com)
- Fix pkg/template test to use stored JSON fixture (mfojtik@redhat.com)
- Set default BuildTriggerPolicy.ImageChange when not given
  (rhcarvalho@gmail.com)
- Bug 1224097: Don't use custom names for route.servicename
  (kargakis@users.noreply.github.com)
- UPSTREAM: expose: Use separate parameters for default and custom name
  (kargakis@users.noreply.github.com)
- Fix pkg/template tests to use v1beta3 (mfojtik@redhat.com)
- Migrate all JSON examples to v1beta3 (mfojtik@redhat.com)
- Add script which migrate all JSON samples to certain version
  (mfojtik@redhat.com)
- Removing unnecessary namespace prefix for deployment/deploymentConfig/Repo
  (j.hadvig@gmail.com)
- Add test to detect config changes (jliggitt@redhat.com)
- Fix flaky TestConcurrentBuild* tests (jliggitt@redhat.com)
- Add eth1 to virtual box interfaces and add support to not watch/monitor port
  for the f5 ipfailover use case (--watch-port=0). (smitram@gmail.com)
- Don't default node selector for openshift admin manag-node cmd
  (rpenta@redhat.com)
- Use tooltip rather than popover on settings page (spadgett@redhat.com)
- Add --selector support to osadm registry (agoldste@redhat.com)
- new-app: Add ability to specify a context directory
  (kargakis@users.noreply.github.com)
- Flip-flop priorities so that we distribute vips evenly. (smitram@gmail.com)
- 'cat' here document rather than 'echo' (mjisyang@gmail.com)

* Wed May 27 2015 Scott Dodson <sdodson@redhat.com> 0.5.2.2
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2122 from sdodson/add-proxy-env-example
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2522 from ironcladlou/refactor-cancel-controller
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2542 from spadgett/breadcrumbs
  (dmcphers+openshiftbot@redhat.com)
- Add proxy examples to master and node sysconfig files (sdodson@redhat.com)
- Merge pull request #2512 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2535 from spadgett/start-build-error
  (dmcphers+openshiftbot@redhat.com)
- Refactor to match upstream (ccoleman@redhat.com)
- UPSTREAM: Add groups to service account JWT (jliggitt@redhat.com)
- UPSTREAM: print more useful error (ccoleman@redhat.com)
- UPSTREAM: implement a generic webhook storage (ccoleman@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):496be63c0078ce7323aede59005ad
  bd3e9eef8c7 (ccoleman@redhat.com)
- Merge pull request #2527 from spadgett/image-loading-msg
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2534 from ironcladlou/remove-deployment-resource
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2359 from spadgett/project-empty-fix
  (dmcphers+openshiftbot@redhat.com)
- Refactor deployment cancellation handling (ironcladlou@gmail.com)
- Add breadcrumbs to pages in create flow (spadgett@redhat.com)
- Merge pull request #2375 from sdodson/openshift-sdn-ovs
  (dmcphers+openshiftbot@redhat.com)
- Show loading message until builders are loaded (spadgett@redhat.com)
- Fix error manually starting first build in Web Console (spadgett@redhat.com)
- Support rolling deployment hooks (ironcladlou@gmail.com)
- Remove Deployment resource (ironcladlou@gmail.com)
- Merge pull request #2452 from jhadvig/err_msgs
  (dmcphers+openshiftbot@redhat.com)
- Don't show get empty project message if project has monopods
  (spadgett@redhat.com)
- Merge pull request #2525 from spadgett/create-page-updates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2519 from deads2k/fix-remove-user
  (dmcphers+openshiftbot@redhat.com)
- Improving/Adding/Refactoring logging messages (j.hadvig@gmail.com)
- Merge pull request #2477 from liggitt/sdn_node_role
  (dmcphers+openshiftbot@redhat.com)
- Update labels and styles for create flow (spadgett@redhat.com)
- fix remove-user/group validation (deads@redhat.com)
- Merge pull request #2465 from bparees/reconcile_buildpods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2514 from liggitt/route_fix
  (dmcphers+openshiftbot@redhat.com)
- Force refresh of projects when OpenShift Origin clicked (spadgett@redhat.com)
- pod/build delete sync (bparees@redhat.com)
- Merge pull request #2410 from liggitt/node_selector
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2492 from spadgett/validate-source-url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2353 from spadgett/pod-warnings
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2508 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2468 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2504 from liggitt/test_cmd
  (dmcphers+openshiftbot@redhat.com)
- Fix ProjectNodeSelector config (jliggitt@redhat.com)
- Merge pull request #2503 from spadgett/button-labels
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2501 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Update vmware fedora version to be consistent (dmcphers@redhat.com)
- Disable `Next` button unless source URL is valid (spadgett@redhat.com)
- Added the deployment cancellation command to the CLI (abhgupta@redhat.com)
- Wait for service account before creating pods (jliggitt@redhat.com)
- system:node, system:sdn-reader, system:sdn-manager roles
  (jliggitt@redhat.com)
- Warn about pods with containers in bad states (spadgett@redhat.com)
- Update openshift-object-describer and k8s-label-selector dependencies
  (spadgett@redhat.com)
- Merge pull request #2497 from kargakis/upstream-resize-update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2493 from spadgett/valid-project-names
  (dmcphers+openshiftbot@redhat.com)
- Formatting fixes (dmcphers@redhat.com)
- Merge pull request #2480 from liggitt/management_console_rename
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2454 from stevekuznetsov/dev/skuznets/bug/1223188
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2469 from abhgupta/agupta-deploy1
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: resize: Enable resource type/name syntax
  (kargakis@users.noreply.github.com)
- Merge pull request #2490 from soltysh/sample_image_name
  (dmcphers+openshiftbot@redhat.com)
- Add minlength to name field on project create form (spadgett@redhat.com)
- Rename 'Management Console' to 'Web Console' (jliggitt@redhat.com)
- Updated image name in sample-app's pullimages.sh script (maszulik@redhat.com)
- UPSTREAM: kube: add pull secrets to service accounts (deads@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Beef up osc label help message (kargakis@users.noreply.github.com)
- Bug 1223252: UPSTREAM: label: Invalidate empty or invalid value labels
  (kargakis@users.noreply.github.com)
- Merge pull request #2220 from soltysh/build_retries
  (dmcphers+openshiftbot@redhat.com)
- Fix repo name (dmcphers@redhat.com)
- Bug 1214205 - editor must always use the original object namespace when
  trying to update (contact@fabianofranz.com)
- Merge pull request #2472 from mfojtik/migrate-examples
  (dmcphers+openshiftbot@redhat.com)
- Unpack and change versions of Template objects in origin-version-change
  (mfojtik@redhat.com)
- Show build pods events together with build events on osc describe build
  (maszulik@redhat.com)
- Fail build after 30 minutes of errors. (maszulik@redhat.com)
- Merge pull request #2457 from markturansky/update_os_volume_plugins
  (dmcphers+openshiftbot@redhat.com)
- use kubernetes plugin list instead of custom os list (mturansk@redhat.com)
- Merge pull request #2446 from detiber/networkConfigJSON
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2438 from rhcarvalho/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2428 from spadgett/orderBy-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2407 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2408 from pweil-/remove-invalid-escapes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2460 from bparees/templateupdate
  (dmcphers+openshiftbot@redhat.com)
- Add ServiceAccount type to list of exposed kube resources
  (jliggitt@redhat.com)
- Setting deployment/cancellation reason based on constants
  (abhgupta@redhat.com)
- Merge pull request #2345 from liggitt/service_accounts
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2456 from gashcrumb/bug-1224083
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2442 from kargakis/wait-while-resizing
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2453 from kargakis/another-expose-bug-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2445 from markturansky/pvc_whitelist
  (dmcphers+openshiftbot@redhat.com)
- Enable service account admission, token controller, auth, bootstrap policy
  (jliggitt@redhat.com)
- UPSTREAM: Add groups to service account JWT (jliggitt@redhat.com)
- UPSTREAM: gate token controller until caches are filled (jliggitt@redhat.com)
- Merge pull request #2398 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2374 from kargakis/disable-dc-triggers-when-reaping
  (dmcphers+openshiftbot@redhat.com)
- add configchange trigger to frontend deployments (bparees@redhat.com)
- Merge pull request #2418 from csrwng/error_on_template_create
  (dmcphers+openshiftbot@redhat.com)
- Added volume plugins to match upstream (mturansk@redhat.com)
- Merge pull request #2422 from spadgett/dup-images
  (dmcphers+openshiftbot@redhat.com)
- Added deleteTemplates method to remove templates from namespace
  (steve.kuznetsov@gmail.com)
- #1224083 - Update openshift-jvm to 1.0.17 (slewis@fusesource.com)
- UPSTREAM: bump timeout back to previous time
  (kargakis@users.noreply.github.com)
- UPSTREAM: Reduce reaper poll interval and wait while resizing
  (kargakis@users.noreply.github.com)
- Don't error out when exposing a label-less service as a route
  (kargakis@users.noreply.github.com)
- Merge pull request #2439 from kargakis/cli-variable-renaming
  (dmcphers+openshiftbot@redhat.com)
- Added Kube PVC to OS whitelist (mturansk@redhat.com)
- Adding deployment cancellation controller (abhgupta@redhat.com)
- Disable triggers while reaping a deploymentConfig
  (kargakis@users.noreply.github.com)
- Display error when processing a template fails. (cewong@redhat.com)
- Don't repeat builder images in image catalog (spadgett@redhat.com)
- Add json tag for NetworkConfig (jdetiber@redhat.com)
- Merge remote-tracking branch 'upstream/pr/2375' (sdodson@redhat.com)
- Move mock client calls under testclient (kargakis@users.noreply.github.com)
- Make golint happier (kargakis@users.noreply.github.com)
- Omit internal field when empty (rhcarvalho@gmail.com)
- Merge pull request #1841 from ncdc/image-pruning
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2431 from csrwng/new_app_template_test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2425 from fabianofranz/bugs_1216930
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2351 from liggitt/name_length_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2419 from spadgett/overview-warning-flicker
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2423 from smarterclayton/pods_to_status
  (dmcphers+openshiftbot@redhat.com)
- Add test-cmd test to validate that we can create using a stored template
  (cewong@redhat.com)
- Merge pull request #2424 from deads2k/add-secret-types
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2416 from gashcrumb/java-link-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2402 from abhgupta/trello_card_739
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2347 from mfojtik/secrets-references
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/openshift-
  sdn):7752990d5095d92905752aa5165a3d65ba8195e6 (sdodson@redhat.com)
- [RPMs] add openshift-sdn-ovs subpackage to carry ovs scripts and ovs dep
  (sdodson@redhat.com)
- Merge pull request #2377 from derekwaynecarr/deployment_registry_updates
  (dmcphers+openshiftbot@redhat.com)
- Fix port ordering for a service (spadgett@redhat.com)
- Merge pull request #2421 from sallyom/new (dmcphers+openshiftbot@redhat.com)
- Bug 1216930 - fix runtime error in rollback --dry-run
  (contact@fabianofranz.com)
- Validate project and image stream namespaces and names (jliggitt@redhat.com)
- Merge pull request #2045 from kargakis/expose-routes-from-services
  (dmcphers+openshiftbot@redhat.com)
- Support custom CA for registry pruning (agoldste@redhat.com)
- Avoid warning icon flicker for deployment configs (spadgett@redhat.com)
- add TODO for watching delete events; catch error on StartMaster; do not try
  to create clusternetwork if it already exists (rchopra@redhat.com)
- Project status should include pod information (ccoleman@redhat.com)
- handle secret types (deads@redhat.com)
- Make link show up again and pass along token to openshift-jvm
  (slewis@fusesource.com)
- Added test cases to verify that ActiveDeadlineSeconds is set
  (abhgupta@redhat.com)
- Use client config's CA for registry pruning (agoldste@redhat.com)
- Duplicate default value in create node config was wrong (ccoleman@redhat.com)
- Replace secrets in BuildConfig with LocalObjectReference (mfojtik@redhat.com)
- Move deployment config to etcdgeneric storage patterns (decarr@redhat.com)
- grammar fix (somalley@redhat.com)
- respect --api-version flag for osc login (deads@redhat.com)
- Setting ActiveDeadlineSeconds on the deployment hook pod
  (abhgupta@redhat.com)
- UPSTREAM: kube: tolerate fatals without newlines (deads@redhat.com)
- Setting max/default ActiveDeadlineSeconds on the deployer pod
  (abhgupta@redhat.com)
- Add func for pruning an image from a stream (agoldste@redhat.com)
- Simplify output in login and project (ccoleman@redhat.com)
- Merge pull request #2387 from fabianofranz/issues_2358
  (dmcphers+openshiftbot@redhat.com)
- Expose routes from services (kargakis@users.noreply.github.com)
- UPSTREAM: expose: Use resource builder (kargakis@users.noreply.github.com)
- Issue 2358 - better handling of server url parsing in osc login
  (contact@fabianofranz.com)
- Merge pull request #2319 from deads2k/forbidden-message
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Attach pull secrets to pods (mfojtik@redhat.com)
- Handle change in imageChange trigger status for v1beta3 (ccoleman@redhat.com)
- Test changes caused by version defaults (ccoleman@redhat.com)
- Add origin-version-change script (ccoleman@redhat.com)
- Make v1beta3 the default storage version (ccoleman@redhat.com)
- Initial commit of v1 api (ccoleman@redhat.com)
- remove unnecessary double escape replacement and add validation to ensure no
  one is relying on the removed behavior (pweil@redhat.com)
- fix looping on a closed channel - bz1222853, bz1223274 (rchopra@redhat.com)
- add prune images command (agoldste@redhat.com)
- Merge pull request #2404 from liggitt/fix_deployer_pods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2396 from deads2k/relax-bundle-secrets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2389 from csrwng/fix_new_app
  (dmcphers+openshiftbot@redhat.com)
- Use correct deployer annotation (jliggitt@redhat.com)
- update forbidden messages (deads@redhat.com)
- Merge pull request #2392 from deads2k/fix-e2e-console
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2391 from smarterclayton/strip_commands
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2365 from liggitt/overview
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2386 from smarterclayton/cpu_profiling_disabled
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2385 from smarterclayton/disable_manage_node_tests
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2357 from abhgupta/issue_2254
  (dmcphers+openshiftbot@redhat.com)
- relax restrictions on bundle secret (deads@redhat.com)
- Merge pull request #2369 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Retry stream updates when pruning (agoldste@redhat.com)
- fix e2e public master (deads@redhat.com)
- Helpful hints for osc deploy make osc status worse (ccoleman@redhat.com)
- Remove summary pruner (agoldste@redhat.com)
- Fix template decode error when using new-app to create from a named template
  (cewong@redhat.com)
- Update pruning tests (agoldste@redhat.com)
- Replace _pods.less with _components.less (sgoodwin@redhat.com)
- Reenable CPU profiling default (ccoleman@redhat.com)
- manage-node tests can't rely on scheduling in hack/test-cmd.sh
  (ccoleman@redhat.com)
- Merge pull request #2039 from derekwaynecarr/sample_template
  (dmcphers+openshiftbot@redhat.com)
- wip (agoldste@redhat.com)
- Delegate manifest deletion to original Repository (agoldste@redhat.com)
- Image Pruning (agoldste@redhat.com)
- Image pruning (agoldste@redhat.com)
- Image pruning (agoldste@redhat.com)
- Image pruning (agoldste@redhat.com)
- Image Pruning (agoldste@redhat.com)
- Image pruning (agoldste@redhat.com)
- Image pruning (agoldste@redhat.com)
- Add image pruning support (agoldste@redhat.com)
- UPSTREAM(docker/distribution): manifest deletions (agoldste@redhat.com)
- UPSTREAM(docker/distribution): custom routes/auth (agoldste@redhat.com)
- UPSTREAM(docker/distribution): add BlobService (agoldste@redhat.com)
- Making the deployer pod name deterministic  - deployer pod name is now the
  same as the deployment name  - if an unrelated pod exists with the same name,
  the deployment is set to Failed (abhgupta@redhat.com)
- UPSTREAM(docker/distribution): add layer unlinking (agoldste@redhat.com)
- Add an example for a quota tracked project (decarr@redhat.com)
- Merge pull request #2113 from mfojtik/image_metadata_tags
  (dmcphers+openshiftbot@redhat.com)
- Refactor overview page, combine route/service blocks (jliggitt@redhat.com)
- static markup and styles for overview (sgoodwin@redhat.com)
- Update install-assets.sh to use curl (jliggitt@redhat.com)
- Add Labels field into DockerConfig type (mfojtik@redhat.com)
- Code cleanup: Leveraging golang swap (abhgupta@redhat.com)

* Wed May 20 2015 Scott Dodson <sdodson@redhat.com> 0.5.2.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2368 from csrwng/fix_webhook_url
  (dmcphers+openshiftbot@redhat.com)
- Fix generic webhook URL display (cewong@redhat.com)
- Merge pull request #2362 from smarterclayton/fix_help
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2356 from smarterclayton/image_metadata
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2364 from derekwaynecarr/build_not_getting_resources
  (dmcphers+openshiftbot@redhat.com)
- Builds from BuildConfig missing BuildParameters.Resources (decarr@redhat.com)
- Code cleanup for openshift admin manage-node (rpenta@redhat.com)
- Update the help on admin commands (ccoleman@redhat.com)
- Merge pull request #2327 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2320 from markturansky/add_pvcbinder
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2323 from csrwng/upstream_kubelet_start
  (dmcphers+openshiftbot@redhat.com)
- Display metadata about images in describe (ccoleman@redhat.com)
- Considering existing deployments for deployment configs  - If a
  running/pending/new deployment exists, the config is requeued  - If multiple
  running/previous/new deployments exists, older ones are cancelled
  (abhgupta@redhat.com)
- Merge pull request #2344 from smarterclayton/improve_image_get
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2349 from brenton/master
  (dmcphers+openshiftbot@redhat.com)
- Use upstream RunKubelet method to run kubelet (cewong@redhat.com)
- Reduce fuzz iterations (ccoleman@redhat.com)
- Merge pull request #2343 from smarterclayton/fix_roundtrip_fuzz
  (ccoleman@redhat.com)
- Merge pull request #2336 from derekwaynecarr/prune_cmd
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2340 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2346 from kargakis/fix-tests
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2339 from smarterclayton/retry_conflicts_on_import
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #27 from brenton/master (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2297 from smarterclayton/set_max_idle
  (dmcphers+openshiftbot@redhat.com)
- Print more info on image tags and images (ccoleman@redhat.com)
- Merge pull request #2337 from bparees/escapes
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Minor change to the ipfailover Dockerfile for consistency
  (bleanhar@redhat.com)
- Minor change to the ipfailover Dockerfile for consistency
  (bleanhar@redhat.com)
- Add admin prune command (decarr@redhat.com)
- Switching the ipfailover pod to build on RHEL (bleanhar@redhat.com)
- test-cmd: Check for removed rc (kargakis@users.noreply.github.com)
- test-cmd: Check for removed rc (kargakis@users.noreply.github.com)
- Merge pull request #2341 from smarterclayton/bump_etcd
  (dmcphers+openshiftbot@redhat.com)
- Build output round trip test can have ':' (ccoleman@redhat.com)
- Refactor etcd server start (ccoleman@redhat.com)
- Merge pull request #2276 from derekwaynecarr/prune_deployments
  (dmcphers+openshiftbot@redhat.com)
- Refactor to match upstream (ccoleman@redhat.com)
- UPSTREAM: print more useful error (ccoleman@redhat.com)
- UPSTREAM: implement a generic webhook storage (ccoleman@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):6b6b47a777b4802c9c1360ea0d583
  da6cfec7363 (ccoleman@redhat.com)
- bump(etcd):v2.0.11 (ccoleman@redhat.com)
- Merge pull request #2338 from fabianofranz/issues_2293
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2328 from spadgett/long-project-name
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2257 from fabianofranz/bugs_1218126
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2006 from smarterclayton/require_go_14
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1944 from kargakis/bundle-stop-and-label
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2315 from mfojtik/kubernetes-8407
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2325 from fabianofranz/issues_2322
  (dmcphers+openshiftbot@redhat.com)
- Retry image import conflicts (ccoleman@redhat.com)
- Merge pull request #2285 from smarterclayton/round_trip_tags
  (dmcphers+openshiftbot@redhat.com)
- Issue 2293 - fixes options help message (contact@fabianofranz.com)
- Merge pull request #2268 from sdodson/rpm-sdn-scripts
  (dmcphers+openshiftbot@redhat.com)
- Add text-overflow: ellipsis style to project select (spadgett@redhat.com)
- remove extraneous escapes (bparees@redhat.com)
- Merge pull request #2317 from sdodson/bz1221773
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2306 from smarterclayton/move_profile_statement
  (dmcphers+openshiftbot@redhat.com)
- More login tests (contact@fabianofranz.com)
- Merge pull request #2180 from ramr/nodeselector
  (dmcphers+openshiftbot@redhat.com)
- Set default project in login so we can generate a ctx name properly
  (contact@fabianofranz.com)
- Bug 1218126 - login needs to make use of token if provided
  (contact@fabianofranz.com)
- gofmt and doc changes for Go 1.4 (ccoleman@redhat.com)
- Merge pull request #2299 from liggitt/test_flake
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2318 from mfojtik/fix-release
  (dmcphers+openshiftbot@redhat.com)
- Issue 2322 - fixes pod rendering in deployments screen
  (contact@fabianofranz.com)
- Merge pull request #2302 from liggitt/ui_token_expiration
  (dmcphers+openshiftbot@redhat.com)
- Add osc stop and osc label (kargakis@users.noreply.github.com)
- Docker registry installed should be logged (ccoleman@redhat.com)
- Stop using ephemeral ports for integration tests (jliggitt@redhat.com)
- Remember token ttl, stop retrieving user/token from localStorage after
  expiration (jliggitt@redhat.com)
- Added Kubernetes PVClaimBinder to OpenShift (mturansk@redhat.com)
- Merge pull request #2223 from derekwaynecarr/improve_project_describe
  (dmcphers+openshiftbot@redhat.com)
- Prune deployment utilities (decarr@redhat.com)
- Fix go cross compiler package names in release image (mfojtik@redhat.com)
- Describe project should show quota and resource limits (decarr@redhat.com)
- Merge pull request #2313 from spadgett/undefined-undefined
  (dmcphers+openshiftbot@redhat.com)
- Set NOFILE to 128k/64k for master/node, set CORE=infinity
  (sdodson@redhat.com)
- UPSTREAM: Disable 'Timestamps' in Docker logs to prevent double-timestamps
  (mfojtik@redhat.com)
- Don't show undefined in error messages when error details incomplete
  (spadgett@redhat.com)
- gofmt fix (kargakis@users.noreply.github.com)
- Merge pull request #2208 from smarterclayton/new_dns
  (dmcphers+openshiftbot@redhat.com)
- Unbreak default profiling behavior (ccoleman@redhat.com)
- Merge pull request #2301 from liggitt/token_ttl
  (dmcphers+openshiftbot@redhat.com)
- Set TTL on oauth tokens (jliggitt@redhat.com)
- Fix gofmt error (jliggitt@redhat.com)
- Merge pull request #2291 from liggitt/secure_cookies
  (dmcphers+openshiftbot@redhat.com)
- Use client.TransportFor to set an etcd transport (ccoleman@redhat.com)
- Merge pull request #2294 from smarterclayton/embed_kube_binaries
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2274 from smarterclayton/rename_sti_internally
  (dmcphers+openshiftbot@redhat.com)
- Expose kubernetes components as symlink binaries (ccoleman@redhat.com)
- Embed Kube binaries (ccoleman@redhat.com)
- bump(): add packages for kubelet/cloudprovider (ccoleman@redhat.com)
- Merge pull request #2290 from liggitt/rand (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2272 from deads2k/create-node-config-newline
  (dmcphers+openshiftbot@redhat.com)
- Use secure csrf/session cookies when serving on HTTPS (jliggitt@redhat.com)
- Use rand.Reader directly to generate tokens (jliggitt@redhat.com)
- Merge pull request #2288 from liggitt/osin (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2287 from liggitt/filter-scrollbar-fix
  (dmcphers+openshiftbot@redhat.com)
- Add custom token generation functions (jliggitt@redhat.com)
- Merge pull request #2280 from openshift/sti_rebase
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/RangelReale/osin):a9958a122a90a3b069389d394284283c19d58913
  (jliggitt@redhat.com)
- Update k8s-label-selector to 0.0.4 (spadgett@redhat.com)
- Merge pull request #1949 from spadgett/create-project
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2278 from deads2k/fix-policy-rule-describer
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2279 from spadgett/obj-describer-0.0.9
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2173 from akostadinov/vagrant_config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1908 from derekwaynecarr/ignore_admission
  (dmcphers+openshiftbot@redhat.com)
- Rename user visible errors to s2i or source-to-image (ccoleman@redhat.com)
- Rename STIStrategy to SourceStrategy in v1beta3 (ccoleman@redhat.com)
- Merge pull request #2267 from deads2k/project-request-template
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2260 from derekwaynecarr/prune_commands
  (dmcphers+openshiftbot@redhat.com)
- update sti image builder sti release (bparees@redhat.com)
- bump (github.com/openshift/source-to-
  image):ac0b2512c9f933afa985b056a7f3cbce1942cacb (bparees@redhat.com)
- Add create project to web console (spadgett@redhat.com)
- Merge pull request #2215 from csrwng/build_controller_ha
  (dmcphers+openshiftbot@redhat.com)
- Update openshift-object-describer dependency to 0.0.9 (spadgett@redhat.com)
- add non-resource urls to role describer (deads@redhat.com)
- Ignore things that cannot be known in admission control (decarr@redhat.com)
- Merge pull request #2271 from deads2k/fix-role-binding-describer
  (dmcphers+openshiftbot@redhat.com)
- Prune builds (decarr@redhat.com)
- Merge pull request #2270 from smarterclayton/master_count
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2226 from deads2k/tighten-login-keys
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2191 from mfojtik/fix_sample_app_readme
  (dmcphers+openshiftbot@redhat.com)
- nits to config output (deads@redhat.com)
- fix rolebinding describer (deads@redhat.com)
- Merge pull request #2054 from gashcrumb/add-console-link
  (dmcphers+openshiftbot@redhat.com)
- Persist a master count config variable to enable multi-master
  (ccoleman@redhat.com)
- Add integration tests to validate build controller HA tolerance
  (cewong@redhat.com)
- Improve image change controller HA tolerance (cewong@redhat.com)
- osc login stanza names (deads@redhat.com)
- don't write project request template (deads@redhat.com)
- Merge pull request #2265 from kargakis/resizer-minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2261 from liggitt/http_only
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2259 from liggitt/cert_subject_alt_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2253 from csrwng/fix_newapp_streamref
  (dmcphers+openshiftbot@redhat.com)
- Copy sdn scripts into node subpackage (sdodson@redhat.com)
- Transition to Kube 1.0 DNS schema (ccoleman@redhat.com)
- Merge pull request #2151 from pravisankar/admin-manage-node
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2234 from smarterclayton/continue_on_error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2262 from liggitt/validate_user_update
  (dmcphers+openshiftbot@redhat.com)
- Add connect link to pod list for pods that expose jolokia port
  (slewis@fusesource.com)
- resize: Defer any kind except dcs to kubectl
  (kargakis@users.noreply.github.com)
- Move creation of Docker registry before osc login (mfojtik@redhat.com)
- Fix curl command in sample-app README (mfojtik@redhat.com)
- Merge pull request #2237 from kargakis/image-stream-name-fix
  (dmcphers+openshiftbot@redhat.com)
- Validate user and identity objects on update (jliggitt@redhat.com)
- Make csrf and session cookies httpOnly (jliggitt@redhat.com)
- Ensure real DNS subjectAltNames precede IP DNS subjectAltNames in generated
  certs (jliggitt@redhat.com)
- Deployment configs on the web console, with deployments grouped
  (contact@fabianofranz.com)
- Merge pull request #2250 from fabianofranz/bugs_1221041
  (dmcphers+openshiftbot@redhat.com)
- Add selector option to router. (smitram@gmail.com)
- Merge pull request #2252 from deads2k/fix-sdn-mapper
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2251 from sspeiche/nodejs-readme
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1686 from liggitt/groups
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2232 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2090 from nak3/bash-completion
  (dmcphers+openshiftbot@redhat.com)
- OpenShift admin command to manage node operations (rpenta@redhat.com)
- Fix new-app build config image reference (cewong@redhat.com)
- Bug 1221041 - fixes example in osc env (contact@fabianofranz.com)
- Merge pull request #2062 from mfojtik/etcd_example
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2157 from mfojtik/pull_secret
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2117 from csrwng/pod_exec_pf_proxy_policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2247 from ironcladlou/fix-deploy-strategy-defaulting
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2243 from smarterclayton/flagtypes_url
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2158 from kargakis/resizing-dcs
  (dmcphers+openshiftbot@redhat.com)
- update sdn types to work nicely in v1beta3 and osc (deads@redhat.com)
- Moved Node.js instructions to be with its own repo (sspeiche@redhat.com)
- Merge pull request #2244 from bparees/nested_err
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2246 from smarterclayton/fix_origin_release_image
  (dmcphers+openshiftbot@redhat.com)
- Added clustered etcd example (mfojtik@redhat.com)
- Add test, include groups from virtual users in users/~ (deads@redhat.com)
- Initial group support (jliggitt@redhat.com)
- fix vagrant-fedora21 networking issue; update openshift-sdn version
  (rchopra@redhat.com)
- Merge pull request #2239 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2241 from jcantrill/git2216_service_invalid_port_spec
  (dmcphers+openshiftbot@redhat.com)
- Add PullSecretName to v1beta1 (mfojtik@redhat.com)
- Set default policy for pod exec, port-forward, and proxy (cewong@redhat.com)
- Fix deploymentConfig defaulting (ironcladlou@gmail.com)
- log nested error (bparees@redhat.com)
- Fix the origin release image (ccoleman@redhat.com)
- flag.Addr should not default URLs to DefaultPort (ccoleman@redhat.com)
- Merge pull request #2098 from spadgett/navbar-wrapping
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2227 from ironcladlou/deploy-annotation-refactor
  (dmcphers+openshiftbot@redhat.com)
- in new-app flow: Fix where to get the port for the service, skip service
  generation if no port is exposed; fix the namespace use to check for resource
  existence (jcantril@redhat.com)
- UPSTREAM: Continue on errors in kubectl (ccoleman@redhat.com)
- Wrap navbar controls naturally for small screens (spadgett@redhat.com)
- Namespace deployment annotations for v1beta3 (ironcladlou@gmail.com)
- Remove entire os coverage package (dmcphers@redhat.com)
- Add osc resize (kargakis@users.noreply.github.com)
- Beef-up new-app help message (kargakis@users.noreply.github.com)
- Bug 1218971: Persist existing imageStream name in buildCofig
  (kargakis@users.noreply.github.com)
- Add support for PULL_DOCKERCFG_PATH to docker build strategy
  (mfojtik@redhat.com)
- Update strategies to inject pull dockercfg into builder containers
  (mfojtik@redhat.com)
- Card origin_devexp_567 - Add PullSecretName to all build strategies
  (mfojtik@redhat.com)
- Fix gofmt errors. (smitram@gmail.com)
- Add NodeSelector support to enable placement of router and ipfailover
  components. (smitram@gmail.com)
- Remove deployment followed by pull/1986 (nakayamakenjiro@gmail.com)
- Add bash-completion support (nakayamakenjiro@gmail.com)
- merge default config with user config (akostadi@redhat.com)

* Thu May 14 2015 Scott Dodson <sdodson@redhat.com> 0.5.1.1
- Kube 0.17 rebase
- sdn integration
* Thu May 14 2015 Scott Dodson <sdodson@redhat.com>
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2230 from csrwng/fix_build_output_conversion
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2128 from fabianofranz/614_deployment_history
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2228 from bparees/v1b1_conversion
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2229 from
  smarterclayton/more_aggressive_v1beta3_build_checks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2204 from derekwaynecarr/build_updates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2218 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2222 from deads2k/user-tilde
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2219 from spadgett/project-sort
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2150 from rajatchopra/sdn_integration
  (dmcphers+openshiftbot@redhat.com)
- Add support to deployments history in osc describe and deploy
  (contact@fabianofranz.com)
- add default conversion handling for imagestream kind (bparees@redhat.com)
- Merge pull request #2089 from kargakis/image-stream-fix
  (dmcphers+openshiftbot@redhat.com)
- Fix conversion of output.DockerImageReference on v1beta3 builds
  (cewong@redhat.com)
- Migrate build and build config to latest patterns in storage
  (decarr@redhat.com)
- Refactor to match upstream (ccoleman@redhat.com)
- UPSTREAM: print more useful error (ccoleman@redhat.com)
- UPSTREAM: implement a generic webhook storage (ccoleman@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: Ensure no namespace on create/update root scope types
  (jliggitt@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):25d32ee5132b41c122fe2929f3c6b
  e7c3eb74f1d (ccoleman@redhat.com)
- Be more aggressive about input and output on build From (ccoleman@redhat.com)
- Merge pull request #2221 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- update user/~ to return a user, even without an backing etcd entry
  (deads@redhat.com)
- Sort projects by display name (spadgett@redhat.com)
- Cleanup coverage profiles from output (dmcphers@redhat.com)
- SDN integration (rchopra@redhat.com)
- Bug 1218971: new-app: Create input imageStream
  (kargakis@users.noreply.github.com)

* Wed May 13 2015 Scott Dodson <sdodson@redhat.com> 0.5.1.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2202 from sdodson/fix-rpm-build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2213 from ironcladlou/deploy-type-descriptions
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2182 from kargakis/test-cmd-fixes
  (dmcphers+openshiftbot@redhat.com)
- Add descriptions to all API fields (ironcladlou@gmail.com)
- Merge pull request #2205 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Make stanzas in test-cmd.sh independent (kargakis@users.noreply.github.com)
- Remove cpu.pprof (ccoleman@redhat.com)
- Merge pull request #2203 from openshift/update_sti
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2193 from spadgett/hide-failed-pods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1986 from smarterclayton/make_v1beta3_default
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2127 from ironcladlou/rolling-deploy
  (dmcphers+openshiftbot@redhat.com)
- bump (github.com/openshift/source-to-
  image):14c0ebafd9875ddba45ad53c220c3886458eaa44 (bparees@redhat.com)
- Ensure ImageStreamTag is exposed in v1beta3 (ccoleman@redhat.com)
- Ensure build output in v1beta3 is in ImageStreamTag (ccoleman@redhat.com)
- Add processedtemplates to policy (ccoleman@redhat.com)
- Switch integration tests to v1beta3 (ccoleman@redhat.com)
- Allow POST on empty namespace (ccoleman@redhat.com)
- Switch default API version to v1beta3 (ccoleman@redhat.com)
- UPSTREAM: allow POST on all namespaces in v1beta3 (ccoleman@redhat.com)
- UPSTREAM: disable minions/status in v1 Kube api (ccoleman@redhat.com)
- Merge pull request #2148 from pravisankar/project-nodeenv
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2196 from ironcladlou/image-trigger-validation-fix
  (dmcphers+openshiftbot@redhat.com)
- Combine coverage (dmcphers@redhat.com)
- Merge pull request #2177 from ramr/dah2dit (dmcphers+openshiftbot@redhat.com)
- Get rid of overly complex loop in favor of two commands with local variables
  (sdodson@redhat.com)
- Merge pull request #2114 from spadgett/routes
  (dmcphers+openshiftbot@redhat.com)
- Added support for project node selector (rpenta@redhat.com)
- Accept ImageStreamTag as an image trigger kind (ironcladlou@gmail.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Implement a Rolling deployment strategy (ironcladlou@gmail.com)
- Merge pull request #2171 from deads2k/create-read-all-role
  (dmcphers+openshiftbot@redhat.com)
- Hide failed monopods in project overview (spadgett@redhat.com)
- Add config for number of cpus and memory (dmcphers@redhat.com)
- add cluster-reader role (deads@redhat.com)
- Merge pull request #2186 from kargakis/fix-template-labeling
  (dmcphers+openshiftbot@redhat.com)
- Show routes on project overview page (spadgett@redhat.com)
- Initialize object labels while processing a template
  (kargakis@users.noreply.github.com)
- Merge pull request #2181 from liggitt/karma_view_loading
  (dmcphers+openshiftbot@redhat.com)
- Fix error loading view during karma tests (jliggitt@redhat.com)
- Merge pull request #2156 from smarterclayton/describe_template_objects
  (dmcphers+openshiftbot@redhat.com)
- Allow process to output a template (ccoleman@redhat.com)
- Merge pull request #2175 from spadgett/hide-build-pods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2176 from bparees/merge_env
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2160 from soltysh/issue1353
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2170 from brenton/osadm1
  (dmcphers+openshiftbot@redhat.com)
- Hide build pods based on annotations (spadgett@redhat.com)
- Updated tests for generic webhook (maszulik@redhat.com)
- Added support for gogs webhooks in github webhook, also removed requirement
  for User-Agent (maszulik@redhat.com)
- Merge pull request #2168 from bparees/build_annotation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2167 from spadgett/empty-labels
  (dmcphers+openshiftbot@redhat.com)
- 'osadm create-api-client-config' was overwriting the client certificate with
  the CA (bleanhar@redhat.com)
- Merge pull request #2109 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Morse code change - dah to dit (dots). (smitram@gmail.com)
- annotate build pods (bparees@redhat.com)
- Only show 'None' when labels is empty (spadgett@redhat.com)
- merge env statements when concatenating (bparees@redhat.com)
- Merge pull request #2165 from liggitt/annotate_generated_host
  (dmcphers+openshiftbot@redhat.com)
- Annotate generated hosts (jliggitt@redhat.com)
- build openshift-pod rpm (tdawson@redhat.com)
- Merge pull request #2155 from smarterclayton/add_env_command
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2134 from kargakis/build-chain-repo-to-stream-nits
  (dmcphers+openshiftbot@redhat.com)
- Add `osc env` command for setting and reading env (ccoleman@redhat.com)
- Clean up various help commands (ccoleman@redhat.com)
- Merge pull request #2149 from rajatchopra/vagrant_fedora21
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2147 from spadgett/cancelled-builds
  (dmcphers+openshiftbot@redhat.com)
- Handle 'Cancelled' and 'Error' build status in the web console
  (spadgett@redhat.com)
- Card origin_devexp_286 - Added ssh key-based access to private git
  repository. (maszulik@redhat.com)
- Fixed godoc for push secrets. (maszulik@redhat.com)
- fix grep and insert hostnames (rchopra@redhat.com)
- Merge pull request #2102 from spadgett/sidebar-help
  (dmcphers+openshiftbot@redhat.com)
- vagrant changes to support fedora21 (rchopra@redhat.com)
- Merge pull request #2107 from akostadinov/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2126 from spadgett/empty-projects
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2133 from deads2k/update-policy-refs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2143 from bparees/dockercfg
  (dmcphers+openshiftbot@redhat.com)
- Preventing multiple deployments from running concurrently
  (abhgupta@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Empty state message for project overview (spadgett@redhat.com)
- Merge pull request #2138 from fabianofranz/issues_1509
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2106 from sosiouxme/20150506-osadm-certs
  (dmcphers+openshiftbot@redhat.com)
- update references to policy commands (deads@redhat.com)
- Merge pull request #2139 from csrwng/fix_build_watch
  (dmcphers+openshiftbot@redhat.com)
- Issue 1509 - fix usage error in openshift start (contact@fabianofranz.com)
- osadm certs: inform user of file writes (lmeyer@redhat.com)
- osadm certs: provide command descriptions (lmeyer@redhat.com)
- osadm create-master-certs: no overwrite by default (lmeyer@redhat.com)
- reduce visibility of dockercfg not found error (bparees@redhat.com)
- Merge pull request #2137 from deads2k/fix-osc-login-relativize
  (dmcphers+openshiftbot@redhat.com)
- Fix watching builds with fields filters (cewong@redhat.com)
- hack/common.sh uses the `which` command (akostadi@redhat.com)
- allow assigning floating IPs to openstack VMs (akostadi@redhat.com)
- VM prefixes for easy sharing AMZ and openstack accounts (akostadi@redhat.com)
- properly relativize paths (deads@redhat.com)
- Merge pull request #2136 from soltysh/fix_webhooks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2131 from pmorie/typo (dmcphers+openshiftbot@redhat.com)
- Removed requirement for content-type in generic webhook, and added testcase
  for gitlab webhook push event. (maszulik@redhat.com)
- build-chain: s/repository/stream (kargakis@users.noreply.github.com)
- Add resource type descriptions to details sidebar (spadgett@redhat.com)
- Grammar fix in README (pmorie@gmail.com)
- Merge pull request #2111 from smarterclayton/merge_edit
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Subresources inherit parent scope (ccoleman@redhat.com)
- Review comments (ccoleman@redhat.com)
- Detect and merge conflicts during osc edit (ccoleman@redhat.com)
- Exclude registered resources from latest.RESTMapper (ccoleman@redhat.com)
- UPSTREAM: print more useful error (ccoleman@redhat.com)
- UPSTREAM: legacy root scope should set NamespaceNone (ccoleman@redhat.com)
- UPSTREAM: SetNamespace should ignore root scopes (ccoleman@redhat.com)
- Perma-deflake TestDNS (ccoleman@redhat.com)
- Merge pull request #2099 from ncdc/fix-test-cmd-no-longer-delete-registry-pod
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2121 from csrwng/bug_1214229
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2082 from smarterclayton/webhooks_v1beta3
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2085 from stefwalter/sudo-readme-docker
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2120 from csrwng/minor_build_fixes
  (dmcphers+openshiftbot@redhat.com)
- Bug 1214229 - project admin cannot start build from another build
  (cewong@redhat.com)
- Add webhook to policy (ccoleman@redhat.com)
- Implement webhooks in v1beta3 (ccoleman@redhat.com)
- UPSTREAM: implement a generic webhook storage (ccoleman@redhat.com)
- change etcd paths (deads@redhat.com)
- Fix issues found by golint or govet within the build code (cewong@redhat.com)
- Merge pull request #2057 from deads2k/deads-create-cluster-policy-commands
  (dmcphers+openshiftbot@redhat.com)
- make project cache handle cluster policy (deads@redhat.com)
- Remove osadm symlinking from ose Dockerfile (sdodson@redhat.com)
- cluster policy commands (deads@redhat.com)
- split cluster policy from local policy (deads@redhat.com)
- Merge pull request #2101 from bparees/rebase_sti
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2093 from deads2k/lowercase-node-names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2086 from jimmidyson/route-objectmetadata
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2091 from deads2k/add-all-addresses-to-cert
  (dmcphers+openshiftbot@redhat.com)
- update for new STI request structure (bparees@redhat.com)
- Merge pull request #2097 from ironcladlou/recreate-preserve-scale-factor-2
  (dmcphers+openshiftbot@redhat.com)
- bump (github.com/openshift/source-to-
  image):e71f7da750b1d81285597afacea8e365a991f04d (bparees@redhat.com)
- Merge pull request #2078 from smarterclayton/change_base_paths
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2088 from ncdc/reenable-deploy-trigger-test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1767 from
  jcantrill/bug1211516_unable_to_create_service_when_create_from_source_console
  (dmcphers+openshiftbot@redhat.com)
- add all possible ips to the serving certs (deads@redhat.com)
- Merge pull request #906 from mfojtik/metadata_proposal
  (dmcphers+openshiftbot@redhat.com)
- Remove deletion of registry pod from test-cmd (agoldste@redhat.com)
- Merge pull request #1597 from nak3/defaultHostname
  (dmcphers+openshiftbot@redhat.com)
- Preserve scaling factor of prior deployment (ironcladlou@gmail.com)
- Merge pull request #1927 from mnagy/build_artifacts_links
  (dmcphers+openshiftbot@redhat.com)
- Change base paths for our resources (ccoleman@redhat.com)
- default node names always lowercase (deads@redhat.com)
- Merge pull request #2079 from deads2k/deads-fix-panic
  (dmcphers+openshiftbot@redhat.com)
- don't apply template if project exists (deads@redhat.com)
- Reenable deploy_trigger_test (agoldste@redhat.com)
- Use v1beta3 ObjectMeta for Route rather than internal representation
  (jimmidyson@gmail.com)
- Initial Image Metadata proposal (mfojtik@redhat.com)
- Merge pull request #1806 from smarterclayton/gitserver
  (dmcphers+openshiftbot@redhat.com)
- README.md: Use sudo in docker commands (stefw@redhat.com)
- Merge pull request #2046 from kargakis/describe-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2031 from sg00dwin/misc-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2064 from pweil-/BZ1218592
  (dmcphers+openshiftbot@redhat.com)
- Revert "Reenable deploy_trigger_test" (ccoleman@redhat.com)
- Merge pull request #2070 from ncdc/test-go-ignore-openshift-local
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2036 from ncdc/osadm-registry-require-client-cert-when-
  secure (dmcphers+openshiftbot@redhat.com)
- Implement a simple Git server for hosting repos (ccoleman@redhat.com)
- generic webhook should handle Git post-receive (ccoleman@redhat.com)
- bump(github.com/AaronO/go-git-http):0ebecedc64b67a3a8674c56724082660be48216e
  (ccoleman@redhat.com)
- Merge pull request #2069 from ncdc/reenable-deploy-trigger-test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2076 from spadgett/events
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2073 from bparees/imagestreamimage_conversion
  (dmcphers+openshiftbot@redhat.com)
- properly handle bad projectRequestTemplates (deads@redhat.com)
- Merge pull request #2056 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Add Browse -> Events page (spadgett@redhat.com)
- Wait for project to be deleted in hack/test-cmd.sh (ccoleman@redhat.com)
- Reenable deploy_trigger_test (agoldste@redhat.com)
- fix handling of imagestreamimage kinds in v1b1 (bparees@redhat.com)
- missed name when switching to new keys (pweil@redhat.com)
- OSE branding (jliggitt@redhat.com)
- Ignore openshift.local.* in test-go.sh (agoldste@redhat.com)
- Merge pull request #2060 from kargakis/imports-fix
  (dmcphers+openshiftbot@redhat.com)
- Add conversion.go for generating conversions (ccoleman@redhat.com)
- Fix grep for registry pod in e2e (agoldste@redhat.com)
- Refactor to match upstream changes (ccoleman@redhat.com)
- UPSTREAM: kube: register types with gob (deads@redhat.com)
- Merge pull request #1965 from mfojtik/zookeeper_example
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Encode binary assets in ASCII only (jliggitt@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- UPSTREAM: Ensure no namespace on create/update root scope types
  (jliggitt@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):c07896ee358addb44cc063eff3db1
  fcd0fe9767b (ccoleman@redhat.com)
- NodeController has been moved (ccoleman@redhat.com)
- Revert "Added support for project node selector" (ccoleman@redhat.com)
- Remove namespace conflict (kargakis@users.noreply.github.com)
- Use unified diff output for testing (nagy.martin@gmail.com)
- Link to the github page for github repositories (nagy.martin@gmail.com)
- Merge pull request #2059 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Rename no syncing var (dmcphers@redhat.com)
- Merge pull request #1998 from kargakis/new-app-fix
  (dmcphers+openshiftbot@redhat.com)
- Added support for project node selector (rpenta@redhat.com)
- Merge pull request #1981 from spadgett/not-a-function-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1991 from fabianofranz/issues_1677
  (dmcphers+openshiftbot@redhat.com)
- Fix not-a-function errors navigating between pages (spadgett@redhat.com)
- Merge pull request #1817 from pweil-/certmanager-test
  (dmcphers+openshiftbot@redhat.com)
- Separate 'Example' field in cli help (contact@fabianofranz.com)
- Make indentation consistent in help and usage (contact@fabianofranz.com)
- Adjust usage on several commands (contact@fabianofranz.com)
- Restores usage to cli templates (contact@fabianofranz.com)
- describe: name/namespace to namespace/name syntax fix
  (kargakis@users.noreply.github.com)
- new-app: Use the right image stream for the build from
  (kargakis@users.noreply.github.com)
- Add cursor pointer to service component (sgoodwin@redhat.com)
- osadm registry: require client cert when secure (agoldste@redhat.com)
- do not rewrite certs unless route has changed (pweil@redhat.com)
- refactor cert manager to allow testing, implement delete functionality
  (pweil@redhat.com)
- Add replicated zookeeper example (mfojtik@redhat.com)
- [BZ-1211516] Fix creating route even when choosing not to
  (jcantril@redhat.com)
- Revert to handle error without glog.Fatal (nakayamakenjiro@gmail.com)
- Replace hostname -f with unmae -n (nakayamakenjiro@gmail.com)

* Mon May 04 2015 Scott Dodson <sdodson@redhat.com> 0.5.0.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #2030 from deads2k/deads-move-policy-commands
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2055 from liggitt/proxy_default_port
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2047 from bparees/php_stream
  (dmcphers+openshiftbot@redhat.com)
- Use default port to proxy to backend (jliggitt@redhat.com)
- add php stream to repo list (bparees@redhat.com)
- Fix formatting and typos (dmcphers@redhat.com)
- Merge pull request #1925 from rhcarvalho/improve-cleanup-script
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1952 from thom311/vagrant-config-paths
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1941 from kargakis/expose
  (dmcphers+openshiftbot@redhat.com)
- Remove trailing spaces (rhcarvalho@gmail.com)
- Improve cleanup script (rhcarvalho@gmail.com)
- Fix formatting in sample-app/README.md (rhcarvalho@gmail.com)
- vagrant: fix creating minion config when provisioning master
  (thaller@redhat.com)
- Merge pull request #2042 from nak3/fix-gofmt
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2033 from deads2k/deads-make-new-project-show-nice-
  message (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1967 from csrwng/hide_template_images
  (dmcphers+openshiftbot@redhat.com)
- Fix gofmt complaint (nakayamakenjiro@gmail.com)
- Merge pull request #2040 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Merge pull request #2037 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2034 from deads2k/deads-allow-access-to-secrets
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2035 from ncdc/allow-system-registry-to-create-image-
  streams (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Reject unbounded cpu and memory pods if quota is restricting it
  #7003 (decarr@redhat.com)
- Merge pull request #1792 from sallyom/bundle_secret
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2011 from deads2k/deads-request-project-template
  (dmcphers+openshiftbot@redhat.com)
- grant admin and edit access to secrets (deads@redhat.com)
- Allow registry to auto-provision image streams (agoldste@redhat.com)
- Bug 1214548 (somalley@redhat.com)
- make the new-project cli display the correct message on denial
  (deads@redhat.com)
- refactor bulk to take the interfaces it needs, not factory (deads@redhat.com)
- use a template for requested projects (deads@redhat.com)
- fix template processing to handle slices (deads@redhat.com)
- promote commands out of experimental (deads@redhat.com)
- Merge pull request #2012 from csrwng/log_policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2028 from ironcladlou/deploy-cmd-output-fix
  (dmcphers+openshiftbot@redhat.com)
- Add pod logs and build logs to default policy constants (cewong@redhat.com)
- Fix minor deploy cmd typo (ironcladlou@gmail.com)
- Merge pull request #1972 from liggitt/details_sidebar
  (dmcphers+openshiftbot@redhat.com)
- Revert "Reverting the v2 registry switch" (sdodson@redhat.com)
- object-describer 0.0.7, build bindata (jliggitt@redhat.com)
- Restyle of overview elements (sgoodwin@redhat.com)
- Object sidebar (jforrest@redhat.com)
- Add word-break mixin (sgoodwin@redhat.com)
- Highlight elements that are currently selected (jforrest@redhat.com)
- Style changes (sgoodwin@redhat.com)
- Object sidebar (jforrest@redhat.com)
- change policy binding names (deads@redhat.com)
- add cluster policy resources (deads@redhat.com)
- Merge pull request #2022 from ncdc/ist-image-not-found
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1994 from smarterclayton/start-build-webhooks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2023 from smarterclayton/new_app_doesnt_need_pull
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2015 from deads2k/deads-respect-remote-kubeconfig
  (dmcphers+openshiftbot@redhat.com)
- Generate webhook URLs from the client (ccoleman@redhat.com)
- Expose (broken) webhooks to v1beta3 (ccoleman@redhat.com)
- osc start-build should be able to trigger webhook (ccoleman@redhat.com)
- UPSTREAM: Allow URL to be generated by request.go (ccoleman@redhat.com)
- Merge pull request #2019 from smarterclayton/invert_router_registry_defaults
  (dmcphers+openshiftbot@redhat.com)
- osc new-app shouldn't pull when image is chained (ccoleman@redhat.com)
- Improve ISTag image not found error response (agoldste@redhat.com)
- Merge pull request #2020 from bparees/generator_logging
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1997 from bparees/update_templates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2014 from ncdc/fix-test-cmd-delete-registry-pod-race
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2013 from ironcladlou/hook-resource-inheritance
  (dmcphers+openshiftbot@redhat.com)
- Invert the default behavior for osadm registry/router (ccoleman@redhat.com)
- Merge pull request #1868 from sallyom/DisplayNameChange
  (dmcphers+openshiftbot@redhat.com)
- add tracing in resolvimagestreamreference (bparees@redhat.com)
- Merge pull request #1934 from derekwaynecarr/resource_requirements_deployment
  (dmcphers+openshiftbot@redhat.com)
- properly handle external kube proxy case (deads@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- freshen template syntax (bparees@redhat.com)
- Merge pull request #2002 from deads2k/deads-allow-node-start
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1966 from kargakis/is-tag-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2008 from ncdc/tag-tracking
  (dmcphers+openshiftbot@redhat.com)
- Use a safer delete of registry pod in test-cmd (agoldste@redhat.com)
- Deployment hooks inherit resources and working dir (ironcladlou@gmail.com)
- Merge pull request #2010 from ncdc/fix-build-imagechangecontroller-
  updateConfig-panic (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2009 from ncdc/bump-spdystream
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #2007 from ironcladlou/hook-environment-inheritance
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1971 from nak3/add-list-project
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1987 from smarterclayton/set_maxproces
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Handle conversion of boolean query parameters with a value of
  'false' (cewong@redhat.com)
- Allow 1 ImageStream tag to track another (agoldste@redhat.com)
- Merge pull request #1886 from csrwng/build_log_subresource
  (dmcphers+openshiftbot@redhat.com)
- Fix build image change controller possible panic (agoldste@redhat.com)
- Merge pull request #1948 from deads2k/deads-project-request-information
  (dmcphers+openshiftbot@redhat.com)
- bump(docker/spdystream):e372247595b2edd26f6d022288e97eed793d70a2
  (agoldste@redhat.com)
- Merge pull request #2004 from deads2k/deads-fix-test-cmd
  (dmcphers+openshiftbot@redhat.com)
- Make hook containers inherit environment (ironcladlou@gmail.com)
- Add long help to osc project (nakayamakenjiro@gmail.com)
- projectrequest get message (deads@redhat.com)
- explicitly set the project to avoid depending on project cache refresh
  (deads@redhat.com)
- if AllowDisabledDocker, allow docker to be missing (deads@redhat.com)
- gofmt fix (kargakis@users.noreply.github.com)
- new-app: Add image tag in generated imageStream
  (kargakis@users.noreply.github.com)
- Merge pull request #1983 from bparees/sti_env
  (dmcphers+openshiftbot@redhat.com)
- Create build log subresource in the style of upstream pod log
  (cewong@redhat.com)
- Merge pull request #1979 from smarterclayton/return_template_from_process
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Use Pod.Spec.Host instead of Pod.Status.HostIP for pod subresources
  (cewong@redhat.com)
- Merge pull request #1935 from sdodson/only-use-nodeconfig
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1984 from smarterclayton/v1beta3_builds
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1982 from deads2k/deads-osc-logout
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1973 from deads2k/deads-make-remove-user-predictable
  (dmcphers+openshiftbot@redhat.com)
- Dump metrics and run a CPU profile in hack/test-cmd (ccoleman@redhat.com)
- Set GOMAXPROCS by default (ccoleman@redhat.com)
- Merge pull request #1947 from ironcladlou/deploy-command
  (dmcphers+openshiftbot@redhat.com)
- Return a template object from template process (ccoleman@redhat.com)
- Merge pull request #1978 from csrwng/switch_pod_log
  (dmcphers+openshiftbot@redhat.com)
- only allow trusted env variables in sti builder container
  (bparees@redhat.com)
- Add v1beta3 cut of builds (ccoleman@redhat.com)
- Cleanup sysconfig values as everything is now in a config file
  (sdodson@redhat.com)
- add logout command (deads@redhat.com)
- Merge pull request #1970 from ncdc/remove-old-registry-configs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1974 from sdodson/rpm-packaging-updates
  (dmcphers+openshiftbot@redhat.com)
- Force no color in grep (ccoleman@redhat.com)
- Merge pull request #1693 from bparees/image_refs
  (dmcphers+openshiftbot@redhat.com)
- Add a deploy command (ironcladlou@gmail.com)
- UPSTREAM: Switch kubelet log command to use pod log subresource
  (cewong@redhat.com)
- Merge pull request #1959 from smarterclayton/fix_templates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1957 from spadgett/fix-templates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1928 from spadgett/empty-ports-array
  (dmcphers+openshiftbot@redhat.com)
- [RPMs] Move clients to common package and add dockerregistry subpkg
  (sdodson@redhat.com)
- force predictable ordering of removing users from a project
  (deads@redhat.com)
- refactor ict to remove from/image parameters (bparees@redhat.com)
- Replace docker-registry-*.json with osadm registry (agoldste@redhat.com)
- Merge pull request #1969 from ncdc/disable-registry-probe
  (dmcphers+openshiftbot@redhat.com)
- Use true generic objects in templates (ccoleman@redhat.com)
- Match changes to make []runtime.Object cleaner (ccoleman@redhat.com)
- UPSTREAM: Support unknown types more cleanly (ccoleman@redhat.com)
- UPSTREAM: add HasAny (deads@redhat.com)
- Merge pull request #1929 from nak3/fixed-osc-project
  (dmcphers+openshiftbot@redhat.com)
- Hide images section when no images detected in template (cewong@redhat.com)
- Disable registry liveness probe to support v1 & v2 (agoldste@redhat.com)
- Web Console: Fix problems creating from templates (spadgett@redhat.com)
- WIP: displayName annotation (somalley@redhat.com)
- Fixed the sample-app readme (maszulik@redhat.com)
- Fix error handling of osc project (nakayamakenjiro@gmail.com)
- Update console for v2 image IDs (agoldste@redhat.com)
- Enable building v2 registry image (agoldste@redhat.com)
- Add info about Docker 1.6 to README (agoldste@redhat.com)
- [RPMs] Require docker >= 1.6.0 (sdodson@redhat.com)
- Require Docker 1.6 for node startup (agoldste@redhat.com)
- v2 registry updates (agoldste@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- bump(docker/distribution):62b70f951f30a711a8a81df1865d0afeeaaa0169
  (agoldste@redhat.com)
- Merge pull request #1917 from jwhonce/wip/bootstrap
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1943 from deads2k/deads-fix-file-refs
  (dmcphers+openshiftbot@redhat.com)
- Pass --force to docker push if needed (agoldste@redhat.com)
- Fix client builds for windows and macosx (sdodson@redhat.com)
- Work around 64MB tmpfs limit (agoldste@redhat.com)
- UPSTREAM: describe: Support resource type/name syntax
  (kargakis@users.noreply.github.com)
- Add Deployment to v1beta3 (ccoleman@redhat.com)
- doc: fix instructions in example doc (mjisyang@gmail.com)
- Fix typo (rhcarvalho@gmail.com)
- Change available loglevel with openshift (nakayamakenjiro@gmail.com)
- default certificate support (pweil@redhat.com)
- Fixes as per @smarterclayton review comments. (smitram@gmail.com)
- Rename from ha*config to ipfailover. Fixes and cleanup as per @smarterclayton
  review comments. (smitram@gmail.com)
- Complete --delete functionality. (smitram@gmail.com)
- fix demo config (smitram@gmail.com)
- Convert to plugin code, add tests and use replica count in the keepalived
  config generator. (smitram@gmail.com)
- Checkpoint ha-config work. Add HostNetwork capabilities. Add volume mount to
  container. (smitram@gmail.com)
- Add HA configuration proposal. (smitram@gmail.com)
- Add new ha-config keepalived failover service container code, image and
  scripts. (smitram@gmail.com)
- Hide ugly output from osc status --help (nakayamakenjiro@gmail.com)
- Use unminified css on login page (jliggitt@redhat.com)
- sort projects (somalley@redhat.com)
- use CheckErr for pretty messages (deads@redhat.com)
- Add route to v1beta3 (ccoleman@redhat.com)
- add osadm to docker image (deads@redhat.com)
- Bug 1215014 - hostname for the node a pod is on is showing up as unknown due
  to an api change (jforrest@redhat.com)
- Embed the openshift-jvm console (jforrest@redhat.com)
- Added Node.js simple example workflow (sspeiche@redhat.com)
- migrate roles and bindings to new storage (deads@redhat.com)
- UPSTREAM: cobra loses arguments with same value as subcommand
  (deads@redhat.com)
- allow login with file that doesn't exist yet (deads@redhat.com)
- make request-project switch projects (deads@redhat.com)
- make project delegatable (deads@redhat.com)
- Initial work on multi-context html5 handling (jliggitt@redhat.com)
- * Add support for local-source configuration (jhonce@redhat.com)
- Merge pull request #1945 from soltysh/issue1480
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1747 from kargakis/new-app-fixes
  (dmcphers+openshiftbot@redhat.com)
- bad file references due to moved config (deads@redhat.com)
- Issue 1480 - switched to using goautoneg package for parsing accept headers
  (maszulik@redhat.com)
- Merge pull request #1914 from jwforres/data_icons
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1932 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Fix getting services in expose cmd
  (kargakis@users.noreply.github.com)
- Tests for new-app (kargakis@users.noreply.github.com)
- Rework new-app (kargakis@users.noreply.github.com)
- Merge pull request #1835 from jhadvig/start_build_ui
  (dmcphers+openshiftbot@redhat.com)
- Remove experimental generate command (kargakis@users.noreply.github.com)
- Merge pull request #1933 from deads2k/deads-change-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1936 from fabianofranz/bump_cobra_and_pflag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1737 from deads2k/deads-openshift-start-take-dir
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: cobra loses arguments with same value as subcommand
  (deads@redhat.com)
- bump(github.com/spf13/cobra):ebb2d55f56cfec37ad899ad410b823805cc38e3c
  (contact@fabianofranz.com)
- bump(github.com/spf13/pflag):60d4c375939ff7ba397a84117d5281256abb298f
  (contact@fabianofranz.com)
- make openshift start --write-config take a dir (deads@redhat.com)
- Merge pull request #1831 from mjisyang/doc-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1779 from pweil-/router-wildcard-cert
  (dmcphers+openshiftbot@redhat.com)
- Add resource requirements to deployment strategy (decarr@redhat.com)
- don't let editors delete a project (deads@redhat.com)
- Show icons embedded as data uris in template and image catalogs in console
  (jforrest@redhat.com)
- Merge pull request #1881 from sspeiche/node-echo
  (dmcphers+openshiftbot@redhat.com)
- Fix formatting (dmcphers@redhat.com)
- Handle portalIP 'None' and empty service.spec.ports (spadgett@redhat.com)
- Start/Rerun a build from the console (j.hadvig@gmail.com)
- Work around 64MB tmpfs limit (agoldste@redhat.com)
- Merge pull request #1930 from kargakis/upstream-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1915 from smarterclayton/add_deploy_to_v1beta3
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: describe: Support resource type/name syntax
  (kargakis@users.noreply.github.com)
- Add Deployment to v1beta3 (ccoleman@redhat.com)
- doc: fix instructions in example doc (mjisyang@gmail.com)
- Merge pull request #1807 from ramr/haconfig
  (dmcphers+openshiftbot@redhat.com)
- Fix typo (rhcarvalho@gmail.com)
- Change available loglevel with openshift (nakayamakenjiro@gmail.com)
- default certificate support (pweil@redhat.com)
- Fixes as per @smarterclayton review comments. (smitram@gmail.com)
- Rename from ha*config to ipfailover. Fixes and cleanup as per @smarterclayton
  review comments. (smitram@gmail.com)
- Complete --delete functionality. (smitram@gmail.com)
- fix demo config (smitram@gmail.com)
- Convert to plugin code, add tests and use replica count in the keepalived
  config generator. (smitram@gmail.com)
- Checkpoint ha-config work. Add HostNetwork capabilities. Add volume mount to
  container. (smitram@gmail.com)
- Add HA configuration proposal. (smitram@gmail.com)
- Add new ha-config keepalived failover service container code, image and
  scripts. (smitram@gmail.com)
- Hide ugly output from osc status --help (nakayamakenjiro@gmail.com)
- Merge pull request #1907 from deads2k/deads-use-checkerr
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1916 from liggitt/login_html
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1911 from sallyom/listSortedProjects
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1906 from smarterclayton/add_route_v1beta3
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1904 from jwforres/bug_1215014_pod_node_hostname
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1838 from jwforres/subconsole_support
  (dmcphers+openshiftbot@redhat.com)
- Use unminified css on login page (jliggitt@redhat.com)
- sort projects (somalley@redhat.com)
- Merge pull request #1905 from deads2k/deads-osadm
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1840 from deads2k/deads-switch-projects
  (dmcphers+openshiftbot@redhat.com)
- use CheckErr for pretty messages (deads@redhat.com)
- Merge pull request #1901 from deads2k/deads-osadm-not-openshift-admin
  (dmcphers+openshiftbot@redhat.com)
- Add route to v1beta3 (ccoleman@redhat.com)
- add osadm to docker image (deads@redhat.com)
- Bug 1215014 - hostname for the node a pod is on is showing up as unknown due
  to an api change (jforrest@redhat.com)
- Merge pull request #1873 from deads2k/deads-migrate-roles-to-new-registry
  (dmcphers+openshiftbot@redhat.com)
- Embed the openshift-jvm console (jforrest@redhat.com)
- Added Node.js simple example workflow (sspeiche@redhat.com)
- migrate roles and bindings to new storage (deads@redhat.com)
- UPSTREAM: cobra loses arguments with same value as subcommand
  (deads@redhat.com)
- allow login with file that doesn't exist yet (deads@redhat.com)
- make request-project switch projects (deads@redhat.com)
- make project delegatable (deads@redhat.com)
- Initial work on multi-context html5 handling (jliggitt@redhat.com)

* Fri Apr 24 2015 Scott Dodson <sdodson@redhat.com> 0.4.4.0
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1898 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Normalize to lower resource names in quota admission
  (decarr@redhat.com)
- [RPMs] Add socat and util-linux dependencies for node (sdodson@redhat.com)
- Merge pull request #1888 from smarterclayton/private_repo_problems
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1869 from deads2k/deads-change-login
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1874 from derekwaynecarr/deployment_events
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1882 from spadgett/multi-port-services
  (dmcphers+openshiftbot@redhat.com)
- make osc login more userfriendly (deads@redhat.com)
- Merge pull request #1887 from jwforres/deployments_js_error
  (dmcphers+openshiftbot@redhat.com)
- Produce events from deployment in failure scenarios (decarr@redhat.com)
- Deployments page throws JS error, still referencing /images when should be
  getting imageStreams (jforrest@redhat.com)
- Allow insecure registries for image import (ccoleman@redhat.com)
- Allow image specs of form foo:500/bar (ccoleman@redhat.com)
- Merge pull request #1884 from ironcladlou/deploy-trigger-validation
  (dmcphers+openshiftbot@redhat.com)
- Adopt multi-port services changes (spadgett@redhat.com)
- Merge pull request #1849 from pweil-/haproxy-stats
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1883 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- Validate ImageStreams in triggers (ironcladlou@gmail.com)
- Merge pull request #1821 from derekwaynecarr/build_resource_requirements
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1827 from derekwaynecarr/build_controller_events
  (dmcphers+openshiftbot@redhat.com)
- enable stats listener for haproxy (pweil@redhat.com)
- UPSTREAM Fix nil pointer that can happen if no container resources are
  supplied (decarr@redhat.com)
- Merge pull request #1793 from fabianofranz/issues_1562
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1847 from deads2k/deads-prevent-slashes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1866 from smarterclayton/misc_cleanup
  (dmcphers+openshiftbot@redhat.com)
- Issue 1562 - relativize paths in client config file
  (contact@fabianofranz.com)
- Merge pull request #1872 from fabianofranz/bugs_1213648
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1864 from deads2k/deads-fix-test-integration
  (dmcphers+openshiftbot@redhat.com)
- Bug 1213648 - switch to a valid project after logging in
  (contact@fabianofranz.com)
- more tests (deads@redhat.com)
- Merge pull request #1783 from jhadvig/fix_url
  (dmcphers+openshiftbot@redhat.com)
- make test-integration.sh allow single test (deads@redhat.com)
- add imagestream validation (deads@redhat.com)
- Merge pull request #1858 from nak3/add-osadm-to-binary
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1867 from ncdc/temporarily-disable-building-v2-registry-
  image (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1837 from deads2k/deads-fix-forbidden
  (dmcphers+openshiftbot@redhat.com)
- Disable building v2 registry image (agoldste@redhat.com)
- Fix URL for build webhooks docs in console + README (j.hadvig@gmail.com)
- Build controller logs events when failing to create pods (decarr@redhat.com)
- Forbidden resources should not break new-app (ccoleman@redhat.com)
- The order of "all" should be predictable (ccoleman@redhat.com)
- Add resource requirements to build parameters (decarr@redhat.com)
- try to match forbidden status to api version request (deads@redhat.com)
- Fix gofmt complaint (nakayamakenjiro@gmail.com)
- Add osadm to tarball (nakayamakenjiro@gmail.com)
- Merge pull request #1845 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1839 from pweil-/multiport-hello-openshift
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1624 from soltysh/post_1609
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1815 from kargakis/move-default-tag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1663 from ejemba/ifconfig-1-4-fix
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM Fixup event object reference generation to allow downstream objects
  (decarr@redhat.com)
- Merge pull request #1843 from deads2k/deads-fix-templates
  (dmcphers+openshiftbot@redhat.com)
- sample template messed up with v1beta3 (deads@redhat.com)
- Merge pull request #1842 from bparees/db_templates
  (dmcphers+openshiftbot@redhat.com)
- Remove /tmp/openshift from README (ccoleman@redhat.com)
- correct registry and repository name (bparees@redhat.com)
- serve on multiple ports (pweil@redhat.com)
- Merge remote-tracking branch 'upstream/pr/1786' (sdodson@redhat.com)
- Merge pull request #1836 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM cherry pick make describeEvents DescribeEvents (decarr@redhat.com)
- add self-provisioned newproject (deads@redhat.com)
- Merge pull request #1797 from smarterclayton/round_trip_test
  (dmcphers+openshiftbot@redhat.com)
- Implement test config (ccoleman@redhat.com)
- Merge pull request #1833 from deads2k/deads-remove-local
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin/master' into round_trip_test
  (ccoleman@redhat.com)
- Merge pull request #1754 from deads2k/deads-move-to-registry
  (dmcphers+openshiftbot@redhat.com)
- migrate policy to new rest storage (deads@redhat.com)
- remove local .openshiftconfig (deads@redhat.com)
- Issue #1488 - added build duration and tweaked build counting in describer
  (maszulik@redhat.com)
- Merge pull request #1802 from liggitt/oauthtoken_policy_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1823 from fabianofranz/issues_1812
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1822 from derekwaynecarr/cherry_picks
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1819 from deads2k/deads-project-admin-create-endpoints
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Allow resource builder to avoid fetching objects
  (jliggitt@redhat.com)
- Improve output of who-can command, fix oauthtokens bootstrap policy
  (jliggitt@redhat.com)
- Prepare v1beta3 Template types (ccoleman@redhat.com)
- Issue 1812 - subcommand 'options' must be exposed in every command that uses
  the main template (contact@fabianofranz.com)
- UPSTREAM Fix nil pointer in limit ranger (decarr@redhat.com)
- Add support to osc new-app from stored template (contact@fabianofranz.com)
- Merge pull request #1698 from deads2k/deads-upstream-6680
  (dmcphers+openshiftbot@redhat.com)
- allow project admins to modify endpoints (deads@redhat.com)
- Merge pull request #827 from mfojtik/generic_sub
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1798 from smarterclayton/fix_http_registry
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- change openshiftconfig loading rules and files (deads@redhat.com)
- Merge pull request #1801 from smarterclayton/add_build_revision_info
  (dmcphers+openshiftbot@redhat.com)
- Replace all hardcoded default tags with DefaultImageTag
  (kargakis@users.noreply.github.com)
- Allow protocol prefixed docker pull specs (ccoleman@redhat.com)
- Merge pull request #1808 from bparees/user_env
  (dmcphers+openshiftbot@redhat.com)
- Support parameter substitution for all string fields (mfojtik@redhat.com)
- copy env variables into sti execution environment (bparees@redhat.com)
- Improve display of standalone build configs (ccoleman@redhat.com)
- Merge pull request #1786 from pweil-/BZ-1202296
  (dmcphers+openshiftbot@redhat.com)
- Add build revision info to osc status (ccoleman@redhat.com)
- UPSTREAM: Suppress aggressive output of warning (ccoleman@redhat.com)
- Internal rename to v1beta3 (ccoleman@redhat.com)
- Update internal to v1beta3 for parity with Kube (ccoleman@redhat.com)
- Add round trip tests for images (ccoleman@redhat.com)
- Merge pull request #1795 from smarterclayton/use_extensions_for_edit
  (dmcphers+openshiftbot@redhat.com)
- Slightly simplify status output for service (ccoleman@redhat.com)
- Use extensions when editing a file (ccoleman@redhat.com)
- Merge pull request #1794 from fabianofranz/master
  (dmcphers+openshiftbot@redhat.com)
- Fix issue in error reason detection (contact@fabianofranz.com)
- Merging the latest for beta3 (bleanhar@redhat.com)
- Merge pull request #1784 from csrwng/proxy_params
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1780 from smarterclayton/fix_1201615_https_in_swagger
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: allow multiple changes in modifyconfig (deads@redhat.com)
- UPSTREAM: change kubeconfig loading order and update filename
  (deads@redhat.com)
- Merge pull request #1788 from deads2k/deads-resync-api-request-resolver
  (dmcphers+openshiftbot@redhat.com)
- Bug 1201615 - Give swagger the correct base URL (ccoleman@redhat.com)
- Merge pull request #1777 from liggitt/google_hosted_domain
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1741 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1776 from bparees/db_templates
  (dmcphers+openshiftbot@redhat.com)
- Exposing the pod manifest file/dir option for the node in Origin
  (abhgupta@redhat.com)
- add db sample templates (bparees@redhat.com)
- UPSTREAM: resync api request resolver (deads@redhat.com)
- Add option to restrict google logins to custom domains (jliggitt@redhat.com)
- Merge pull request #1781 from mnagy/fix_tmp_path
  (dmcphers+openshiftbot@redhat.com)
- backends will be named based on the route ns/name, not the service name
  (pweil@redhat.com)
- Merging the lastest from upstream for beta3 (bleanhar@redhat.com)
- UPSTREAM: Add URL parameters to proxy redirect Location header
  (cewong@redhat.com)
- missing json tag (deads@redhat.com)
- Remove trailing } (nagy.martin@gmail.com)
- Merge pull request #1758 from smarterclayton/add_an_edit
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1768 from pweil-/router-tls-validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1763 from rhcarvalho/cleanup-contrib-docs
  (dmcphers+openshiftbot@redhat.com)
- Check /dev/tty if STDIN is not a tty (ccoleman@redhat.com)
- Add an edit command (ccoleman@redhat.com)
- stronger validation for tls termination type (pweil@redhat.com)
- Merge pull request #1762 from mnagy/use_openshift_mysql_image
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1774 from ncdc/fix-build-image-change-trigger
  (dmcphers+openshiftbot@redhat.com)
- Don't trigger a build if the namespaces differ (agoldste@redhat.com)
- Merge pull request #1635 from kargakis/comment-fix
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: It should be possible to have lists with mixed versions
  (ccoleman@redhat.com)
- Take storage version for kube/os as config params (ccoleman@redhat.com)
- First cut of resources without significant changes (ccoleman@redhat.com)
- Initial v1beta2 (experimental) cut (ccoleman@redhat.com)
- Merge pull request #1769 from jcantrill/bug1212362_router_doesnt_work
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: create parent dir structure when saving config to file
  (contact@fabianofranz.com)
- [BZ-1212362] Remove tls block for unsecure router generation from web console
  (jcantril@redhat.com)
- Some commenting here and there (kargakis@users.noreply.github.com)
- Merge pull request #1765 from deads2k/deads-disallow-empty-node-names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1750 from kargakis/use-docker-parser
  (dmcphers+openshiftbot@redhat.com)
- disallow empty static node names (deads@redhat.com)
- Merge pull request #1759 from ncdc/bump-cadvisor
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1755 from deads2k/deads-start-simplifying-loading
  (dmcphers+openshiftbot@redhat.com)
- generate: Use Docker parser for validation
  (kargakis@users.noreply.github.com)
- change KUBECONFIG references to OPENSHIFTCONFIG (deads@redhat.com)
- restrict openshift loading chain to only openshift (deads@redhat.com)
- Use our mysql image (nagy.martin@gmail.com)
- Remove misleading comment from docs (rhcarvalho@gmail.com)
- Bump cadvisor, libcontainer (agoldste@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Bug 1211235 - Validate user against OpenShift in case of docker login for v2
  registry (rpenta@redhat.com)
- UPSTREAM: allow multiple changes in modifyconfig (deads@redhat.com)
- Merge pull request #1752 from liggitt/bundle_secret
  (dmcphers+openshiftbot@redhat.com)
- bundle-secret updates (jliggitt@redhat.com)
- Merge pull request #1749 from csrwng/fix_pod_logs
  (dmcphers+openshiftbot@redhat.com)
- Reverting the v2 registry switch (bleanhar@redhat.com)
- Merge pull request #1732 from sg00dwin/builder-image-tile-clear-Bug-1211210
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1726 from
  jcantrill/bug1211516_unable_to_create_service_when_create_from_source_console
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Fix a small regression on api server proxy after switch to v1beta3.
  #6701 (cewong@redhat.com)
- Merge pull request #1518 from sallyom/bundle_secret
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1731 from liggitt/no_namespace_oauth
  (dmcphers+openshiftbot@redhat.com)
- Removing unnecessary code (bleanhar@redhat.com)
- Updating the ose image build scripts. (bleanhar@redhat.com)
- Docker Registry v2 sub rpm (bleanhar@redhat.com)
- Merge remote-tracking branch 'upstream/pr/1748' (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1739 from deads2k/deads-upstream-6761
  (dmcphers+openshiftbot@redhat.com)
- [RPMs] Require docker >= 1.6.0 (sdodson@redhat.com)
- Convert OAuth registries to generic etcd (jliggitt@redhat.com)
- Merge pull request #1670 from kargakis/build-chain-dot-fix
  (dmcphers+openshiftbot@redhat.com)
- WIP: bundle-secret per feedback/rebase error (somalley@redhat.com)
- UPSTREAM: add flattening and minifying options to config view #6761
  (deads@redhat.com)
- Fix for Bug 1211210 in the ui where the tile list for builder images don't
  clear correctly in some situations. Remove animation transition css from
  _core, import new _component-animations and scope rule to .show-hide class
  (sgoodwin@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1729 from fabianofranz/bump_cobra
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1582 from sdodson/cross-platform
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1720 from liggitt/token_expiry_display
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1722 from liggitt/integration_bind_addrs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1717 from liggitt/basicauthpassword_keys
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1645 from ironcladlou/deployment-hooks-2-electric-
  boogaloo (dmcphers+openshiftbot@redhat.com)
- bump(github.com/spf13/cobra): 9cb5e8502924a8ff1cce18a9348b61995d7b4fde
  (contact@fabianofranz.com)
- bump(github.com/spf13/pflag): 18d831e92d67eafd1b0db8af9ffddbd04f7ae1f4
  (contact@fabianofranz.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1680 from pweil-/separate-edge-tls
  (dmcphers+openshiftbot@redhat.com)
- [BUG 1211516] Fix service definition generation for v1beta3 when creating an
  app from source in the web console (jcantril@redhat.com)
- Merge pull request #1651 from deads2k/deads-change-bootstrap-policy
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1644 from kargakis/readme-links-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1711 from deads2k/deads-eliminate-basic-auth-flags
  (dmcphers+openshiftbot@redhat.com)
- Show token expiration times (jliggitt@redhat.com)
- remove --username and --password from most osc commands (deads@redhat.com)
- Merge pull request #1713 from derekwaynecarr/cache_updates
  (dmcphers+openshiftbot@redhat.com)
- Fix 'address already in use' errors in integration tests
  (jliggitt@redhat.com)
- Merge pull request #1719 from liggitt/access_token_timeout
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1715 from liggitt/windows_terminal
  (dmcphers+openshiftbot@redhat.com)
- Lengthen default access token lifetime to 1 day (jliggitt@redhat.com)
- Update keys for basicauth URL IDP to use OpenID standard claim names
  (jliggitt@redhat.com)
- Suppress swagger debug output (ccoleman@redhat.com)
- Bug 1209774: Trim windows newlines correctly (jliggitt@redhat.com)
- [RPMs] Generrate cross platform clients for MacOSX and Windows
  (sdodson@redhat.com)
- Add missing omitempty tags (ironcladlou@gmail.com)
- Minimize CPU churn in auth cache, re-enable integration test check
  (decarr@redhat.com)
- UPSTREAM: allow selective removal of kubeconfig override flags #6768
  (deads@redhat.com)
- Add e2e test coverage (ironcladlou@gmail.com)
- WIP (ironcladlou@gmail.com)
- Fix tests to be ifconfig 1.4.X compatible (epo@jemba.net)
- Implement deployment hooks (ironcladlou@gmail.com)
- separate tls and non-tls backend lookups (pweil@redhat.com)
- build-chain: Print only DOT output when specified
  (kargakis@users.noreply.github.com)
- allow everyone get,list access on the image group in openshift
  (deads@redhat.com)
- Fix loose links in README (kargakis@users.noreply.github.com)

* Mon Apr 13 2015 Scott Dodson <sdodson@redhat.com> 0.4.3.2
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1309 from sdodson/tito-tagging
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1709 from ncdc/1701-return-correct-type-for-
  imagestreamtag-imagestreamimage (dmcphers+openshiftbot@redhat.com)
- Remove all calls to the /images endpoint (jforrest@redhat.com)
- Correct kind, name, selfLink for ISTag, ISImage (agoldste@redhat.com)
- Merge pull request #1703 from kargakis/test-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1695 from sdodson/syslog-identifier
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1700 from jwforres/default_limit_range
  (dmcphers+openshiftbot@redhat.com)
- Add limit range defaults to settings page (jforrest@redhat.com)
- Merge pull request #1678 from csrwng/newapp_prevent_dups
  (dmcphers+openshiftbot@redhat.com)
- build-chain: Fix test names for split (kargakis@users.noreply.github.com)
- Merge pull request #1684 from liggitt/csrf_basic_auth
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1682 from liggitt/oauth_error_handling
  (dmcphers+openshiftbot@redhat.com)
- Automatic commit of package [openshift] release [0.4.3.1].
  (sdodson@redhat.com)
- Create from template fails with JS error (jforrest@redhat.com)
- Enforce admission control of Origin resources in terminating namespaces
  (decarr@redhat.com)
- update osc config to use the same files as osc get (deads@redhat.com)
- UPSTREAM: make kubectl config behave more expectedly #6585 (deads@redhat.com)
- UPSTREAM: make APIInfoResolve work against subresources (deads@redhat.com)
- fix simulator urls (pweil@redhat.com)
- Provide easy delete of resources for new-app and generate
  (kargakis@users.noreply.github.com)
- Bug 1210659 - create from template fixes (jforrest@redhat.com)
- Cleanup the STI and Docker build output (mfojtik@redhat.com)
- UPSTREAM: Support setting up aliases for groups of resources
  (kargakis@users.noreply.github.com)
- Instructions to reload Network Manager (jliggitt@redhat.com)
- Update router integration test (ccoleman@redhat.com)
- TEMPORARY: osc build-logs failing in other namespace (ccoleman@redhat.com)
- Version lock e2e output tests (ccoleman@redhat.com)
- Master should send convertor, v1beta3 not experimental (ccoleman@redhat.com)
- Fix bug in error output (ccoleman@redhat.com)
- Refactor tests with port, command, testclient changes (ccoleman@redhat.com)
- Use Args instead of Command for OpenShift (ccoleman@redhat.com)
- Event recording has changed upstream, and changes to master/node args
  (ccoleman@redhat.com)
- Handle multi-port services in part (ccoleman@redhat.com)
- Update commands to handle change to cmd.Factory upstream
  (ccoleman@redhat.com)
- Refactor from upstream (ccoleman@redhat.com)
- UPSTREAM: Pass mapping version to printer always (ccoleman@redhat.com)
- UPSTREAM: entrypoint has wrong serialization flags in JSON
  (ccoleman@redhat.com)
- UPSTREAM: Ensure no namespace on create/update root scope types
  (jliggitt@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- UPSTREAM: Disable UI for Kubernetes (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Encode binary assets in ASCII only (jliggitt@redhat.com)
- UPSTREAM: support subresources in api info resolver (deads@redhat.com)
- UPSTREAM: Don't use command pipes for exec/port forward (agoldste@redhat.com)
- UPSTREAM: Prometheus can't be cross-compiled safely (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):b12d75d0eeeadda1282f5738663bf
  e38717ebaf4 (ccoleman@redhat.com)
- [DEVEXP-457] Create application from source. Includes changes from jwforres,
  cewong, sgoodwin (jcantril@redhat.com)
- NetworkPlugin option in node config (rchopra@redhat.com)
- [RPMs]: tuned profiles get installed into /usr/lib not /usr/lib64
  (sdodson@redhat.com)
- Now that we have better error handling, dont attempt login when we get a 0
  status code (jforrest@redhat.com)
- Update Browse->Images to be Image Streams in console (jforrest@redhat.com)
- Stop polling for pods in the console and open a websocket instead
  (jforrest@redhat.com)
- Adding some more notes on probing containers (bleanhar@redhat.com)
- Auto provision image repo on push (agoldste@redhat.com)
- Setup aliases for imageStream* (kargakis@users.noreply.github.com)
- Remove admission/resourcedefaults (ccoleman@redhat.com)
- Update rebase-kube (ccoleman@redhat.com)
- probe delay (pweil@redhat.com)
- Customize messenger styling fix bug Bug 1203949 to correct sidebar display in
  ie and resized logo to fix ie issue (sgoodwin@redhat.com)
- Provide both a host and guest profile (sdodson@redhat.com)
- Display login failures in osc (jliggitt@redhat.com)
- Add placeholder challenger when login is only possible via browser
  (jliggitt@redhat.com)
- Protect browsers from basic-auth CSRF attacks (jliggitt@redhat.com)
- Merge pull request #1697 from jwforres/fix_create_from_template
  (dmcphers+openshiftbot@redhat.com)
- Prevent duplicate resources in new-app (cewong@redhat.com)
- Merge pull request #1676 from derekwaynecarr/block_origin_admission
  (dmcphers+openshiftbot@redhat.com)
- Disable refresh_token generation (jliggitt@redhat.com)
- Pass through remote OAuth errors (jliggitt@redhat.com)
- Create from template fails with JS error (jforrest@redhat.com)
- Merge pull request #1668 from deads2k/deads-upstream-6585
  (dmcphers+openshiftbot@redhat.com)
- Enforce admission control of Origin resources in terminating namespaces
  (decarr@redhat.com)
- Merge pull request #1671 from deads2k/deads-fix-namespace-subresources
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1691 from jwforres/bug_1210659_template_library_fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1681 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- update osc config to use the same files as osc get (deads@redhat.com)
- UPSTREAM: make kubectl config behave more expectedly #6585 (deads@redhat.com)
- Add SyslogIdentifier=openshift-{master,node} respectively
  (sdodson@redhat.com)
- UPSTREAM: make APIInfoResolve work against subresources (deads@redhat.com)
- Merge pull request #1659 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1404 from ncdc/auto-provision-image-repo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1687 from mfojtik/cleanup_builder
  (dmcphers+openshiftbot@redhat.com)
- fix simulator urls (pweil@redhat.com)
- Merge pull request #1537 from kargakis/delete-all-by-label
  (dmcphers+openshiftbot@redhat.com)
- Provide easy delete of resources for new-app and generate
  (kargakis@users.noreply.github.com)
- Merge pull request #1016 from sdodson/BZ1190654
  (dmcphers+openshiftbot@redhat.com)
- Bug 1210659 - create from template fixes (jforrest@redhat.com)
- Merge pull request #1664 from kargakis/aliases
  (dmcphers+openshiftbot@redhat.com)
- Cleanup the STI and Docker build output (mfojtik@redhat.com)
- UPSTREAM: Support setting up aliases for groups of resources
  (kargakis@users.noreply.github.com)
- Instructions to reload Network Manager (jliggitt@redhat.com)
- Update router integration test (ccoleman@redhat.com)
- TEMPORARY: osc build-logs failing in other namespace (ccoleman@redhat.com)
- Version lock e2e output tests (ccoleman@redhat.com)
- Master should send convertor, v1beta3 not experimental (ccoleman@redhat.com)
- Fix bug in error output (ccoleman@redhat.com)
- Refactor tests with port, command, testclient changes (ccoleman@redhat.com)
- Use Args instead of Command for OpenShift (ccoleman@redhat.com)
- Event recording has changed upstream, and changes to master/node args
  (ccoleman@redhat.com)
- Handle multi-port services in part (ccoleman@redhat.com)
- Update commands to handle change to cmd.Factory upstream
  (ccoleman@redhat.com)
- Refactor from upstream (ccoleman@redhat.com)
- UPSTREAM: Pass mapping version to printer always (ccoleman@redhat.com)
- UPSTREAM: entrypoint has wrong serialization flags in JSON
  (ccoleman@redhat.com)
- UPSTREAM: Ensure no namespace on create/update root scope types
  (jliggitt@redhat.com)
- UPSTREAM: add context to ManifestService methods (rpenta@redhat.com)
- UPSTREAM: Handle missing resolv.conf (ccoleman@redhat.com)
- UPSTREAM: Disable UI for Kubernetes (ccoleman@redhat.com)
- UPSTREAM: Disable systemd activation for DNS (ccoleman@redhat.com)
- UPSTREAM: Encode binary assets in ASCII only (jliggitt@redhat.com)
- UPSTREAM: support subresources in api info resolver (deads@redhat.com)
- UPSTREAM: Don't use command pipes for exec/port forward (agoldste@redhat.com)
- UPSTREAM: Prometheus can't be cross-compiled safely (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):b12d75d0eeeadda1282f5738663bf
  e38717ebaf4 (ccoleman@redhat.com)
- [DEVEXP-457] Create application from source. Includes changes from jwforres,
  cewong, sgoodwin (jcantril@redhat.com)
- NetworkPlugin option in node config (rchopra@redhat.com)
- [RPMs]: tuned profiles get installed into /usr/lib not /usr/lib64
  (sdodson@redhat.com)
- Merge pull request #1672 from jwforres/image_streams
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1673 from jwforres/stop_polling_pods
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1658 from pweil-/router-probe-delay
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1675 from jwforres/stop_login_on_zero_error
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1669 from brenton/master
  (dmcphers+openshiftbot@redhat.com)
- Now that we have better error handling, dont attempt login when we get a 0
  status code (jforrest@redhat.com)
- Update Browse->Images to be Image Streams in console (jforrest@redhat.com)
- Stop polling for pods in the console and open a websocket instead
  (jforrest@redhat.com)
- Adding some scripts for handling the sti-basicauthurl build/save
  (bleanhar@redhat.com)
- Adding some more notes on probing containers (bleanhar@redhat.com)
- Merge pull request #1627 from sg00dwin/messenger-and-iefix
  (dmcphers+openshiftbot@redhat.com)
- Auto provision image repo on push (agoldste@redhat.com)
- Setup aliases for imageStream* (kargakis@users.noreply.github.com)
- Automatic commit of package [openshift] release [0.4.3.0].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Remove admission/resourcedefaults (ccoleman@redhat.com)
- Update rebase-kube (ccoleman@redhat.com)
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
- probe delay (pweil@redhat.com)
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
- Automatic commit of package [openshift] release [0.4.2.5].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- OAuth secret config (jliggitt@redhat.com)
- Customize messenger styling fix bug Bug 1203949 to correct sidebar display in
  ie and resized logo to fix ie issue (sgoodwin@redhat.com)
- Fix 'tito release' (sdodson@redhat.com)
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
- Automatic commit of package [openshift] release [0.4.2.4].
  (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.4.2.3].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Updated #1525 leftovers in sample-app (maszulik@redhat.com)
- Bug 1206109: Handle specified tags that don't exist in the specified image
  repository (kargakis@users.noreply.github.com)
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
- Register lower and camelCase for v1beta1 (decarr@redhat.com)
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
- Automatic commit of package [openshift] release [0.4.2.2].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Move hostname detection warning to runtime (jliggitt@redhat.com)
- Make osc.exe work on Windows (jliggitt@redhat.com)
- Merge pull request #1587 from liggitt/image_format_node
  (dmcphers+openshiftbot@redhat.com)
- add delays when handling retries so we don't tight loop (bparees@redhat.com)
- UPSTREAM: add a blocking accept method to RateLimiter
  https://github.com/GoogleCloudPlatform/kubernetes/pull/6314
  (bparees@redhat.com)
- Merge pull request #1554 from deads2k/deads-fix-start-node
  (dmcphers+openshiftbot@redhat.com)
- display pretty strings for policy rule extensions (deads@redhat.com)
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
- UPSTREAM Client must specify a resource version on finalize
  (decarr@redhat.com)
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
- fix typo (pweil@redhat.com)
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
- new-app should treat image repositories as more specific than docker images
  (ccoleman@redhat.com)
- Automatic commit of package [openshift] release [0.4.2.1].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- UPSTREAM: Encode binary assets in ASCII only (jliggitt@redhat.com)
- Restore asset tests on Jenkins (jliggitt@redhat.com)
- Rewrite deployment ImageRepo handling (ironcladlou@gmail.com)
- Send a birthcry event when openshift node starts (pmorie@gmail.com)
- Temporarily remove asset build failures from Jenkins (jliggitt@redhat.com)
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
- Merge pull request #1527 from abhgupta/abhgupta-dev
  (dmcphers+openshiftbot@redhat.com)
- Add helper for p12 cert creation (jliggitt@redhat.com)
- Update vagrant cert wiring (jliggitt@redhat.com)
- Fix build logs with authenticated node (jliggitt@redhat.com)
- Serve node/etcd over https (jliggitt@redhat.com)
- Merge pull request #1523 from pweil-/fix-resetbefore-methods
  (dmcphers+openshiftbot@redhat.com)
- Use the Fake cAdvisor interface on Macs (ccoleman@redhat.com)
- Merge pull request #1343 from fabianofranz/test_e2e_with_login
  (dmcphers+openshiftbot@redhat.com)
- resolve from references when creating builds
  https://bugzilla.redhat.com/show_bug.cgi?id=1206052 (bparees@redhat.com)
- Merge pull request #1496 from kargakis/minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1513 from kargakis/describe-help
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1451 from ironcladlou/post-deployment-hook-proposal
  (dmcphers+openshiftbot@redhat.com)
- fix ResetBefore* methods (pweil@redhat.com)
- Add a forced version tagger for --use-version (sdodson@redhat.com)
- Teach our tito tagger to use vX.Y.Z tags (sdodson@redhat.com)
- Reset rpm specfile version to 0.0.1, add RPM build docs to HACKING.md
  (sdodson@redhat.com)
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
- Automatic commit of package [openshift] release [0.4.2.0].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/pr/1309' (sdodson@redhat.com)
- Merge remote-tracking branch 'upstream/master' (sdodson@redhat.com)
- Merge pull request #1506 from bparees/build_ui
  (dmcphers+openshiftbot@redhat.com)
- Add deprecation warnings for OAUTH envvars (jliggitt@redhat.com)
- expose oauth config in config (deads@redhat.com)
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
- new-app: Avoid extra declaration (kargakis@users.noreply.github.com)
- Moved LabelSelector and LabelFilter to their own bower component
  (jforrest@redhat.com)
- Wrap describe command (kargakis@users.noreply.github.com)
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
- Console now on /console (ccoleman@redhat.com)
- Move common template labels out of the objects description field
  (ccoleman@redhat.com)
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
- Make service dependencies considerably more strict (sdodson@redhat.com)
- Show warning on terminating projects and disable Create button
  (jforrest@redhat.com)
- Test build-chain in hack/test-cmd (kargakis@users.noreply.github.com)
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
- Expand validation and status spec (ironcladlou@gmail.com)
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
- Provide both a host and guest profile (sdodson@redhat.com)
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
- Merge pull request #1253 from kargakis/cli-usage
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1048 from derekwaynecarr/enable_quota_on_admission
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1159 from kargakis/build-logs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #968 from sdodson/alpha-beta
  (dmcphers+openshiftbot@redhat.com)
- Address some cli usage msg inconsistencies
  (kargakis@users.noreply.github.com)
- Remove use of Docker registry code (ccoleman@redhat.com)
- Remove dockerutils (ccoleman@redhat.com)
- UPSTREAM: docker/utils does not need to access autogen (ccoleman@redhat.com)
- Remove fake docker autogen package (ccoleman@redhat.com)
- Merge pull request #1248 from bparees/repo_race
  (dmcphers+openshiftbot@redhat.com)
- create the buildconfig before creating the first imagerepo
  (bparees@redhat.com)
- Only build images during build-cross (ironcladlou@gmail.com)
- Merge pull request #1186 from deads2k/deads-personal-subject-access-review
  (dmcphers+openshiftbot@redhat.com)
- Ensure that we get the latest tags before we build (sdodson@redhat.com)
- Merge pull request #1238 from deads2k/deads-add-redirect-to-viewers
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1197 from kargakis/stream-logs
  (dmcphers+openshiftbot@redhat.com)
- add redirect to list of approved verbs (deads@redhat.com)
- Merge pull request #1231 from deads2k/deads-only-resolve-needed
  (dmcphers+openshiftbot@redhat.com)
- only resolve roles for bindings that matter (deads@redhat.com)
- Merge pull request #1233 from pweil-/vagrant-master-addr
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1218 from mfojtik/fix_print_parameters
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1184 from kargakis/parsing
  (dmcphers+openshiftbot@redhat.com)
- Implement logs streaming option when starting a build
  (kargakis@users.noreply.github.com)
- Merge pull request #1225 from liggitt/build_compare_tag_and_namespace
  (dmcphers+openshiftbot@redhat.com)
- Turn on quota related admission control plug-ins (decarr@redhat.com)
- use master ip address so that minions can reach master in multi-node setup.
  (pweil@redhat.com)
- allow current-user subjectaccessreview (deads@redhat.com)
- Use Docker parser when manipulating Dockerfiles
  (kargakis@users.noreply.github.com)
- Fix broken --parameters switch for process (mfojtik@redhat.com)
- Add test to exercise invalid parameter in Template (mfojtik@redhat.com)
- Return error when Template generator failed to generate parameter value
  (mfojtik@redhat.com)
- Merge pull request #1227 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1194 from bparees/build_errors
  (dmcphers+openshiftbot@redhat.com)
- Fix escaping and value (dmcphers@redhat.com)
- Merge pull request #1205 from jmccormick2001/master
  (dmcphers+openshiftbot@redhat.com)
- Make sure image, namespace, and tag match (jliggitt@redhat.com)
- Merge pull request #1213 from pweil-/router-namespace-serviceunits
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1150 from csrwng/bug_1191047
  (dmcphers+openshiftbot@redhat.com)
- retry build errors (bparees@redhat.com)
- Merge pull request #1222 from csrwng/authorize_templates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1204 from sdodson/systemd-notify
  (dmcphers+openshiftbot@redhat.com)
- Fix new-app generation from local source (cewong@redhat.com)
- add namespace to internal route keys (pweil@redhat.com)
- Add templates to list of authorized resources (cewong@redhat.com)
- Merge pull request #1220 from bparees/revert_repo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1183 from kargakis/godeps
  (dmcphers+openshiftbot@redhat.com)
- add export KUBECONFIG to osc create example
  (jeff.mccormick@crunchydatasolutions.com)
- Merge pull request #1055 from nak3/hardcoded-library-path
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1178 from mfojtik/fix_bc_labels
  (dmcphers+openshiftbot@redhat.com)
- switch access review users and groups to stringsets (deads@redhat.com)
- add subject access review integration tests (deads@redhat.com)
- Only push latest and the current tag (sdodson@redhat.com)
- Add BuildLogs method on Client (kargakis@users.noreply.github.com)
- Merge pull request #1136 from smarterclayton/rebase_kube
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1216 from deads2k/deads-stop-updates-to-refs
  (dmcphers+openshiftbot@redhat.com)
- Switch services to notify (sdodson@redhat.com)
- Add systemd notification on service startup completion (sdodson@redhat.com)
- bump(github.com/coreos/go-systemd): 2d21675230a81a503f4363f4aa3490af06d52bb8
  (sdodson@redhat.com)
- Revert "add annotations to image repos" (bparees@redhat.com)
- Bump sti-image-builder to STI v0.2 (sdodson@redhat.com)
- Bump openshift-0.4-0 (sdodson@redhat.com)
- prevent changes to rolebinding.RoleRef (deads@redhat.com)
- Merge tag 'v0.4' (sdodson@redhat.com)
- Merge pull request #1215 from csrwng/bug_1191960
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1210 from kargakis/minor-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1212 from liggitt/jenkins
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1196 from soltysh/hacking_cleaning
  (dmcphers+openshiftbot@redhat.com)
- Removed hack/config-go.sh from HACKING.md, it's not used anymore.
  (maszulik@redhat.com)
- Bug 1191960 - Remove --master from usage text for ex generate
  (cewong@redhat.com)
- Merge pull request #1182 from deads2k/deads-fix-node-list
  (dmcphers+openshiftbot@redhat.com)
- Update Jenkins example with auth identity (jliggitt@redhat.com)
- Merge pull request #1074 from deads2k/deads-the-big-one
  (dmcphers+openshiftbot@redhat.com)
- fix nodelist defaulting (deads@redhat.com)
- Automatic commit of package [openshift] release [0.3.4-0].
  (sdodson@redhat.com)
- Merge tag 'v0.3.4' (sdodson@redhat.com)
- Remove extra error check (kargakis@users.noreply.github.com)
- Merge pull request #1206 from bparees/image_tags
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1133 from pweil-/routes-console
  (dmcphers+openshiftbot@redhat.com)
- add annotations to image repos (bparees@redhat.com)
- Merge pull request #1203 from bparees/int_tests
  (dmcphers+openshiftbot@redhat.com)
- fix osc create example to include cert dir kubconfig parameter
  (jeff.mccormick@crunchydatasolutions.com)
- add routes to services (pweil@redhat.com)
- exit with error if no tests are found (bparees@redhat.com)
- Merge pull request #1202 from csrwng/sample_app_labels
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1109 from tdawson/2015-02/haproxy-rpm
  (dmcphers+openshiftbot@redhat.com)
- Add default template label to sample app templates (cewong@redhat.com)
- Fix router integration test (jliggitt@redhat.com)
- Add version command for all binaries/symlinks
  (kargakis@users.noreply.github.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1195 from smarterclayton/better_integration_output
  (dmcphers+openshiftbot@redhat.com)
- Updating ruby-20 links and image name so they point to new repo and image
  (j.hadvig@gmail.com)
- Better output for integration tests (ccoleman@redhat.com)
- Add RootResourceAccessReview to replace use of empty namespace
  (ccoleman@redhat.com)
- Better output for integration tests (ccoleman@redhat.com)
- UPSTREAM: Validate TCPSocket handler correctly (ccoleman@redhat.com)
- UPSTREAM: Relax constraints on container status while fetching container logs
  (vishnuk@google.com)
- Remove references to old volume source, handle endpoints change
  (ccoleman@redhat.com)
- UPSTREAM: Make setSelfLink not bail out (ccoleman@redhat.com)
- UPSTREAM: special command "help" must be aware of context
  (contact@fabianofranz.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- UPSTREAM: Add 'release' field to raven-go (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):6241a211c8f35a6147aa3a0236f68
  0ffa8e11037 (ccoleman@redhat.com)
- bump(github.com/docker/docker):7d2188f9955d3f2002ff8c2e566ef84121de5217
  (kargakis@users.noreply.github.com)
- Wrap some commands to display OpenShift-specific usage msg
  (kargakis@users.noreply.github.com)
- Merge pull request #1192 from bparees/integration
  (dmcphers+openshiftbot@redhat.com)
- support multiple go versions (bparees@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- fix up the jenkins example (bparees@redhat.com)
- Merge pull request #1190 from deads2k/deads-fix-stuck-merge-queue
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1188 from deads2k/deads-fix-image-format-flag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1170 from ironcladlou/deploy-retry-refactor
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1176 from liggitt/test_ui_bind_address
  (dmcphers+openshiftbot@redhat.com)
- extend the wait time for the project authorization cache (deads@redhat.com)
- Refactor deploy controllers to use RetryController (ironcladlou@gmail.com)
- Specify UI bind address in integration tests (jliggitt@redhat.com)
- reconnect --image (deads@redhat.com)
- Add htpasswd file param (jliggitt@redhat.com)
- Prevent challenging client from looping (jliggitt@redhat.com)
- Simplify auto-grant (jliggitt@redhat.com)
- Merge pull request #1174 from liggitt/project_name_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge tag 'v0.3.3' (sdodson@redhat.com)
- use rpm instead of build haproxy from source (tdawson@redhat.com)
- fix nodelist access for kube master (deads@redhat.com)
- enforce authorization (deads@redhat.com)
- Merge pull request #1173 from liggitt/new_project_add_user
  (dmcphers+openshiftbot@redhat.com)
- Put BuildConfig labels into metadata for sample-app (mfojtik@redhat.com)
- Revert "Support multiple Dockerfiles with custom-docker-builder"
  (bparees@redhat.com)
- Merge pull request #1172 from liggitt/test_osc_list_projects
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1163 from ncdc/allow-multiple-tags-with-same-image-id
  (dmcphers+openshiftbot@redhat.com)
- Match project name validation to namespace name validation
  (jliggitt@redhat.com)
- Make "new-project --admin" reuse "add-user" (jliggitt@redhat.com)
- Automatic commit of package [openshift] release [0.3.3-0].
  (sdodson@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Add test to make sure osc can list new projects (jliggitt@redhat.com)
- Merge pull request #1169 from sdodson/fix-openshift-node-service
  (bleanhar@redhat.com)
- Merge pull request #1167 from jwforres/fix_usage_comparison
  (dmcphers+openshiftbot@redhat.com)
- Explicitly set --kubeconfig when starting node (sdodson@redhat.com)
- let project admins use resource access review (deads@redhat.com)
- Fix Used vs Max quota comparison in web console (jforrest@redhat.com)
- add context namespacing filter (deads@redhat.com)
- Merge pull request #1165 from liggitt/project_auth_cache
  (dmcphers+openshiftbot@redhat.com)
- Re-enable project auth cache, add UI integration test (jliggitt@redhat.com)
- Merge pull request #1091 from ironcladlou/deploy-retries
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1153 from jwforres/pod_details
  (dmcphers+openshiftbot@redhat.com)
- Output more details on pod details page in web console (jforrest@redhat.com)
- Allow multiple tags to refer to the same image (agoldste@redhat.com)
- Add controller retry support scaffolding (ironcladlou@gmail.com)
- Merge pull request #1156 from liggitt/start_config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1116 from pweil-/router-live-probe
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Support AddIfNotPresent function (ironcladlou@gmail.com)
- Fix TLS EOF errors in log at start (jliggitt@redhat.com)
- Ensure create of master policy namespace happens after policy will allow it
  (jliggitt@redhat.com)
- Rework --kubeconfig handler, misc tweaks (jliggitt@redhat.com)
- make start.config immutable (deads@redhat.com)
- Merge pull request #1160 from soltysh/contextdir_conversion
  (dmcphers+openshiftbot@redhat.com)
- Fixed contextDir conversion after moving it from DockerBuildStrategy to
  BuildSource (maszulik@redhat.com)
- Make builder image naming consistent between build strategy describe
  (maszulik@redhat.com)
- Merge pull request #1122 from kargakis/markdown
  (dmcphers+openshiftbot@redhat.com)
- Add verify-jsonformat to Travis (mfojtik@redhat.com)
- Fix formatting and errors in JSON files (mfojtik@redhat.com)
- Added ./hack/verify-jsonformat.sh (mfojtik@redhat.com)
- Merge pull request #1144 from soltysh/image_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1152 from colemickens/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Support multiple Dockerfiles with custom-docker-builder
  (cole.mickens@gmail.com)
- Merge pull request #1148 from bparees/remove_local
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1134 from jwforres/build_config_details
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1149 from jwforres/autoupdate_timestamp
  (dmcphers+openshiftbot@redhat.com)
- Set global timer to autoupdate relative timestamps in the UI
  (jforrest@redhat.com)
- Merge pull request #1119 from bparees/optional_output
  (dmcphers+openshiftbot@redhat.com)
- remove use_local env (bparees@redhat.com)
- Builds page - group builds by build config and show more details for build
  configs (jforrest@redhat.com)
- Merge pull request #1146 from bparees/generator_ruby_ref
  (dmcphers+openshiftbot@redhat.com)
- make build output optional (bparees@redhat.com)
- use ruby-20-centos7 in generated buildconfig (bparees@redhat.com)
- Use --from-build flag only for re-running builds
  (kargakis@users.noreply.github.com)
- Changed image names for ImageRepository objects, since it was using two
  exactly the same in different tests (maszulik@redhat.com)
- add liveness probe to router (pweil@redhat.com)
- Fix docs (kargakis@users.noreply.github.com)
- Merge pull request #1138 from mfojtik/sti_rebase2
  (dmcphers+openshiftbot@redhat.com)
- Rename 'Clean' to 'Incremental' in STI builder (mfojtik@redhat.com)
- Changed Info->Infof to have the variable printed (maszulik@redhat.com)
- Merge pull request #1097 from jwforres/integration_tests
  (dmcphers+openshiftbot@redhat.com)
- Add optional e2e UI tests (jforrest@redhat.com)
- bump(github.com/openshift/source-to-
  image):c0c154efcba27ea5693c428bfe28560c220b4850 (mfojtik@redhat.com)
- Make cli examples consistent across OpenShift
  (kargakis@users.noreply.github.com)
- Merge pull request #1127 from kargakis/api-consistency
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1124 from akram/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1086 from csrwng/bug_1190577
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1093 from csrwng/bug_1190575
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Merge pull request #1110 from liggitt/username_validation
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1132 from ironcladlou/rollback-cli-printer-fix
  (dmcphers+openshiftbot@redhat.com)
- Validate usernames don't contain problematic URL sequences
  (jliggitt@redhat.com)
- Merge pull request #1131 from ironcladlou/deploy-generatename-fix
  (dmcphers+openshiftbot@redhat.com)
- Use a versioned printer for rollback configs (ironcladlou@gmail.com)
- Fix broken deployer pod GenerateName reference (ironcladlou@gmail.com)
- update must gather for policy (deads@redhat.com)
- Generate: handle image with multiple ports in EXPOSE statement
  (cewong@redhat.com)
- Update command parameter and help text to refer to single port
  (cewong@redhat.com)
- Merge pull request #1104 from deads2k/deads-update-readme
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1113 from deads2k/deads-pretty-up-describe
  (dmcphers+openshiftbot@redhat.com)
- update readme to take advantage of authorization (deads@redhat.com)
- Merge pull request #1095 from csrwng/bug_1194487
  (dmcphers+openshiftbot@redhat.com)
- Cleanup dangling images from cache (sdodson@redhat.com)
- Make Clients API consistent (kargakis@users.noreply.github.com)
- Add htpasswd SHA/MD5 support (jliggitt@redhat.com)
- Merge pull request #1077 from liggitt/kubeconfig_public_master_context
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1107 from soltysh/rename_base_image
  (dmcphers+openshiftbot@redhat.com)
- route definition requires name in metadata (akram.benaissi@free.fr)
- Rename BaseImage -> Image for DockerBuildStrategy to be consistent with
  STIBuilderStrategy about field naming (maszulik@redhat.com)
- Generate master and public-master .kubeconfig contexts (jliggitt@redhat.com)
- Add options to clear authorization headers for basic/bearer auth
  (jliggitt@redhat.com)
- Merge pull request #1114 from bparees/build_names
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1108 from deads2k/deads-create-policy-cache
  (dmcphers+openshiftbot@redhat.com)
- better build names (bparees@redhat.com)
- Merge pull request #1100 from smarterclayton/add_registry_create_command
  (dmcphers+openshiftbot@redhat.com)
- pretty up policy describer (deads@redhat.com)
- UPSTREAM: get the keys from a string map (deads@redhat.com)
- create policy cache (deads@redhat.com)
- Merge pull request #967 from goldmann/service_fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1112 from liggitt/deny_password
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #1111 from danmcp/master
  (dmcphers+openshiftbot@redhat.com)
- Add ose-docker-registry to ose-build script (sdodson@redhat.com)
- Add 'deny' password authenticator (jliggitt@redhat.com)
- Fix case (dmcphers@redhat.com)
- Add a command to install / check a registry (ccoleman@redhat.com)
- Merge pull request #1101 from soltysh/card426
  (dmcphers+openshiftbot@redhat.com)
- Merge remote-tracking branch 'origin-upstream/master' (sdodson@redhat.com)
- Automatic commit of package [openshift] release [0.3.2-0].
  (sdodson@redhat.com)
- Merge tag 'v0.3.2' (sdodson@redhat.com)
- refactor authorization for sanity (deads@redhat.com)
- Card devexp_426 - Force clean builds by default for STI (maszulik@redhat.com)
- Better management of the systemd services (marek.goldmann@gmail.com)
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
- Bug 1194487 - Fix generate command repository detection (cewong@redhat.com)
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
- Replace hardcoded-library-path (nakayamakenjiro@gmail.com)
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
- Change references from alpha to beta release (sdodson@redhat.com)
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
- Automatic commit of package [openshift] release [0.2.2-0].
  (sdodson@redhat.com)
- Merge pull request #860 from bparees/check_dockerfile
  (dmcphers+openshiftbot@redhat.com)
- Merge tag 'v0.2.2' (sdodson@redhat.com)
- use start-build instead of curl (bparees@redhat.com)
- Merge pull request #870 from mfojtik/context_dir
  (dmcphers+openshiftbot@redhat.com)
- Add CONTEXT_DIR support for sti-image-builder image (mfojtik@redhat.com)
- Merge pull request #845 from mnagy/add_upstream_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #866 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Generate openshift service on provision (dmcphers@redhat.com)
- Merge pull request #861 from smarterclayton/version_images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #851 from bparees/remove_guestbook
  (dmcphers+openshiftbot@redhat.com)
- Expose two new flags on master --images and --latest-images
  (ccoleman@redhat.com)
- Create an openshift/origin-pod image (ccoleman@redhat.com)
- check for error on missing docker file (bparees@redhat.com)
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
- Adding tests for help consistency to test-cmd.sh (contact@fabianofranz.com)
- UPSTREAM: special command "help" must be aware of context
  (contact@fabianofranz.com)
- Merge pull request #842 from deads2k/deads-registry-errors
  (dmcphers+openshiftbot@redhat.com)
- better description of output to field (bparees@redhat.com)
- experimental policy cli (deads@redhat.com)
- UPSTREAM: need to make sure --help flags is registered before calling pflag
  (contact@fabianofranz.com)
- move kubernetes capabilities to server start (patrick.hemmer@gmail.com)
- Add --check option to run golint and gofmt in ./hack/build-in-docker.sh
  (mfojtik@redhat.com)
- Add retry logic to recreate deployment strategy (ironcladlou@gmail.com)
- Very simple authorizing proxy for Kubernetes (ccoleman@redhat.com)
- Unify authorization logic into a more structured form (ccoleman@redhat.com)
- User registry should transform server errors (ccoleman@redhat.com)
- UPSTREAM: Handle case insensitive node names and squash logging
  (ccoleman@redhat.com)
- UPSTREAM: Use new resource builder in kubectl update #3805
  (nagy.martin@gmail.com)
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
- add missing reencrypt validations (pweil@redhat.com)
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
- Automatic commit of package [openshift] release [0.2.1-4].
  (sdodson@redhat.com)
- Allow bower to run as root (dmcphers@redhat.com)
- Vagrantfile: improve providers and usability/readability (lmeyer@redhat.com)
- UPSTREAM: resolve relative paths in .kubeconfig (deads@redhat.com)
- Pin haproxy to 1.5.10 (sdodson@redhat.com)
- Merge pull request #805 from smarterclayton/symlink_osc_in_image
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #793 from deads2k/deads-make-e2e-project
  (dmcphers+openshiftbot@redhat.com)
- 'osc' should be symlinked in the openshift/origin Docker image
  (ccoleman@redhat.com)
- Split master and node packaging (sdodson@redhat.com)
- create test project for e2e (deads@redhat.com)
- Fix the Vagrant network setup (lhuard@amadeus.com)
- Merge pull request #785 from mfojtik/sti_env
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #796 from deads2k/deads-stop-the-madness
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #610 from deads2k/deads-openshift-authorization
  (dmcphers+openshiftbot@redhat.com)
- stop "OpenShift will terminate as soon as a panic occurs" from spamming
  during e2e (deads@redhat.com)
- Merge pull request #757 from liggitt/login_page
  (dmcphers+openshiftbot@redhat.com)
- Add asset server hostname to cert, generate cert without ports
  (jliggitt@redhat.com)
- Integration tests should let master load its own API (ccoleman@redhat.com)
- Build openshift-web-console oauth client (jliggitt@redhat.com)
- Externalize and doc auth config (jliggitt@redhat.com)
- Auto grant, make oauth sessions short, handle session decode failures
  (jliggitt@redhat.com)
- Prettify token display page (jliggitt@redhat.com)
- Add login template (jliggitt@redhat.com)
- Merge pull request #782 from fabianofranz/osc_config
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #790 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Rename fedora image (dmcphers@redhat.com)
- Merge pull request #760 from bparees/immutable_builds
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #698 from mfojtik/card_471
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #787 from jwforres/no_cache_config
  (dmcphers+openshiftbot@redhat.com)
- refactor build interfaces (bparees@redhat.com)
- Allow to customize env variables for STI Build strategy (mfojtik@redhat.com)
- Don't cache the generated config.js or it won't pick up changes in startup
  options (jforrest@redhat.com)
- Makes 'config' set of commands experimental (contact@fabianofranz.com)
- Fix guestbook example to not collide with 'frontend' service
  (mfojtik@redhat.com)
- Convert Config to kapi.List{} and fix Template to use runtime.Object{}
  (mfojtik@redhat.com)
- UPSTREAM(ae3f10): Ensure the ptr is pointing to reflect.Slice in ExtractList
  (mfojtik@redhat.com)
- UPSTREAM(e7df8a): Fix ExtractList to support extraction from generic
  api.List{} (mfojtik@redhat.com)
- osc binary not present for Mac or Windows, windows needs .exe
  (ccoleman@redhat.com)
- Exposes 'osc config' to manage .kubeconfig files (contact@fabianofranz.com)
- Merge pull request #781 from liggitt/skip_cert_gen
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #779 from smarterclayton/save_osc_command
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #776 from smarterclayton/tag_images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #652 from pweil-/router-ssl
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #768 from pweil-/router-install-remote
  (dmcphers+openshiftbot@redhat.com)
- Skip client cert gen if it exists (jliggitt@redhat.com)
- Merge pull request #778 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Skip server cert gen if exists and is valid (jliggitt@redhat.com)
- Merge pull request #777 from liggitt/public_master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #748 from derekwaynecarr/project_should_have_no_ns
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #755 from smarterclayton/config_generator_errors
  (dmcphers+openshiftbot@redhat.com)
- Copy openshift binary in the tar (ccoleman@redhat.com)
- Merge pull request #758 from sg00dwin/less-partials-split
  (dmcphers+openshiftbot@redhat.com)
- Switch default instance size (dmcphers@redhat.com)
- Set public kubernetes master when starting kube (jliggitt@redhat.com)
- Allow images to be tagged in hack/push-release.sh (ccoleman@redhat.com)
- Merge pull request #775 from liggitt/fail_invalid_tokens
  (dmcphers+openshiftbot@redhat.com)
- add server arg to osc command to support remote masters (pweil@redhat.com)
- Add option to unionauth to fail on error, fail on invalid or expired bearer
  tokens (jliggitt@redhat.com)
- Merge pull request #764 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Update bindata.go (decarr@redhat.com)
- Update ui code to not use project.metadata.namespace (decarr@redhat.com)
- Align project with upstream resources that exist outside of namespace
  (decarr@redhat.com)
- fix public-ip for node-sdn (rchopra@redhat.com)
- Fix bindata.go for the less partials split (jforrest@redhat.com)
- Change box url and name back (dmcphers@redhat.com)
- router TLS (pweil@redhat.com)
- Merge pull request #759 from csrwng/registry_client
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #762 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Make insert key configurable (dmcphers@redhat.com)
- Merge pull request #761 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Make the provider configs optional (dmcphers@redhat.com)
- Merge pull request #754 from pmorie/vagrant
  (dmcphers+openshiftbot@redhat.com)
- Splitting .less into partials and refactor of code. Including openshift-icon
  font set for now. (sgoodwin@redhat.com)
- bump(github.com/smarterclayton/go-
  dockerregistryclient):3b6185cb3ac3811057e317dcff91f36eef17b8b0
  (ccoleman@redhat.com)
- Merge pull request #756 from deads2k/deads-really-kill-openshift
  (dmcphers+openshiftbot@redhat.com)
- kill openshift process during e2e (deads@redhat.com)
- resolve osapi endpoint to be configured as upstream to resolve 406
  (jcantril@redhat.com)
- Merge pull request #753 from smarterclayton/add_sentry
  (dmcphers+openshiftbot@redhat.com)
- Config Generator should not return raw errors via the REST API
  (ccoleman@redhat.com)
- Fix vagrant single vm environment (pmorie@gmail.com)
- Enable authentication, add x509 cert auth, anonymous auth
  (jliggitt@redhat.com)
- Separate OAuth and API muxes, pass authenticator into master
  (jliggitt@redhat.com)
- Add x509 authenticator (jliggitt@redhat.com)
- Add groups to userinfo (jliggitt@redhat.com)
- Allow panics to be reported to Sentry (ccoleman@redhat.com)
- UPSTREAM: Allow panics and unhandled errors to be reported to external
  targets (ccoleman@redhat.com)
- UPSTREAM: Add 'release' field to raven-go (ccoleman@redhat.com)
- bump(github.com/getsentry/raven-go):3fd636ed242c26c0f55bc9ee1fe47e1d6d2d77f7
  (ccoleman@redhat.com)
- Merge pull request #750 from kargakis/router
  (dmcphers+openshiftbot@redhat.com)
- Add missing error assignments (michaliskargakis@gmail.com)
- Merge pull request #736 from smarterclayton/add_profiler
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #745 from smarterclayton/add_install_registry
  (dmcphers+openshiftbot@redhat.com)
- Add profiling tools to the OpenShift binary (ccoleman@redhat.com)
- Add instructions to ignore vboxnet interfaces on the host.
  (mrunalp@gmail.com)
- Move registry install function to a reusable spot (ccoleman@redhat.com)
- Remove unused api target from Makefile comments (vvitek@redhat.com)
- add dns recommendations (bparees@redhat.com)
- Fix bug #686 (pmorie@gmail.com)
- Merge pull request #734 from sosiouxme/201401-asset-server-ips
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #722 from liggitt/routing
  (dmcphers+openshiftbot@redhat.com)
- Lock webcomponents version (jliggitt@redhat.com)
- bump(github.com/pkg/profile):c795610ec6e479e5795f7852db65ea15073674a6
  (ccoleman@redhat.com)
- Merge pull request #707 from rajatchopra/sdn
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #726 from bparees/jenkins_artifacts
  (dmcphers+openshiftbot@redhat.com)
- Use correct client config to run token cmd (jliggitt@redhat.com)
- Update routing readme and script to set up TLS (jliggitt@redhat.com)
- Separate asset bind and asset public addr (jliggitt@redhat.com)
- provide an environment switch to choose ovs-simple as the overlay network for
  the cluster (rchopra@redhat.com)
- start: provide + use flags for public API addresses (lmeyer@redhat.com)
- Merge pull request #679 from smarterclayton/reference_build_output
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #732 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- No need to redoc k8s in origin (dmcphers@redhat.com)
- Review comments 2 - Pass Codec through and return error on setBuildEnv
  (ccoleman@redhat.com)
- Disable race detection on test-integration because of #731
  (ccoleman@redhat.com)
- Crash on Panic when ENV var set (ccoleman@redhat.com)
- asset server: change bind addr (lmeyer@redhat.com)
- delete large log files before archiving to jenkins (bparees@redhat.com)
- Merge pull request #728 from TomasTomecek/packaging-store-runtime-data-better
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #727 from TomasTomecek/packaging-fix-etc-sysconfig
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #702 from jhadvig/buildlog-phase-fix
  (dmcphers+openshiftbot@redhat.com)
- Bug: Build logs could be accessed only when the pod phase was running
  (j.hadvig@gmail.com)
- packaging: put runtime data to /var/lib/origin (ttomecek@redhat.com)
- packaging: remove invalid options in /etc/sysconf/os (ttomecek@redhat.com)
- Merge pull request #706 from fabianofranz/rebase_kube_docs
  (dmcphers+openshiftbot@redhat.com)
- WIP - Template updates (ccoleman@redhat.com)
- More flexibility to test-cmd and test-end-to-end (ccoleman@redhat.com)
- Remove the need to load deployments from config_generator
  (ccoleman@redhat.com)
- Merge pull request #719 from ironcladlou/apiserver-caps-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #720 from sg00dwin/label-filter-dev
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #708 from smarterclayton/send_events
  (dmcphers+openshiftbot@redhat.com)
- Allow more goroutines to exit during test cases with RunUntil
  (ccoleman@redhat.com)
- Return the most recently updated deployment config after PUT
  (ccoleman@redhat.com)
- Cleanup integration tests (ccoleman@redhat.com)
- Add "from" object reference to both ImageChangeTriggers (ccoleman@redhat.com)
- Support service variable substitution in docker registry variable
  (ccoleman@redhat.com)
- Use Status.DockerImageRepository from CLI (ccoleman@redhat.com)
- Check for an ImageRepositoryMapping with metadata/name before DIR
  (ccoleman@redhat.com)
- Review comments (ccoleman@redhat.com)
- When creating build, lookup "to" field if specified (ccoleman@redhat.com)
- Define "to" and "dockerImageReference" as new fields on BuildOutput
  (ccoleman@redhat.com)
- UPSTREAM: Add RunUntil(stopCh) to reflector and poller to allow termination
  (ccoleman@redhat.com)
- UPSTREAM: Expose validation.ValidateLabels for reuse (ccoleman@redhat.com)
- Disable other e2e tests (ccoleman@redhat.com)
- Merge pull request #648 from ironcladlou/rollback-api
  (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Use ExtractObj instead of ExtractList in Kubelet
  (ccoleman@redhat.com)
- Introduce deployment rollback API (ironcladlou@gmail.com)
- design policy (deads@redhat.com)
- Enable privileged capabilities in apiserver (ironcladlou@gmail.com)
- Merge pull request #696 from bparees/build_tests
  (dmcphers+openshiftbot@redhat.com)
- Wireframes of label filter interface states event-based user actions
  (sgoodwin@redhat.com)
- run custom, docker builds in e2e (bparees@redhat.com)
- Merge pull request #682 from goldmann/openshift-node-restart
  (dmcphers+openshiftbot@redhat.com)
- Updating HACKING doc with more detailed information about Kubernetes rebases
  (contact@fabianofranz.com)
- Merge pull request #717 from soltysh/resourceversion_removal
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #715 from liggitt/listen_scheme
  (dmcphers+openshiftbot@redhat.com)
- Remove hard-coded resourceVersion values from build watches
  (maszulik@redhat.com)
- Merge pull request #713 from liggitt/tls_token_request
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #701 from pmorie/deadcode
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #695 from pmorie/typo (dmcphers+openshiftbot@redhat.com)
- Merge pull request #705 from ironcladlou/remove-hardcoded-watch-versions
  (dmcphers+openshiftbot@redhat.com)
- Make vagrant run in http for now (jliggitt@redhat.com)
- Default --master scheme to match --listen (jliggitt@redhat.com)
- Rebuild to pick up new webcomponents (jliggitt@redhat.com)
- Fix internal token request with TLS (jliggitt@redhat.com)
- Remove flag hard-coding (pmorie@gmail.com)
- Send events from the master (ccoleman@redhat.com)
- Don't hard-code resourceVersion in watches (ironcladlou@gmail.com)
- Merge pull request #638 from liggitt/tls (dmcphers+openshiftbot@redhat.com)
- Enable TLS (jliggitt@redhat.com)
- Merge pull request #691 from pmorie/docs (dmcphers+openshiftbot@redhat.com)
- UPSTREAM: Allow changing global default server hostname (jliggitt@redhat.com)
- Exclude .git and node_modules from test search (ironcladlou@gmail.com)
- UPSTREAM: Use CAFile even when cert/key is not specified, allow client config
  to take cert data directly (jliggitt@redhat.com)
- Merge pull request #694 from mfojtik/remove_yaml
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #689 from csrwng/fix_docker_build
  (dmcphers+openshiftbot@redhat.com)
- Typo in deployment controller factory (pmorie@gmail.com)
- Remove yaml from all object types (mfojtik@redhat.com)
- Router doc json cleanup (pmorie@gmail.com)
- Make OSC use non-interactive auth loading, fix flag binding
  (jliggitt@redhat.com)
- UPSTREAM: make kubectl factory flag binding optional (jliggitt@redhat.com)
- Fix docker build with context directory (cewong@redhat.com)
- Merge pull request #687 from sosiouxme/20150120-model-docs
  (dmcphers+openshiftbot@redhat.com)
- docs: Filling in some initial model descriptions (lmeyer@redhat.com)
- Merge pull request #677 from pmorie/build (dmcphers+openshiftbot@redhat.com)
- Merge pull request #680 from bparees/fix_sample
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #672 from sdodson/tito-rel-eng
  (dmcphers+openshiftbot@redhat.com)
- docs: *_model.adoc => .md (lmeyer@redhat.com)
- Make hack/build-go.sh create symlinks for openshift (pmorie@gmail.com)
- UPSTREAM: bump(github.com/jteeuwen/go-bindata):
  f94581bd91620d0ccd9c22bb1d2de13f6a605857 (jliggitt@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- Refactor to match changes upstream (mfojtik@redhat.com)
- Refactor to match changes upstream (contact@fabianofranz.com)
- UPSTREAM: api registration right on mux makes it invisible to container
  (contact@fabianofranz.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):21b661ecf3038dc50f75d345276c9
  cf460af9df2 (contact@fabianofranz.com)
- Merge pull request #659 from jwforres/console_main_navigation
  (dmcphers+openshiftbot@redhat.com)
- In case of failure, wait and restart the openshift-node service
  (marek.goldmann@gmail.com)
- Pull in hawtio-core-navigation and build console nav around it. Styles and
  theming by @sg00dwin (jforrest@redhat.com)
- fix namespacing so sample works again (bparees@redhat.com)
- Fix gofmt detection (jliggitt@redhat.com)
- Merge pull request #676 from bparees/quote_privileged
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #658 from jwforres/console_hits_master_hostname
  (dmcphers+openshiftbot@redhat.com)
- add missing quotes to privileged (bparees@redhat.com)
- Merge pull request #635 from smarterclayton/image_stream_status
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #674 from liggitt/cancel_build
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #673 from smarterclayton/fix_constants
  (dmcphers+openshiftbot@redhat.com)
- Require docker-io 1.3.2 or later. Remove -devel FIXME (sdodson@redhat.com)
- Support `GET /imageRepositoryTags/<name>:<tag>` (ccoleman@redhat.com)
- Properly version Docker image metadata sent to the API (ccoleman@redhat.com)
- UPSTREAM: Expose TypeAccessor for objects without metadata
  (ccoleman@redhat.com)
- Update end-to-end to use new registry pattern (ccoleman@redhat.com)
- Depend on docker-io (sdodson@redhat.com)
- Tolerate missing build logs in cancel-build (jliggitt@redhat.com)
- Constants are mismatched on BuildTriggerType (ccoleman@redhat.com)
- Move systemd unit and sysconfig files to rel-eng (sdodson@redhat.com)
- Merge pull request #665 from smarterclayton/add_delete_methods
  (dmcphers+openshiftbot@redhat.com)
- ImageRepository lookup should check the OpenShift for <namespace>/<name>
  (ccoleman@redhat.com)
- UPSTREAM: Disable auto-pull when tag is "latest" (ccoleman@redhat.com)
- Initial pass at getting tito set up for release engineering purposes
  (sdodson@redhat.com)
- Merge pull request #664 from smarterclayton/test_fixes
  (dmcphers+openshiftbot@redhat.com)
- Update sample app README now that cors-allowed-origins option isn't required
  for embedded web console (jforrest@redhat.com)
- Move specification for api version for web console into console code.  Add
  127.0.0.1 to default cors list. (jforrest@redhat.com)
- Web console talks to master over configured master host/port and embedded web
  console allowed by default in CORS list (jforrest@redhat.com)
- Bug 1176815 - Improve error reporting for build-log (mfojtik@redhat.com)
- Update README with simpler commands (ccoleman@redhat.com)
- Merge pull request #667 from smarterclayton/whitespace
  (dmcphers+openshiftbot@redhat.com)
- Minor whitespace cleanup (ccoleman@redhat.com)
- Ensure tests shut themselves down, and fix race in imagechange test
  (ccoleman@redhat.com)
- Add Delete methods to various Image objects (ccoleman@redhat.com)
- Update the Makefile to run all tests (ccoleman@redhat.com)
- One last path fix (matthicksj@gmail.com)
- Fixing openshift path (matthicksj@gmail.com)
- Updating to use Makefile and updating path (matthicksj@gmail.com)
- Merge pull request #661 from bparees/privilged_builds
  (dmcphers+openshiftbot@redhat.com)
- use privileged containers for builds and docker registry (bparees@redhat.com)
- Merge pull request #660 from sosiouxme/model-docs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #649 from akostadinov/master
  (dmcphers+openshiftbot@redhat.com)
- docs: place for k8s/os model descriptions (lmeyer@redhat.com)
- Merge pull request #657 from bparees/copy_build
  (dmcphers+openshiftbot@redhat.com)
- deep copy when creating build from buildconfig (bparees@redhat.com)
- Merge pull request #654 from mfojtik/sti_update_flow
  (dmcphers+openshiftbot@redhat.com)
- Push the new STI images into configured output imageRepository
  (mfojtik@redhat.com)
- Merge pull request #646 from nhr/added_sample_app_prerun_info
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #647 from csrwng/untag_built_images
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #653 from luksa/vagrant_master_ip
  (dmcphers+openshiftbot@redhat.com)
- Add the hawtio-extension-service as a dependency (jforrest@redhat.com)
- Fixed bug: the master was advertising its internal IP when running the dev
  cluster through Vagrant (marko.luksa@gmail.com)
- ignore tag duplicaiton with multiple runs of e2e tests (akostadi@redhat.com)
- make sure errors don't abort tear down (akostadi@redhat.com)
- add image change trigger example to json (bparees@redhat.com)
- correct end-to-end test executable name in HACKING doc (akostadi@redhat.com)
- Fixed SELinux and firewalld commands for temporary disabling.
  (hripps@redhat.com)
- Modified with changes from review 1 (hripps@redhat.com)
- Delete built images after pushing them to registry (cewong@redhat.com)
- Added info about necessary SELinux & firewalld setup (hripps@redhat.com)
- Merge pull request #642 from mfojtik/fix_sti_builder
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #644 from akostadinov/master
  (dmcphers+openshiftbot@redhat.com)
- git is also required, otherwise `go get` does not work (akostadi@redhat.com)
- Do not tag with GIT ref when the SOURCE_REF is not set (sti-image-builder)
  (mfojtik@redhat.com)
- Fixed unpacking of sti in sti-image-builder (mfojtik@redhat.com)
- Merge pull request #557 from bparees/add_dependency_id
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #639 from bparees/hardcode_registry_ip
  (dmcphers+openshiftbot@redhat.com)
- introduce image change build trigger logic (bparees@redhat.com)
- force the docker registry ip to be constant (bparees@redhat.com)
- Merge pull request #632 from jwforres/refactor_data_service
  (dmcphers+openshiftbot@redhat.com)
- Refactor data service to better handle resourceVersion, unsubscribe, context
  specific callback lists, etc... (jforrest@redhat.com)
- Merge pull request #631 from ironcladlou/deployment-docs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #629 from mfojtik/sti-builder-image
  (dmcphers+openshiftbot@redhat.com)
- Added STI image builder image (mfojtik@redhat.com)
- Rewrite and re-scope deployment documentation (ironcladlou@gmail.com)
- Merge pull request #627 from bparees/update_readme
  (dmcphers+openshiftbot@redhat.com)
- update readme to work with web console (bparees@redhat.com)
- Double error print in the OpenShift command (ccoleman@redhat.com)
- Merge pull request #584 from ironcladlou/rc-deployments-redux
  (dmcphers+openshiftbot@redhat.com)
- Implement deployments without Deployment (ironcladlou@gmail.com)
- Merge pull request #622 from csrwng/versioned_webhook
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #625 from soltysh/sti_update
  (dmcphers+openshiftbot@redhat.com)
- Add generic webhook payload to versioned types (cewong@redhat.com)
- Merge pull request #628 from jianlinliu/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #621 from thoraxe/containerized-docs-update
  (dmcphers+openshiftbot@redhat.com)
- Updated STI builder according to latest STI version (maszulik@redhat.com)
- bump(github.com/openshift/source-to-
  image/pkg/sti):5813879841b75b7eb88169d3265a0560fdf50b12 (maszulik@redhat.com)
- correct awk command to get correct endpoint (jialiu@redhat.com)
- correct awk command to get correct endpoint (jialiu@redhat.com)
- Merge pull request #624 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Merge pull request #619 from soltysh/sti_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #616 from rajatchopra/master
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #614 from luksa/OPENSHIFT_NUM_MINIONS
  (dmcphers+openshiftbot@redhat.com)
- Fixing sti url (dmcphers@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Merge remote-tracking branch 'upstream/master' into containerized-docs-update
  (erikmjacobs@gmail.com)
- added setup documentation when using the docker container and the sample app
  (erikmjacobs@gmail.com)
- Merge pull request #620 from jwforres/include_hawtio_core
  (dmcphers+openshiftbot@redhat.com)
- tweaked getting started readme (erikmjacobs@gmail.com)
- Pull in the hawtio-core framework into the console (jforrest@redhat.com)
- Merge pull request #618 from ironcladlou/configurable-grunt
  (dmcphers+openshiftbot@redhat.com)
- Updated STI builder according to latest STI version (maszulik@redhat.com)
- bump(github.com/openshift/source-to-
  image/pkg/sti):48cf2e985b571ddc67cfb84a59a339be38b98a81 (maszulik@redhat.com)
- Make grunt hostname/port configurable (ironcladlou@gmail.com)
- Vagrantfile: accommodate empty lines in ~/.awscred (jolamb@redhat.com)
- Merge pull request #613 from mfojtik/process_improvements
  (dmcphers+openshiftbot@redhat.com)
- ssl_fc_has_sni is meant to work post ssl termination, use req_ssl_sni to
  directly lookup the extacted sni from the map. (rchopra@redhat.com)
- Added --parameters and --value options for kubectl#process command
  (mfojtik@redhat.com)
- OPENSHIFT_NUM_MINIONS env var was not honored (marko.luksa@gmail.com)
- Merge pull request #569 from jwforres/capabilities_proposal
  (dmcphers+openshiftbot@redhat.com)
- add instructions for insecure registry config (bparees@redhat.com)
- Merge pull request #602 from thesteve0/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #600 from smarterclayton/rebase
  (dmcphers+openshiftbot@redhat.com)
- Update README.md (scitronpousty@gmail.com)
- Specifically add github.com/docker/docker/pkg/units (ccoleman@redhat.com)
- Refactor to match upstream (ccoleman@redhat.com)
- UPSTREAM: Disable UIs for Kubernetes and etcd (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):6624b64f440a0f10a8d9ca401c3b1
  40f1bf2f945 (ccoleman@redhat.com)
- Merge pull request #599 from jasonkuhrt/patch-1
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #596 from jwforres/etag_assets_commit
  (dmcphers+openshiftbot@redhat.com)
- Cache control assets with ETag based on commit (jforrest@redhat.com)
- Kube make clean doesn't work without Docker (ccoleman@redhat.com)
- Update cli.md (jasonkuhrt@me.com)
- Issue 253 - Added STI scripts location as part of the STI strategy
  (maszulik@redhat.com)
- fix template test fixture (rajatchopra@gmail.com)
- fix template : redis-master label missing. issue#573 (rajatchopra@gmail.com)
- Merge pull request #592 from derekwaynecarr/ns_client_updates
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #594 from smarterclayton/testcase_improvements
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #591 from jwforres/enable_html5_mode
  (dmcphers+openshiftbot@redhat.com)
- Enable html5 mode in the console (jforrest@redhat.com)
- Improve example validation and add template fixtures for example data
  (ccoleman@redhat.com)
- Allow resources posted to /templateConfigs to omit name (ccoleman@redhat.com)
- Template parameter processing shouldn't return errors for unrecognized types
  (ccoleman@redhat.com)
- Fix errors in guestbook template.json (ccoleman@redhat.com)
- Return typed errors from template config processing (ccoleman@redhat.com)
- Set uid, creation timestamp using standard utility from kube
  (decarr@redhat.com)
- Remove debug statement from master.go (ccoleman@redhat.com)
- Templates don't round trip (ccoleman@redhat.com)
- Merge pull request #593 from bparees/interface
  (dmcphers+openshiftbot@redhat.com)
- fix interface typo (bparees@redhat.com)
- Ensure namespace will be serialized in proper location when moving to path
  param (decarr@redhat.com)
- Merge pull request #586 from VojtechVitek/config_delete
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #582 from jhadvig/golint_typo
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #585 from sg00dwin/nav-docs
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #587 from soltysh/cobra_update
  (dmcphers+openshiftbot@redhat.com)
- Update bindata to match upstream dependencies (jforrest@redhat.com)
- bump(github.com/spf13/cobra):e1e66f7b4e667751cf530ddb6e72b79d6eeb0235
  (maszulik@redhat.com)
- UPSTREAM: kubectl delete command: adding labelSelector (vvitek@redhat.com)
- Proposal for capabilities (jforrest@redhat.com)
- Update nav with labels (sgoodwin@redhat.com)
- Merge pull request #575 from jhadvig/golint_fix
  (dmcphers+openshiftbot@redhat.com)
- Incorrect path argument for Golint (j.hadvig@gmail.com)
- Update bindata to match an updated dependency (jforrest@redhat.com)
- Reintroduced Vagrant version sensitivity (hripps@redhat.com)
- Updated Vagrantfile to reflect config processing changes as of Vagrant 1.7.1
  (hripps@redhat.com)
- Clean up (j.hadvig@gmail.com)
- V3 console navigation structure and interaction (sgoodwin@redhat.com)
- Remove API docs and replace with link to Swagger UI (ccoleman@redhat.com)
- Support the swagger API from OpenShift master (ccoleman@redhat.com)
- UPSTREAM: Take HandlerContainer as input to master to allow extension
  (ccoleman@redhat.com)
- Merge pull request #500 from smarterclayton/add_image_repo_status
  (dmcphers+openshiftbot@redhat.com)
- Fix 'openshift cli' longDesc string arg reference (vvitek@redhat.com)
- Merge pull request #514 from pmorie/router-refactor
  (dmcphers+openshiftbot@redhat.com)
- Router refactor (pmorie@gmail.com)
- Merge pull request #542 from marianitadn/bug-1171673
  (dmcphers+openshiftbot@redhat.com)
- fix #551, make the service ip range private (deads@redhat.com)
- Cancel new builds. Update client logic. (maria.nita.dn@gmail.com)
- Cleanup OpenShift CLI (ccoleman@redhat.com)
- test-cmd should use raw 'osc' calls (ccoleman@redhat.com)
- Make test-end-to-end more readable (ccoleman@redhat.com)
- Golint is broken on Mac (ccoleman@redhat.com)
- Merge pull request #547 from deads2k/deads-rebase-kubernetes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #562 from jwforres/fix_project_description
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #559 from VojtechVitek/checkErr
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #522 from akram/router_controller_fixed
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #470 from deads2k/deads-add-challenging-cli-transport
  (dmcphers+openshiftbot@redhat.com)
- tweaks to handle new kubernetes, kick travis again (deads@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):3910b2d6e18759b3a9a6920c7f3d0
  ccd122df7f9 (deads@redhat.com)
- Move from go 1.4rc2 to 1.4 release (jordan@liggitt.net)
- Merge pull request #564 from bparees/sample_helper
  (dmcphers+openshiftbot@redhat.com)
- helper script to fix registry ip (bparees@redhat.com)
- Merge pull request #541 from mfojtik/custom_build
  (dmcphers+openshiftbot@redhat.com)
- Rename STIBuildStrategy.BuildImage to Image (mfojtik@redhat.com)
- Refactoring router.go to handle errors. Improving code coverage - Add port
  forwarding from 8080 to localhost:8080 on VirtualBox - Refactoring router.go
  to handle errors. Improving code coverage. - Adding port forwarding from 80
  to localhost:1080 on VirtualBox, and comments on how to test locally -
  Applying refactor change to controller/test/test_router.go - Fixing
  identation - Fixing unit tests - Organizing imports - Merge (akram@free.fr)
- Fix project description in example and console to work as an annotation
  (jforrest@redhat.com)
- fix auth packages, kick travis yet again (deads@redhat.com)
- add cli challenge interaction, kick travis again (deads@redhat.com)
- Merge pull request #555 from deads2k/deads-track-down-flaky-travis
  (dmcphers+openshiftbot@redhat.com)
- Added documentation for custom build (mfojtik@redhat.com)
- cancel-build fails if no pod was assigned (deads@redhat.com)
- Initial addition of CustomBuild type (mfojtik@redhat.com)
- Improve error reporting (vvitek@redhat.com)
- add instructions to edit registry service ip (bparees@redhat.com)
- Add go1.4 to travis (jliggitt@redhat.com)
- Merge pull request #544 from pmorie/issue538
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #483 from deads2k/deads-rebase-kubernetes
  (dmcphers+openshiftbot@redhat.com)
- EventQueue should provide events for replaced state (pmorie@gmail.com)
- Merge pull request #543 from mfojtik/fix_install_assets
  (dmcphers+openshiftbot@redhat.com)
- Exit install-assets.sh when the command fails (mfojtik@redhat.com)
- make openshift run on new kubernetes (deads@redhat.com)
- UPSTREAM: go-restful, fix race conditions https://github.com/emicklei/go-
  restful/pull/168 (deads@redhat.com)
- UPSTREAM: Add util.Until (ccoleman@redhat.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):b614f22935df36f8c1d6bd3c5c9fe
  850e79fd729 (deads@redhat.com)
- Add a Status field on ImageRepository (ccoleman@redhat.com)
- Various Go style fixes (mfojtik@redhat.com)
- Merge pull request #532 from csrwng/fix_webhook_display
  (dmcphers+openshiftbot@redhat.com)
- increase retries to account for slow systems (bparees@redhat.com)
- Merge pull request #506 from pmorie/router-shards-proposal
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #443 from marianitadn/cancel-builds
  (dmcphers+openshiftbot@redhat.com)
- Add command to cancel build     - Add flag to print build logs  - Add flag to
  restart build     - Test command (maria.nita.dn@gmail.com)
- Merge pull request #535 from bparees/e2e_build_logs
  (dmcphers+openshiftbot@redhat.com)
- Add PodManager to build resource, to handle pod delete and create
  (maria.nita.dn@gmail.com)
- Add resource flag and status for cancelling build (maria.nita.dn@gmail.com)
- Merge pull request #526 from mfojtik/install-assets-quiet
  (dmcphers+openshiftbot@redhat.com)
- always dump build log to build.log (bparees@redhat.com)
- Router sharding proposal 2/2 (pmorie@gmail.com)
- Router sharding proposal 1/2 (pweil@redhat.com)
- Simplify webhook URL display (cewong@redhat.com)
- Add strategy and revision output to Build describer (cewong@redhat.com)
- Merge pull request #447 from fabianofranz/osc_skeleton
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #463 from iNecas/vagrant-libvirt
  (dmcphers+openshiftbot@redhat.com)
- Marks 'openshift kube' as deprecated (contact@fabianofranz.com)
- Removes 'openshift kubectl' in favor of 'openshift cli', 'kubectl' set as an
  alias (contact@fabianofranz.com)
- Exposes 'openshift cli', removing the osc binary for now
  (contact@fabianofranz.com)
- Basic structure for the end-user client command (osc)
  (contact@fabianofranz.com)
- Install assets more quietly (mfojtik@redhat.com)
- Merge pull request #523 from mfojtik/kubectl_project
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #530 from mfojtik/golint_local
  (dmcphers+openshiftbot@redhat.com)
- Add '-m' option to verify-golint to check just modified files
  (mfojtik@redhat.com)
- Merge pull request #464 from iNecas/fix-vagrant-rsync
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #527 from jhadvig/BZ_1170545
  (dmcphers+openshiftbot@redhat.com)
- Bug 1170545 - Error about deployment not found (j.hadvig@gmail.com)
- Merge pull request #524 from jhadvig/origin_fixes
  (dmcphers+openshiftbot@redhat.com)
- ALL_CAPS to CamelCase (j.hadvig@gmail.com)
- Array declaration change (j.hadvig@gmail.com)
- receiver name should not be an underscore (j.hadvig@gmail.com)
- Reciever name consistency (j.hadvig@gmail.com)
- Removing underscore from method name (j.hadvig@gmail.com)
- Use DeploymentStrategy instead of recreate.DeploymentStrategy
  (j.hadvig@gmail.com)
- replacing 'var += 1' with 'var++' (j.hadvig@gmail.com)
- error strings should not end with punctuation (j.hadvig@gmail.com)
- Fix variable names (j.hadvig@gmail.com)
- Fix panic in webhook printing (mfojtik@redhat.com)
- Refactor printing of values in kubectl describe (mfojtik@redhat.com)
- Add missing Update, Create and Delete methods to Project client
  (mfojtik@redhat.com)
- Remove namespace from Project client and Describer (mfojtik@redhat.com)
- Merge pull request #518 from mfojtik/go_fixes
  (dmcphers+openshiftbot@redhat.com)
- fix service spec camel case (pweil@redhat.com)
- Merge pull request #508 from VojtechVitek/config_labels
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #496 from soltysh/bug1167191
  (dmcphers+openshiftbot@redhat.com)
- Fix #286: Fix Config with nil Labels (vvitek@redhat.com)
- bump(github.com/openshift/source-to-image):
  1075509c5833e58fda33f03ce07307d7193d74f4 (maszulik@redhat.com)
- Simplify code to get rid variable shadowing (maszulik@redhat.com)
- replace build trigger with build command (bparees@redhat.com)
- Fix failed tests (mfojtik@redhat.com)
- getUrl -> getURL (mfojtik@redhat.com)
- Rename grant.GrantFormRenderer to grant.FormRenderer (mfojtik@redhat.com)
- Fix variable names (Url -> URL, client_id -> clientID, etc...)
  (mfojtik@redhat.com)
- Replace ALL_CAPS in basicauth_test with camel case (mfojtik@redhat.com)
- Use 'template.Processor' instead of 'template.TemplateProcessor'
  (mfojtik@redhat.com)
- Replace errors.New(fmt.Sprintf(...)) with fmt.Errorf(...)
  (mfojtik@redhat.com)
- Get rid of else from an if block when it contains return (mfojtik@redhat.com)
- Ascii should be ASCII (mfojtik@redhat.com)
- webhookUrl should be webhookURL (mfojtik@redhat.com)
- NoDefaultIP should be ErrNoDefaultIP (mfojtik@redhat.com)
- Fixed godoc in server/start (mfojtik@redhat.com)
- Remove else from env() func (mfojtik@redhat.com)
- Fixed godoc for origin/auth package (mfojtik@redhat.com)
- Fixed missing godoc in cmd/infra package (mfojtik@redhat.com)
- Added missing godoc and obsolete notice (mfojtik@redhat.com)
- Added missing godoc (mfojtik@redhat.com)
- Fixed if block that ends with a return in serialization test
  (mfojtik@redhat.com)
- Added godoc for multimapper constant (mfojtik@redhat.com)
- Rename haproxy.HaproxyRouter to haproxy.Router (mfojtik@redhat.com)
- Added Project describer (mfojtik@redhat.com)
- Added client for Project (mfojtik@redhat.com)
- Replace Description with Annotations in Project (mfojtik@redhat.com)
- Fixed wrong User() and UserIdentityMappings in Fake client
  (mfojtik@redhat.com)
- Added kubectl Describer for Origin objects (mfojtik@redhat.com)
- Rename config.mergeMaps() as util.MergeInto() (vvitek@redhat.com)
- UPSTREAM: Add Labels and Annotations to MetadataAccessor (vvitek@redhat.com)
- UPSTREAM: meta_test should not depend on runtime.TypeMeta (vvitek@redhat.com)
- Merge pull request #513 from bparees/use_openshift_jenkins
  (dmcphers+openshiftbot@redhat.com)
- update to use new openshift jenkins image tag (bparees@redhat.com)
- Merge pull request #482 from bparees/link_troubleshooting
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #486 from pweil-/issue-403-cherry-pick
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #492 from jhadvig/build_trigger_kubectl
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #505 from liggitt/cleanup_asset_test
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #510 from bparees/fix_e2e
  (dmcphers+openshiftbot@redhat.com)
- allow unbound skip_image_cleanup variable (bparees@redhat.com)
- Porting manual build-trigger to kubectl (j.hadvig@gmail.com)
- Gzip assets (jliggitt@redhat.com)
- Add dependency on github.com/daaku/go.httpgzip
  (3f59977b58c61991f5ed3670bbd141937d808b06) (jliggitt@redhat.com)
- Simplify asset test, keep line breaks in css and js (jliggitt@redhat.com)
- Merge pull request #499 from jimmidyson/cadvisor-update
  (dmcphers+openshiftbot@redhat.com)
- Change AddRoute signature to use ep structs, factor out functions for
  testability, add unit tests for said functions (pweil@redhat.com)
- Merge pull request #487 from bparees/add_jenkins
  (dmcphers+openshiftbot@redhat.com)
- add jenkins sample (bparees@redhat.com)
- Fixup the gofmt'ed bindata.go file (jforrest@redhat.com)
- UPSTREAM: Fixes #458 - retrieval of Docker container stats from cadvisor
  (jimmidyson@gmail.com)
- bump(github.com/google/cadvisor): 89088df70eca64cf9d6b9a23a3d2bc21a30916d6
  (jimmidyson@gmail.com)
- Merge pull request #453 from akram/port_forwarding
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #448 from derekwaynecarr/project_example_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #488 from pmorie/router (dmcphers+openshiftbot@redhat.com)
- Merge pull request #497 from smarterclayton/better_aliased_commands
  (dmcphers+openshiftbot@redhat.com)
- Stop deployment controllers during integration tests (ccoleman@redhat.com)
- UPSTREAM: Add util.Until (ccoleman@redhat.com)
- Prepare for nested commands (ccoleman@redhat.com)
- bump(github.com/spf13/cobra): b825817fc0fc59fc1657bc8202204a04ae3d679d
  (ccoleman@redhat.com)
- Merge pull request #494 from mfojtik/kubectl_readme
  (dmcphers+openshiftbot@redhat.com)
- Added docs/cli.md describing the kubectl interface (mfojtik@redhat.com)
- Merge pull request #495 from soltysh/common_fixups
  (dmcphers+openshiftbot@redhat.com)
- Fixed typos, mixing comments and removed unused method (maszulik@redhat.com)
- Fix documentation to use kubectl instead of kubecfg (mfojtik@redhat.com)
- Replace kubecfg with kubectl in e2e test (mfojtik@redhat.com)
- Fix incorrect namespace for origin kubectl commands (mfojtik@redhat.com)
- Fixed typo in kubectl build-log (mfojtik@redhat.com)
- Cleanup godoc / tests for router, event queue (pmorie@gmail.com)
- Merge pull request #475 from mfojtik/verify_golint
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #485 from mfojtik/skip_image_cleanup
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #481 from jhadvig/type_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #484 from mfojtik/kubectl_lost_commits
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #480 from pmorie/router (dmcphers+openshiftbot@redhat.com)
- Add SKIP_IMAGE_CLEANUP env variable for E2E test (mfojtik@redhat.com)
- Fixed test-service.json fixture to be v1beta2 (mfojtik@redhat.com)
- Switch test-cmd.sh to use kubectl instead of kubecfg (mfojtik@redhat.com)
- Use ResourceFromFile to get namespace and data (mfojtik@redhat.com)
- Merge pull request #474 from mfojtik/kubectl
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #471 from derekwaynecarr/hack_pythia
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #401 from jwforres/project_overview
  (dmcphers+openshiftbot@redhat.com)
- Refactor router controller unit tests (pmorie@gmail.com)
- link debugging guide from other docs (bparees@redhat.com)
- Merge pull request #465 from bparees/troubleshooting
  (dmcphers+openshiftbot@redhat.com)
- Correct urlVars field name (j.hadvig@gmail.com)
- UPSTREAM: Fix pluralization in RESTMapper when kind ends with 'y' (#2569)
  (mfojtik@redhat.com)
- Define printer for Origin objects in kubectl (mfojtik@redhat.com)
- Added TODOs and NOTE into Config#Apply (mfojtik@redhat.com)
- Added 'openshift kubectl build-logs' command (mfojtik@redhat.com)
- Added 'openshift kubectl process' command (mfojtik@redhat.com)
- Added 'openshift kubectl apply' command (mfojtik@redhat.com)
- Initial import for pkg/cmd/kubectl into origin (mfojtik@redhat.com)
- Added MultiRESTMapper into pkg/api/meta (mfojtik@redhat.com)
- create a troubleshooting guide (bparees@redhat.com)
- print invalid responses (bparees@redhat.com)
- Added ./hack/verify-golint.sh command (mfojtik@redhat.com)
- Make install-assets.sh work outside TRAVIS (jliggitt@redhat.com)
- Initial implementation of a project's overview in the web console
  (jforrest@redhat.com)
- Add hack script for pythia tool (decarr@redhat.com)
- Add EventQueue and refactor LBManager to use client/cache (pmorie@gmail.com)
- Merge pull request #468 from jwforres/bindata_stabilized_output
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #460 from jhadvig/build_trigger
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #429 from pweil-/router-multi
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #457 from liggitt/fixup_oauth_types
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #445 from soltysh/client_interface_update
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #462 from csrwng/fix_items_annotations
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/jteeuwen/go-bindata):f94581bd91620d0ccd9c22bb1d2de13f6a605857
  (jforrest@redhat.com)
- Include required fields on AccessToken type, always serialize Items fields
  (jliggitt@redhat.com)
- Manual build trigger (j.hadvig@gmail.com)
- Rework client.Interface to match upstream (maszulik@redhat.com)
- vagrant-libvirt support (inecas@redhat.com)
- Fix vagrant rsync (inecas@redhat.com)
- Remove omitempty annotation from Items property in list types
  (cewong@redhat.com)
- Add build namespace to built image environment (cewong@redhat.com)
- Use upstream ParseWatchResourceVersion (agoldste@redhat.com)
- Merge pull request #456 from ncdc/image-repository-fixes
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #410 from csrwng/manual_build_trigger
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #409 from csrwng/buildconfig_watch
  (dmcphers+openshiftbot@redhat.com)
- ImageRepository fixes (agoldste@redhat.com)
- Add auth-proxy-test (deads@redhat.com)
- Manual build launch (cewong@redhat.com)
- Add watch method for build configs (cewong@redhat.com)
- Adding port forwarding from 80 to localhost:1080 on VirtualBox, fixing indent
  and comments on how to test locally (akram@free.fr)
- Ref mailing list (ccoleman@redhat.com)
- Merge pull request #451 from bparees/bump_sti_version
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #446 from liggitt/identity_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #449 from derekwaynecarr/fixup_makefile
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #413 from csrwng/fix_generic_webhook_errormsg
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #430 from csrwng/fix_build_api_json_tags
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #415 from jhadvig/logging_fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #452 from bparees/fix_samples
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/source-to-image)
  81ea479a67c279351661653c2a40f9428d4e259b (bparees@redhat.com)
- update registery ip references to reflect new default services
  (bparees@redhat.com)
- Merge pull request #450 from ironcladlou/recreate-strategy-correlation-fix
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #406 from deads2k/deads-add-union-auth-request-handler
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #417 from ironcladlou/image-change-trigger-fix
  (dmcphers+openshiftbot@redhat.com)
- Change Identity.Name to Identity.UserName (jliggitt@redhat.com)
- Correlate Pods to Deployments during Recreate (ironcladlou@gmail.com)
- add union auth request handler (deads@redhat.com)
- Merge pull request #422 from bparees/generic_hook
  (dmcphers+openshiftbot@redhat.com)
- Go builds are under /local (decarr@redhat.com)
- Fix project example after rebase (decarr@redhat.com)
- update readme to use generic webhook (bparees@redhat.com)
- add endpoint for displaying token (deads@redhat.com)
- Merge pull request #425 from liggitt/auth_interfaces
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #438 from bparees/param_registry
  (dmcphers+openshiftbot@redhat.com)
- Fix error message in generic webhook (cewong@redhat.com)
- Merge pull request #416 from bparees/fix_nil_generic
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #437 from jwforres/update_bindata_godep
  (dmcphers+openshiftbot@redhat.com)
- Make auth interfaces more flexible (jliggitt@redhat.com)
- update registry config to be more self-sufficient (bparees@redhat.com)
- Fix Build API JSON tags for BuildStrategy (cewong@redhat.com)
- Fix deployment image change trigger bug (ironcladlou@gmail.com)
- fix checking for nil body on generic webhook invocation (bparees@redhat.com)
- minor change to install script to allow passing id. Moved router readme to
  docs. Updated readme for HA setup and documented DNS RR. (pweil@redhat.com)
- Fix ID -> Name in printers (pmorie@gmail.com)
- Kube rebase (3/3) (soltysh@gmail.com)
- Kube rebase (2/3) (mfojtik@redhat.com)
- Kube rebase (1/3) (contact@fabianofranz.com)
- bump(github.com/GoogleCloudPlatform/kubernetes):
  97cb1fa2df8b57be5ceaae290e02872f291b7b7e (contact@fabianofranz.com)
- Update to use uncompressed bindata for easier diffs (jforrest@redhat.com)
- bump(github.com/jteeuwen/go-bindata):bbd0c6e271208dce66d8fda4bc536453cd27fc4a
  (jforrest@redhat.com)
- Fixing typos (dmcphers@redhat.com)
- Log streaming logic update (j.hadvig@gmail.com)
- Add --net=host option for simplicity in Docker steps. (ccoleman@redhat.com)
- Docker run command was wrong (ccoleman@redhat.com)
- Refactor build config to use triggers (cewong@redhat.com)
- Merge pull request #399 from bparees/update_sti_version
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #402 from csrwng/docker_no_cache
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #395 from csrwng/fix_api_refs
  (dmcphers+openshiftbot@redhat.com)
- Add NoCache flag to docker builds (cewong@redhat.com)
- Merge pull request #397 from pmorie/docs (dmcphers+openshiftbot@redhat.com)
- Merge pull request #383 from pweil-/router-e2e-rebase
  (dmcphers+openshiftbot@redhat.com)
- bump(github.com/openshift/source-to-image)
  53a27ab4cc8c6abfe84904a6503490bbf0bf7abb (bparees@redhat.com)
- Fix API references (cewong@redhat.com)
- router e2e integration (pweil@redhat.com)
- Deployment proposal: add detail to image spec (pmorie@gmail.com)
- Merge pull request #393 from ironcladlou/deploy-int-test-improvements
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #392 from jcantrill/116318_missing_arg_webhook_log_message
  (dmcphers+openshiftbot@redhat.com)
- Strengthen deploy int test assertions (ironcladlou@gmail.com)
- Merge pull request #390 from ironcladlou/constant-cleanup
  (dmcphers+openshiftbot@redhat.com)
- [BZ1163618] Add missing user agent to error message (jcantril@redhat.com)
- Merge pull request #389 from ironcladlou/recreate-strategy-tests
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #391 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Clean up and add tests to the Recreate strategy (ironcladlou@gmail.com)
- Better sudoer seding (dmcphers@redhat.com)
- Merge pull request #372 from deads2k/deads-add-token-option-to-cli
  (dmcphers+openshiftbot@redhat.com)
- Use constants for deployment annotations (ironcladlou@gmail.com)
- Merge pull request #367 from deads2k/deads-add-user-identity-provisioning
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #388 from csrwng/fix_watch_resource_kind
  (dmcphers+openshiftbot@redhat.com)
- add user identity mapping provisioning (deads@redhat.com)
- Merge pull request #386 from pweil-/router-pr303-combined
  (dmcphers+openshiftbot@redhat.com)
- Fix build watch resource kind (cewong@redhat.com)
- Update deployment API examples and docs (ironcladlou@gmail.com)
- Fix naming of openshift/origin-deployer image (ironcladlou@gmail.com)
- Implement deployments with pods (ironcladlou@gmail.com)
- comments round 1 (deads@redhat.com)
- gofmt (pweil@redhat.com)
- test case and remove newline from glog statements (pweil@redhat.com)
- Do not delete the entire structure when just an alias is removed. bz1157388
  (rchopra@redhat.com)
- Merge pull request #364 from marianitadn/test-examples
  (dmcphers+openshiftbot@redhat.com)
- Godoc fix (j.hadvig@gmail.com)
- typo fixed (deads@redhat.com)
- Remove obsolete API examples (maria.nita.dn@gmail.com)
- Add command to run gofmt for bad formatted Go files (maria.nita.dn@gmail.com)
- Check all expected files have an existent JSON file. Update list of expected
  files (maria.nita.dn@gmail.com)
- Generate API doc for new examples (maria.nita.dn@gmail.com)
- Validate API examples. Rename files (maria.nita.dn@gmail.com)
- Merge pull request #362 from csrwng/add_clean_build_flag
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #381 from danmcp/master (dmcphers+openshiftbot@redhat.com)
- Merge pull request #379 from ironcladlou/image-dockerfile-fix
  (dmcphers+openshiftbot@redhat.com)
- Switch back to gp2 volumes (dmcphers@redhat.com)
- Fix typos (dmcphers@redhat.com)
- add token option to clientcmd (deads@redhat.com)
- Fix WORKDIR typo in Dockerfile (ironcladlou@gmail.com)
- Fix typo (dmcphers@redhat.com)
- use token from command line (deads@redhat.com)
- update openshift path (bparees@users.noreply.github.com)
- Merge pull request #363 from pweil-/router-readme-vagrant
  (abhgupta@redhat.com)
- Merge pull request #376 from smarterclayton/make_start_testable
  (dmcphers+openshiftbot@redhat.com)
- Remove extra binaries (ccoleman@redhat.com)
- Move standalone commands into the pkg/cmd pattern (ccoleman@redhat.com)
- Add utility functions for commands that connect to a master
  (ccoleman@redhat.com)
- Move start logic into its own method and return errors. (ccoleman@redhat.com)
- router readme and install script (pweil@redhat.com)
- Fixing typo in URL (abhgupta@redhat.com)
- Use a standard working dir for Docker images (ccoleman@redhat.com)
- Merge pull request #373 from pmorie/docs (ccoleman@redhat.com)
- OpenShift in a container README (ccoleman@redhat.com)
- Minor changes to test artifacts (pmorie@gmail.com)
- WIP: deployments proposal (pmorie@gmail.com)
- Merge pull request #370 from ironcladlou/deploy-watch-api
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #350 from bparees/enable_sti
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #369 from csrwng/bump_sti_add9ff4973
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #334 from liggitt/oauth_grant
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #343 from jcantrill/391_generic_web_hook
  (dmcphers+openshiftbot@redhat.com)
- Enhance deployment list and watch APIs (ironcladlou@gmail.com)
- bump(github.com/openshift/source-to-image)
  add9ff4973d949b4c82fb6a217e6919bb6e6be23 (cewong@redhat.com)
- Add grant approval, scope.Add, tests (jliggitt@redhat.com)
- Add a flag to support clean STI builds to the STIBuildStrategy
  (cewong@redhat.com)
- Convert builder images to use go (cewong@redhat.com)
- Fix deployment pod template comparison (cewong@redhat.com)
- rework sample to use STI build type (bparees@redhat.com)
- [DEVEXP 391] Add generic webhook to trigger builds manually
  (jcantril@redhat.com)
- Merge pull request #360 from liggitt/coverage
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #353 from VojtechVitek/template_registry_structure
  (dmcphers+openshiftbot@redhat.com)
- Document test-go.sh, add option to show coverage for all tests
  (jliggitt@redhat.com)
- Merge pull request #336 from pmorie/deploy-trigger-wip
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #355 from deads2k/deads-use-common-mocks
  (dmcphers+openshiftbot@redhat.com)
- Make deploymentConfigs with config change triggers deploy automatically when
  created (pmorie@gmail.com)
- Merge pull request #359 from liggitt/coverage
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #352 from VojtechVitek/hacking_godep
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #330 from deads2k/deads-rebase-kubernetes-for-demo
  (dmcphers+openshiftbot@redhat.com)
- Generate html coverage info (jliggitt@redhat.com)
- Mention commit message format for bumping Godeps (vvitek@redhat.com)
- make use of common mock objects (deads@redhat.com)
- Updated pre-pulled images (soltysh@gmail.com)
- Fixed buildLogs (soltysh@gmail.com)
- Reorganize the template pkg, make it match upstream patterns
  (vvitek@redhat.com)
- Merge pull request #345 from pweil-/fix-route-watch
  (dmcphers+openshiftbot@redhat.com)
- Merge pull request #349 from ncdc/fix-flakes
  (dmcphers+openshiftbot@redhat.com)
- More flaky test fixes (agoldste@redhat.com)
- UPSTREAM: Support PUT returning 201 on Create, kick travis
  (ccoleman@redhat.com)
- kubernetes cherry-pick 2074 and 2140 (deads@redhat.com)
- gofmt etcd_test.go and fix broken route test after change to argument type
  (pweil@redhat.com)
- unit test for route watches (pweil@redhat.com)
- fix issues with route watches returning 404 and 500 errors (pweil@redhat.com)

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
