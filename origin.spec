#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode
%global import_path github.com/openshift/origin
%global sdn_import_path github.com/openshift/openshift-sdn

# docker_version is the version of docker requires by packages
%global docker_version 1.8.2
# tuned_version is the version of tuned requires by packages
%global tuned_version  2.3
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.3.1
# this is the version we obsolete up to. The packaging changed for Origin
# 1.0.6 and OSE 3.1 such that 'openshift' package names were no longer used.
%global package_refector_version 3.0.2.900
# %commit and %ldflags are intended to be set by tito custom builders provided
# in the .tito/lib directory. The values in this spec file will not be kept up to date.
%{!?commit:
%global commit 86b5e46426ba828f49195af21c56f7c6674b48f7
}
%global shortcommit %(c=%{commit}; echo ${c:0:7})
# ldflags from hack/common.sh os::build:ldflags
%{!?ldflags:
%global ldflags -X github.com/openshift/origin/pkg/version.majorFromGit 0 -X github.com/openshift/origin/pkg/version.minorFromGit 0+ -X github.com/openshift/origin/pkg/version.versionFromGit v0.0.1 -X github.com/openshift/origin/pkg/version.commitFromGit 86b5e46 -X k8s.io/kubernetes/pkg/version.gitCommit 6241a21 -X k8s.io/kubernetes/pkg/version.gitVersion v0.11.0-330-g6241a21
}

 %if 0%{?fedora} || 0%{?epel}
%global make_redistributable 0
%else
%global make_redistributable 1
%endif

%if "%{dist}" == ".el7aos"
%global package_name atomic-openshift
%global product_name Atomic OpenShift
%else
%global package_name origin
%global product_name Origin
%endif

Name:           %{package_name}
# Version is not kept up to date and is intended to be set by tito custom
# builders provided in the .tito/lib directory of this project
Version:        0.0.1
Release:        0%{?dist}
Summary:        Open Source Container Management by Red Hat
License:        ASL 2.0
URL:            https://%{import_path}
ExclusiveArch:  x86_64
Source0:        https://%{import_path}/archive/%{commit}/%{name}-%{version}.tar.gz
BuildRequires:  systemd
BuildRequires:  golang >= 1.4
Requires:       %{name}-clients = %{version}-%{release}
Requires:       iptables
Obsoletes:      openshift < %{package_refector_version}

