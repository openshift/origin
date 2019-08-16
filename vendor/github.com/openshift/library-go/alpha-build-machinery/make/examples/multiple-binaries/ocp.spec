#debuginfo not supported with Go
%global debug_package %{nil}
# modifying the Go binaries breaks the DWARF debugging
%global __os_install_post %{_rpmconfigdir}/brp-compress

%global golang_version 1.12
%global product_name OpenShift

%{!?version: %global version 0.0.1}
%{!?release: %global release 1}

Name:           openshift
Version:        %{version}
Release:        %{release}%{dist}
Summary:        OpenShift client binaries
License:        ASL 2.0
URL:            https://%{go_package}

# If go_arches not defined fall through to implicit golang archs
%if 0%{?go_arches:1}
ExclusiveArch:  %{go_arches}
%else
ExclusiveArch:  x86_64 aarch64 ppc64le s390x
%endif

#BuildRequires:  bsdtar
BuildRequires:  golang >= %{golang_version}

%description
%{summary}

%prep

%build
make build

%install
install -d %{buildroot}%{_bindir}

install -p -m 755 oc %{buildroot}%{_bindir}/oc
install -p -m 755 openshift %{buildroot}%{_bindir}/openshift

%files
%{_bindir}/oc
%{_bindir}/openshift

%changelog