# This bundled provides section was generated using the define_bundled_spec.py
# script found in the relative root of the origin source code.
#
# These are defined as per:
# https://fedoraproject.org/wiki/Packaging:Guidelines#Bundling_and_Duplication_of_system_libraries
#
Provides: golang(bundled(bitbucket.org/ww/goautoneg)) = 75cd24fc2f2c2a2088577d12123ddee5f54e0675
Provides: golang(bundled(code.google.com/p/go-uuid/uuid)) = 7dda39b2e7d5e265014674c5af696ba4186679e9
Provides: golang(bundled(github.com/AaronO/go-git-http)) = 0ebecedc64b67a3a8674c56724082660be48216e
Provides: golang(bundled(github.com/AdRoll/goamz/aws)) = cc210f45dcb9889c2769a274522be2bf70edfb99
Provides: golang(bundled(github.com/AdRoll/goamz/s3)) = cc210f45dcb9889c2769a274522be2bf70edfb99
Provides: golang(bundled(github.com/ClusterHQ/flocker-go)) = 3f33ece70f6571f0ec45bfae2f243ab11fab6c52
Provides: golang(bundled(github.com/MakeNowJust/heredoc)) = 1d91351acdc1cb2f2c995864674b754134b86ca7
Provides: golang(bundled(github.com/RangelReale/osin)) = c07b3bd1ee57089f63e6325c0ea035ceed2e905c
Provides: golang(bundled(github.com/RangelReale/osincli)) = 23618ea0fc3faa3f43954ce8ff48e31f5c784212
Provides: golang(bundled(github.com/Sirupsen/logrus)) = aaf92c95712104318fc35409745f1533aa5ff327
Provides: golang(bundled(github.com/abbot/go-http-auth)) = c0ef4539dfab4d21c8ef20ba2924f9fc6f186d35
Provides: golang(bundled(github.com/appc/cni/libcni)) = 2a58bd9379ca33579f0cf631945b717aa4fa373d
Provides: golang(bundled(github.com/appc/cni/pkg/invoke)) = 2a58bd9379ca33579f0cf631945b717aa4fa373d
Provides: golang(bundled(github.com/appc/cni/pkg/types)) = 2a58bd9379ca33579f0cf631945b717aa4fa373d
Provides: golang(bundled(github.com/appc/spec/schema)) = c928a0c907c96034dfc0a69098b2179db5ae7e37
Provides: golang(bundled(github.com/aws/aws-sdk-go/aws)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/endpoints)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/protocol/ec2query)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/protocol/query)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/protocol/rest)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/protocol/xml/xmlutil)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/internal/signer/v4)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/service/autoscaling)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/service/ec2)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/aws/aws-sdk-go/service/elb)) = c4ae871ffc03691a7b039fa751a1e7afee56e920
Provides: golang(bundled(github.com/beorn7/perks/quantile)) = b965b613227fddccbfffe13eae360ed3fa822f8d
Provides: golang(bundled(github.com/bradfitz/http2)) = f8202bc903bda493ebba4aa54922d78430c2c42f
Provides: golang(bundled(github.com/coreos/etcd/client)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/discovery)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/error)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/etcdserver)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/migrate)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/crc)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/fileutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/idutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/ioutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/netutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/osutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/pbutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/runtime)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/timeutil)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/transport)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/types)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/pkg/wait)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/raft)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/rafthttp)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/snap)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/storage/storagepb)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/store)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/version)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/etcd/wal)) = ff8d1ecb9f2bf966c0e6929156be4432786b9217
Provides: golang(bundled(github.com/coreos/go-etcd/etcd)) = de3514f25635bbfb024fdaf2a8d5f67378492675
Provides: golang(bundled(github.com/coreos/go-oidc/http)) = ee7cb1fb480df22f7d8c4c90199e438e454ca3b6
Provides: golang(bundled(github.com/coreos/go-oidc/jose)) = ee7cb1fb480df22f7d8c4c90199e438e454ca3b6
Provides: golang(bundled(github.com/coreos/go-oidc/key)) = ee7cb1fb480df22f7d8c4c90199e438e454ca3b6
Provides: golang(bundled(github.com/coreos/go-oidc/oauth2)) = ee7cb1fb480df22f7d8c4c90199e438e454ca3b6
Provides: golang(bundled(github.com/coreos/go-oidc/oidc)) = ee7cb1fb480df22f7d8c4c90199e438e454ca3b6
Provides: golang(bundled(github.com/coreos/go-semver/semver)) = d043ae190b3202550d026daf009359bb5d761672
Provides: golang(bundled(github.com/coreos/go-systemd/activation)) = 97e243d21a8e232e9d8af38ba2366dfcfceebeba
Provides: golang(bundled(github.com/coreos/go-systemd/daemon)) = 97e243d21a8e232e9d8af38ba2366dfcfceebeba
Provides: golang(bundled(github.com/coreos/go-systemd/dbus)) = 97e243d21a8e232e9d8af38ba2366dfcfceebeba
Provides: golang(bundled(github.com/coreos/go-systemd/unit)) = 97e243d21a8e232e9d8af38ba2366dfcfceebeba
Provides: golang(bundled(github.com/coreos/pkg/capnslog)) = 42a8c3b1a6f917bb8346ef738f32712a7ca0ede7
Provides: golang(bundled(github.com/coreos/pkg/health)) = 42a8c3b1a6f917bb8346ef738f32712a7ca0ede7
Provides: golang(bundled(github.com/coreos/pkg/httputil)) = 42a8c3b1a6f917bb8346ef738f32712a7ca0ede7
Provides: golang(bundled(github.com/coreos/pkg/timeutil)) = 42a8c3b1a6f917bb8346ef738f32712a7ca0ede7
Provides: golang(bundled(github.com/cpuguy83/go-md2man/md2man)) = 71acacd42f85e5e82f70a55327789582a5200a90
Provides: golang(bundled(github.com/davecgh/go-spew/spew)) = 3e6e67c4dcea3ac2f25fd4731abc0e1deaf36216
Provides: golang(bundled(github.com/daviddengcn/go-colortext)) = b5c0891944c2f150ccc9d02aecf51b76c14c2948
Provides: golang(bundled(github.com/dgrijalva/jwt-go)) = 5ca80149b9d3f8b863af0e2bb6742e608603bd99
Provides: golang(bundled(github.com/docker/distribution)) = 1341222284b3a6b4e77fb64571ad423ed58b0d34
Provides: golang(bundled(github.com/docker/docker/builder/command)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/builder/parser)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/jsonmessage)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/mount)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/parsers)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/symlink)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/tarsum)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/term)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/timeutils)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/docker/pkg/units)) = 2b27fe17a1b3fb8472fde96d768fa70996adf201
Provides: golang(bundled(github.com/docker/libcontainer)) = 5dc7ba0f24332273461e45bc49edcb4d5aa6c44c
Provides: golang(bundled(github.com/docker/libtrust)) = c54fbb67c1f1e68d7d6f8d2ad7c9360404616a41
Provides: golang(bundled(github.com/docker/spdystream)) = 43bffc458d55aa784be658c9867fbefcfcb7fecf
Provides: golang(bundled(github.com/elazarl/go-bindata-assetfs)) = 3dcc96556217539f50599357fb481ac0dc7439b9
Provides: golang(bundled(github.com/emicklei/go-restful)) = 1f9a0ee00ff93717a275e15b30cf7df356255877
Provides: golang(bundled(github.com/evanphx/json-patch)) = 7dd4489c2eb6073e5a9d7746c3274c5b5f0387df
Provides: golang(bundled(github.com/fsouza/go-dockerclient)) = 1399676f53e6ccf46e0bf00751b21bed329bc60e
Provides: golang(bundled(github.com/garyburd/redigo/internal)) = 535138d7bcd717d6531c701ef5933d98b1866257
Provides: golang(bundled(github.com/garyburd/redigo/redis)) = 535138d7bcd717d6531c701ef5933d98b1866257
Provides: golang(bundled(github.com/getsentry/raven-go)) = 86cd4063c535cbbcbf43d84424dbd5911ab1b818
Provides: golang(bundled(github.com/ghodss/yaml)) = 73d445a93680fa1a78ae23a5839bad48f32ba1ee
Provides: golang(bundled(github.com/go-ldap/ldap)) = b4c9518ccf0d85087c925e4a3c9d5802c9bc7025
Provides: golang(bundled(github.com/godbus/dbus)) = 939230d2086a4f1870e04c52e0a376c25bae0ec4
Provides: golang(bundled(github.com/gogo/protobuf/proto)) = 2093b57e5ca2ccbee4626814100bc1aada691b18
Provides: golang(bundled(github.com/golang/glog)) = 44145f04b68cf362d9c4df2182967c2275eaefed
Provides: golang(bundled(github.com/golang/groupcache/lru)) = 604ed5785183e59ae2789449d89e73f3a2a77987
Provides: golang(bundled(github.com/golang/protobuf/proto)) = 7f07925444bb51fa4cf9dfe6f7661876f8852275
Provides: golang(bundled(github.com/gonum/blas)) = 80dca99229cccca259b550ae3f755cf79c65a224
Provides: golang(bundled(github.com/gonum/graph)) = bde6d0fbd9dec5a997e906611fe0364001364c41
Provides: golang(bundled(github.com/gonum/internal/asm)) = 5b84ddfb9d3e72d73b8de858c97650be140935c0
Provides: golang(bundled(github.com/gonum/lapack)) = 88ec467285859a6cd23900147d250a8af1f38b10
Provides: golang(bundled(github.com/gonum/matrix/mat64)) = fb1396264e2e259ff714a408a7b0142d238b198d
Provides: golang(bundled(github.com/google/cadvisor/api)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/cache/memory)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/collector)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/container)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/events)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/fs)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/healthz)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/http)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/info/v1)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/info/v2)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/manager)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/metrics)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/pages)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/storage)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/summary)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/utils)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/validate)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/cadvisor/version)) = aa6f80814bc6fdb43a0ed12719658225420ffb7d
Provides: golang(bundled(github.com/google/gofuzz)) = bbcb9da2d746f8bdbd6a936686a0a6067ada0ec5
Provides: golang(bundled(github.com/gorilla/context)) = 215affda49addc4c8ef7e2534915df2c8c35c6cd
Provides: golang(bundled(github.com/gorilla/handlers)) = 4ef72b2795a418935d497c8db213080be06f8850
Provides: golang(bundled(github.com/gorilla/mux)) = 8096f47503459bcc74d1f4c487b7e6e42e5746b5
Provides: golang(bundled(github.com/gorilla/securecookie)) = 1b0c7f6e9ab3d7f500fd7d50c7ad835ff428139b
Provides: golang(bundled(github.com/gorilla/sessions)) = aa5e036e6c44aec69a32eb41097001978b29ad31
Provides: golang(bundled(github.com/hashicorp/golang-lru)) = 7f9ef20a0256f494e24126014135cf893ab71e9e
Provides: golang(bundled(github.com/imdario/mergo)) = 6633656539c1639d9d78127b7d47c622b5d7b6dc
Provides: golang(bundled(github.com/inconshreveable/mousetrap)) = 76626ae9c91c4f2a10f34cad8ce83ea42c93bb75
Provides: golang(bundled(github.com/influxdb/influxdb/client)) = afde71eb1740fd763ab9450e1f700ba0e53c36d0
Provides: golang(bundled(github.com/jlhawn/go-crypto)) = cd738dde20f0b3782516181b0866c9bb9db47401
Provides: golang(bundled(github.com/jonboulle/clockwork)) = 3f831b65b61282ba6bece21b91beea2edc4c887a
Provides: golang(bundled(github.com/jteeuwen/go-bindata)) = bfe36d3254337b7cc18024805dfab2106613abdf
Provides: golang(bundled(github.com/juju/ratelimit)) = 772f5c38e468398c4511514f4f6aa9a4185bc0a0
Provides: golang(bundled(github.com/kr/pty)) = 05017fcccf23c823bfdea560dcc958a136e54fb7
Provides: golang(bundled(github.com/matttproud/golang_protobuf_extensions/pbutil)) = fc2b8d3a73c4867e51861bbdd5ae3c1f0869dd6a
Provides: golang(bundled(github.com/mesos/mesos-go/detector)) = b164c06f346af1e93aecb6502f83d31dbacdbb91
Provides: golang(bundled(github.com/mesos/mesos-go/mesosproto)) = b164c06f346af1e93aecb6502f83d31dbacdbb91
Provides: golang(bundled(github.com/mesos/mesos-go/mesosutil)) = b164c06f346af1e93aecb6502f83d31dbacdbb91
Provides: golang(bundled(github.com/mesos/mesos-go/upid)) = b164c06f346af1e93aecb6502f83d31dbacdbb91
Provides: golang(bundled(github.com/miekg/dns)) = 46e689ee1104f5db16a173a798fb0f0ee5c7d3ef
Provides: golang(bundled(github.com/mitchellh/mapstructure)) = 740c764bc6149d3f1806231418adb9f52c11bcbf
Provides: golang(bundled(github.com/mxk/go-flowrate/flowrate)) = cca7078d478f8520f85629ad7c68962d31ed7682
Provides: golang(bundled(github.com/onsi/ginkgo)) = d981d36e9884231afa909627b9c275e4ba678f90
Provides: golang(bundled(github.com/onsi/gomega)) = 8adf9e1730c55cdc590de7d49766cb2acc88d8f2
Provides: golang(bundled(github.com/openshift/openshift-sdn/pkg)) = d5965ee039bb85c5ec9ef7f455a8c03ac0ff0214
Provides: golang(bundled(github.com/openshift/openshift-sdn/plugins)) = d5965ee039bb85c5ec9ef7f455a8c03ac0ff0214
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/api)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/build)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/docker)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/errors)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/ignore)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/scm)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/scripts)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/tar)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/openshift/source-to-image/pkg/util)) = c9985b5443c4a0a0ffb38b3478031dcc2dc8638d
Provides: golang(bundled(github.com/pborman/uuid)) = ca53cad383cad2479bbba7f7a1a05797ec1386e4
Provides: golang(bundled(github.com/pkg/profile)) = c795610ec6e479e5795f7852db65ea15073674a6
Provides: golang(bundled(github.com/prometheus/client_golang/extraction)) = 692492e54b553a81013254cc1fba4b6dd76fad30
Provides: golang(bundled(github.com/prometheus/client_golang/prometheus)) = 3b78d7a77f51ccbc364d4bc170920153022cfd08
Provides: golang(bundled(github.com/prometheus/client_model/go)) = fa8ad6fec33561be4280a8f0514318c79d7f6cb6
Provides: golang(bundled(github.com/prometheus/common/expfmt)) = ef7a9a5fb138aa5d3a19988537606226869a0390
Provides: golang(bundled(github.com/prometheus/common/model)) = ef7a9a5fb138aa5d3a19988537606226869a0390
Provides: golang(bundled(github.com/prometheus/procfs)) = 490cc6eb5fa45bf8a8b7b73c8bc82a8160e8531d
Provides: golang(bundled(github.com/rackspace/gophercloud)) = f92863476c034f851073599c09d90cd61ee95b3d
Provides: golang(bundled(github.com/russross/blackfriday)) = 8cec3a854e68dba10faabbe31c089abf4a3e57a6
Provides: golang(bundled(github.com/samuel/go-zookeeper/zk)) = 177002e16a0061912f02377e2dd8951a8b3551bc
Provides: golang(bundled(github.com/scalingdata/gcfg)) = 37aabad69cfd3d20b8390d902a8b10e245c615ff
Provides: golang(bundled(github.com/shurcooL/sanitized_anchor_name)) = 244f5ac324cb97e1987ef901a0081a77bfd8e845
Provides: golang(bundled(github.com/skynetservices/skydns/backends/etcd)) = bb2ebadc9746f23e4a296e3cbdb8c01e956baee1
Provides: golang(bundled(github.com/skynetservices/skydns/cache)) = bb2ebadc9746f23e4a296e3cbdb8c01e956baee1
Provides: golang(bundled(github.com/skynetservices/skydns/msg)) = bb2ebadc9746f23e4a296e3cbdb8c01e956baee1
Provides: golang(bundled(github.com/skynetservices/skydns/server)) = bb2ebadc9746f23e4a296e3cbdb8c01e956baee1
Provides: golang(bundled(github.com/spf13/cobra)) = d732ab3a34e6e9e6b5bdac80707c2b6bad852936
Provides: golang(bundled(github.com/spf13/pflag)) = b084184666e02084b8ccb9b704bf0d79c466eb1d
Provides: golang(bundled(github.com/stretchr/objx)) = d40df0cc104c06eae2dfe03d7dddb83802d52f9a
Provides: golang(bundled(github.com/stretchr/testify/assert)) = 089c7181b8c728499929ff09b62d3fdd8df8adff
Provides: golang(bundled(github.com/stretchr/testify/mock)) = 089c7181b8c728499929ff09b62d3fdd8df8adff
Provides: golang(bundled(github.com/stretchr/testify/require)) = 089c7181b8c728499929ff09b62d3fdd8df8adff
Provides: golang(bundled(github.com/syndtr/gocapability/capability)) = 2c00daeb6c3b45114c80ac44119e7b8801fdd852
Provides: golang(bundled(github.com/ugorji/go/codec)) = 2f4b94206aae781e63846a9bf02ad83c387d5296
Provides: golang(bundled(github.com/vaughan0/go-ini)) = a98ad7ee00ec53921f08832bc06ecf7fd600e6a1
Provides: golang(bundled(github.com/vjeantet/asn1-ber)) = 85041cd0f4769ebf4a5ae600b1e921e630d6aff0
Provides: golang(bundled(github.com/vjeantet/ldapserver)) = 19fbc46ed12348d5122812c8303fb82e49b6c25d
Provides: golang(bundled(golang.org/x/crypto/bcrypt)) = c84e1f8e3a7e322d497cd16c0e8a13c7e127baf3
Provides: golang(bundled(golang.org/x/crypto/blowfish)) = c84e1f8e3a7e322d497cd16c0e8a13c7e127baf3
Provides: golang(bundled(golang.org/x/crypto/ssh)) = c84e1f8e3a7e322d497cd16c0e8a13c7e127baf3
Provides: golang(bundled(golang.org/x/exp/inotify)) = d00e13ec443927751b2bd49e97dea7bf3b6a6487
Provides: golang(bundled(golang.org/x/net/context)) = c2528b2dd8352441850638a8bb678c2ad056fd3e
Provides: golang(bundled(golang.org/x/net/html)) = c2528b2dd8352441850638a8bb678c2ad056fd3e
Provides: golang(bundled(golang.org/x/net/internal/timeseries)) = ea47fc708ee3e20177f3ca3716217c4ab75942cb
Provides: golang(bundled(golang.org/x/net/trace)) = ea47fc708ee3e20177f3ca3716217c4ab75942cb
Provides: golang(bundled(golang.org/x/net/websocket)) = c2528b2dd8352441850638a8bb678c2ad056fd3e
Provides: golang(bundled(golang.org/x/oauth2)) = b5adcc2dcdf009d0391547edc6ecbaff889f5bb9
Provides: golang(bundled(google.golang.org/api/cloudmonitoring/v2beta2)) = 0c2979aeaa5b573e60d3ddffe5ce8dca8df309bd
Provides: golang(bundled(google.golang.org/api/compute/v1)) = 0c2979aeaa5b573e60d3ddffe5ce8dca8df309bd
Provides: golang(bundled(google.golang.org/api/container/v1beta1)) = 0c2979aeaa5b573e60d3ddffe5ce8dca8df309bd
Provides: golang(bundled(google.golang.org/api/googleapi)) = 0c2979aeaa5b573e60d3ddffe5ce8dca8df309bd
Provides: golang(bundled(google.golang.org/cloud/compute/metadata)) = 2e43671e4ad874a7bca65746ff3edb38e6e93762
Provides: golang(bundled(google.golang.org/cloud/internal)) = 2e43671e4ad874a7bca65746ff3edb38e6e93762
Provides: golang(bundled(google.golang.org/grpc)) = 868330046b32ec2d0e37a3d8d8cdacff14f32555
Provides: golang(bundled(gopkg.in/asn1-ber.v1)) = 9eae18c3681ae3d3c677ac2b80a8fe57de45fc09
Provides: golang(bundled(gopkg.in/yaml.v2)) = d466437aa4adc35830964cffc5b5f262c63ddcb4
Provides: golang(bundled(k8s.io/heapster/api/v1/types)) = 0e1b652781812dee2c51c75180fc590223e0b9c6
Provides: golang(bundled(k8s.io/kubernetes/cmd/kube-apiserver/app)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/cmd/kube-controller-manager/app)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/cmd/kube-proxy/app)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/cmd/kubelet/app)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/admission)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/api)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/apis/extensions)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/apiserver)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/auth/authenticator)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/auth/authorizer)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/auth/handlers)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/auth/user)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/capabilities)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/client/cache)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/client/chaosclient)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/client/metrics)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/client/record)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/client/unversioned)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/cloudprovider)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/controller)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/conversion)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/credentialprovider)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/fieldpath)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/fields)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/healthz)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/httplog)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/kubectl)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/kubelet)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/labels)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/master)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/probe)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/proxy)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/componentstatus)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/controller)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/daemonset)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/deployment)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/endpoint)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/event)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/experimental/controller/etcd)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/generic)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/horizontalpodautoscaler)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/ingress)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/job)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/limitrange)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/namespace)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/node)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/persistentvolume)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/persistentvolumeclaim)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/pod)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/podtemplate)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/registrytest)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/resourcequota)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/secret)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/service)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/serviceaccount)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/thirdpartyresource)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/registry/thirdpartyresourcedata)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/runtime)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/securitycontext)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/securitycontextconstraints)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/securitycontextconstraints)) = 86b4e777e1947c1bc00e422306a3ca74cbd54dbe
Provides: golang(bundled(k8s.io/kubernetes/pkg/storage)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/tools)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/types)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/ui)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/util)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/version)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/volume)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/pkg/watch)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/cmd/kube-scheduler/app)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/admit)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/deny)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/exec)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/initialresources)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/limitranger)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/namespace/autoprovision)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/namespace/exists)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/resourcequota)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/securitycontext/scdeny)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/admission/serviceaccount)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/password/passwordfile)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/request/basicauth)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/request/keystone)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/request/union)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/request/x509)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/token/oidc)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/auth/authenticator/token/tokenfile)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/plugin/pkg/scheduler)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/test/e2e)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/forked/coreos/go-etcd/etcd)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/forked/json)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/forked/reflect)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/golang/expansion)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/golang/netutil)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(k8s.io/kubernetes/third_party/golang/template)) = 4c8e6f47ec23f390978e651232b375f5f9cde3c7
Provides: golang(bundled(speter.net/go/exp/math/dec/inf)) = 42ca6cd68aa922bc3f32f1e056e61b65945d9ad7

%description
%{summary}

%package master
Summary:        %{product_name} Master
Requires:       %{name} = %{version}-%{release}
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd
Obsoletes:      openshift-master < %{package_refector_version}

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
Requires:       ethtool
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd
Obsoletes:      openshift-node < %{package_refector_version}

%description node
%{summary}

%package -n tuned-profiles-%{name}-node
Summary:        Tuned profiles for %{product_name} Node hosts
Requires:       tuned >= %{tuned_version}
Obsoletes:      tuned-profiles-openshift-node < %{package_refector_version}

%description -n tuned-profiles-%{name}-node
%{summary}

%package clients
Summary:        %{product_name} Client binaries for Linux
Obsoletes:      openshift-clients < %{package_refector_version}

%description clients
%{summary}

%if 0%{?make_redistributable}
%package clients-redistributable
Summary:        %{product_name} Client binaries for Linux, Mac OSX, and Windows
BuildRequires:  golang-pkg-darwin-amd64
BuildRequires:  golang-pkg-windows-386
Obsoletes:      openshift-clients-redistributable < %{package_refector_version}

%description clients-redistributable
%{summary}
%endif

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

%package recycle
Summary:        %{product_name} Recycler
Requires:       %{name} = %{version}-%{release}

%description recycle
%{summary}

%package sdn-ovs
Summary:          %{product_name} SDN Plugin for Open vSwitch
Requires:         openvswitch >= %{openvswitch_version}
Requires:         %{name}-node = %{version}-%{release}
Requires:         bridge-utils
Requires:         ethtool
Obsoletes:        openshift-sdn-ovs < %{package_refector_version}

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
for cmd in oc openshift dockerregistry recycle
do
        go install -ldflags "%{ldflags}" %{import_path}/cmd/${cmd}
done

%if 0%{?make_redistributable}
# Build clients for other platforms
GOOS=windows GOARCH=386 go install -ldflags "%{ldflags}" %{import_path}/cmd/oc
GOOS=darwin GOARCH=amd64 go install -ldflags "%{ldflags}" %{import_path}/cmd/oc
%endif

#Build our pod
pushd images/pod/
    go build -ldflags "%{ldflags}" pod.go
popd

%install

install -d %{buildroot}%{_bindir}

# Install linux components
for bin in oc openshift dockerregistry recycle
do
  echo "+++ INSTALLING ${bin}"
  install -p -m 755 _build/bin/${bin} %{buildroot}%{_bindir}/${bin}
done

%if 0%{?make_redistributable}
# Install client executable for windows and mac
install -d %{buildroot}%{_datadir}/%{name}/{linux,macosx,windows}
install -p -m 755 _build/bin/oc %{buildroot}%{_datadir}/%{name}/linux/oc
install -p -m 755 _build/bin/darwin_amd64/oc %{buildroot}/%{_datadir}/%{name}/macosx/oc
install -p -m 755 _build/bin/windows_386/oc.exe %{buildroot}/%{_datadir}/%{name}/windows/oc.exe
%endif

#Install pod
install -p -m 755 images/pod/pod %{buildroot}%{_bindir}/

install -d -m 0755 %{buildroot}%{_unitdir}

mkdir -p %{buildroot}%{_sysconfdir}/sysconfig

for cmd in \
    openshift-router \
    openshift-deploy \
    openshift-sti-build \
    openshift-docker-build \
    origin \
    atomic-enterprise \
    oadm \
    kubernetes \
    kubelet \
    kube-proxy \
    kube-apiserver \
    kube-controller-manager \
    kube-scheduler
do
    ln -s %{_bindir}/openshift %{buildroot}%{_bindir}/$cmd
done

ln -s oc %{buildroot}%{_bindir}/kubectl

install -d -m 0755 %{buildroot}%{_sysconfdir}/origin/{master,node}

# different service for origin vs aos
install -m 0644 contrib/systemd/%{name}-master.service %{buildroot}%{_unitdir}/%{name}-master.service
install -m 0644 contrib/systemd/%{name}-node.service %{buildroot}%{_unitdir}/%{name}-node.service
# same sysconfig files for origin vs aos
install -m 0644 contrib/systemd/origin-master.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-master
install -m 0644 contrib/systemd/origin-node.sysconfig %{buildroot}%{_sysconfdir}/sysconfig/%{name}-node
install -d -m 0755 %{buildroot}%{_prefix}/lib/tuned/%{name}-node-{guest,host}
install -m 0644 contrib/tuned/origin-node-guest/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-guest/tuned.conf
install -m 0644 contrib/tuned/origin-node-host/tuned.conf %{buildroot}%{_prefix}/lib/tuned/%{name}-node-host/tuned.conf
install -d -m 0755 %{buildroot}%{_mandir}/man7
install -m 0644 contrib/tuned/man/tuned-profiles-origin-node.7 %{buildroot}%{_mandir}/man7/tuned-profiles-%{name}-node.7

mkdir -p %{buildroot}%{_sharedstatedir}/origin


# Install sdn scripts
install -d -m 0755 %{buildroot}%{_unitdir}/docker.service.d
install -p -m 0644 contrib/systemd/docker-sdn-ovs.conf %{buildroot}%{_unitdir}/docker.service.d/
pushd _thirdpartyhacks/src/%{sdn_import_path}/plugins/osdn/flatsdn/bin
   install -p -m 755 openshift-ovs-subnet %{buildroot}%{_bindir}/openshift-ovs-subnet
   install -p -m 755 openshift-sdn-kube-subnet-setup.sh %{buildroot}%{_bindir}/openshift-sdn-kube-subnet-setup.sh
popd
pushd _thirdpartyhacks/src/%{sdn_import_path}/plugins/osdn/multitenant/bin
   install -p -m 755 openshift-ovs-multitenant %{buildroot}%{_bindir}/openshift-ovs-multitenant
   install -p -m 755 openshift-sdn-multitenant-setup.sh %{buildroot}%{_bindir}/openshift-sdn-multitenant-setup.sh
popd
install -d -m 0755 %{buildroot}%{_unitdir}/%{name}-node.service.d
install -p -m 0644 contrib/systemd/openshift-sdn-ovs.conf %{buildroot}%{_unitdir}/%{name}-node.service.d/openshift-sdn-ovs.conf

# Install bash completions
install -d -m 755 %{buildroot}%{_sysconfdir}/bash_completion.d/
install -p -m 644 contrib/completions/bash/* %{buildroot}%{_sysconfdir}/bash_completion.d/
# Generate atomic-enterprise bash completions
%{__sed} -e "s|openshift|atomic-enterprise|g" contrib/completions/bash/openshift > %{buildroot}%{_sysconfdir}/bash_completion.d/atomic-enterprise

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/openshift
%{_bindir}/openshift-router
%{_bindir}/openshift-deploy
%{_bindir}/openshift-sti-build
%{_bindir}/openshift-docker-build
%{_bindir}/origin
%{_bindir}/atomic-enterprise
%{_bindir}/oadm
%{_bindir}/kubernetes
%{_bindir}/kubelet
%{_bindir}/kube-proxy
%{_bindir}/kube-apiserver
%{_bindir}/kube-controller-manager
%{_bindir}/kube-scheduler
%{_sharedstatedir}/origin
%{_sysconfdir}/bash_completion.d/atomic-enterprise
%{_sysconfdir}/bash_completion.d/oadm
%{_sysconfdir}/bash_completion.d/openshift
%dir %config(noreplace) %{_sysconfdir}/origin

%pre
# If /etc/openshift exists and /etc/origin doesn't, symlink it to /etc/origin
if [ -d "%{_sysconfdir}/openshift" ]; then
  if ! [ -d "%{_sysconfdir}/origin"  ]; then
    ln -s %{_sysconfdir}/openshift %{_sysconfdir}/origin
  fi
fi
if [ -d "%{_sharedstatedir}/openshift" ]; then
  if ! [ -d "%{_sharedstatedir}/origin"  ]; then
    ln -s %{_sharedstatedir}/openshift %{_sharedstatedir}/origin
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
  %{_bindir}/openshift start master --write-config=%{_sysconfdir}/origin/master
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
%{_bindir}/openshift-ovs-subnet
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
%{_bindir}/oc
%{_bindir}/kubectl
%{_sysconfdir}/bash_completion.d/oc

%if 0%{?make_redistributable}
%files clients-redistributable
%{_datadir}/%{name}/linux/oc
%{_datadir}/%{name}/macosx/oc
%{_datadir}/%{name}/windows/oc.exe
%endif

%files dockerregistry
%defattr(-,root,root,-)
%{_bindir}/dockerregistry

%files pod
%defattr(-,root,root,-)
%{_bindir}/pod

%files recycle
%defattr(-,root,root,-)
%{_bindir}/recycle


%changelog
* Fri Sep 18 2015 Scott Dodson <sdodson@redhat.com> 0.2-9
- Rename from openshift -> origin
- Symlink /var/lib/origin to /var/lib/openshift if /var/lib/openshift exists

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
