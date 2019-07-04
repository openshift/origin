module k8s.io/apiserver

go 1.12

require (
	bitbucket.org/bertimus9/systemstat v0.0.0-20180207000608-0eeff89b0690
	bitbucket.org/ww/goautoneg v0.0.0-20190703120000-75cd24fc2f2c2a2088577d12123ddee5f54e0675
	cloud.google.com/go v0.41.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/AaronO/go-git-http v0.0.0-20161214145340-1d9485b3a98f
	github.com/Azure/azure-sdk-for-go v31.0.0+incompatible
	github.com/Azure/go-ansiterm v0.0.0-20190703120000-d6e3b3328b783f23731bc4d058875b0371ff8109
	github.com/Azure/go-autorest v0.0.0-20190703120000-ea233b6412b0421a65dc6160e16c893364664a95
	github.com/Azure/go-autorest/autorest v0.3.0
	github.com/Azure/go-autorest/autorest/adal v0.1.0
	github.com/Azure/go-autorest/autorest/date v0.1.0
	github.com/Azure/go-autorest/autorest/mocks v0.1.0
	github.com/Azure/go-autorest/autorest/to v0.2.0
	github.com/Azure/go-autorest/autorest/validation v0.1.0
	github.com/Azure/go-autorest/logger v0.1.0
	github.com/Azure/go-autorest/tracing v0.1.0
	github.com/BurntSushi/toml v0.3.1
	github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802
	github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20190703120000-f8e99590510076aa1e3ff07df946f05220c50fb4
	github.com/JeffAshton/win_pdh v0.0.0-20190703120000-76bb4ee9f0ab50f77826f2a2ee7fb9d3880d6ec2
	github.com/MakeNowJust/heredoc v0.0.0-20180919145318-e9091a26100e9cfb2b6a8f470085bfa541931a91
	github.com/Microsoft/go-winio v0.4.12
	github.com/Microsoft/hcsshim v0.8.6
	github.com/NYTimes/gziphandler v1.1.1
	github.com/Nvveen/Gotty v0.0.0-20190703120000-cd527374f1e5bff4938207604a14f2e38a9cf512
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20190703120000-5bd2802263f21d8788851d5305584c82a5c75d7e
	github.com/RangelReale/osin v0.0.0-20190703120000-2dc1b43167692cdc89446b99b98fa9de6bff020f
	github.com/RangelReale/osincli v0.0.0-20190703120000-fababb0555f21315d1a34af6615a16eaab44396b
	github.com/Rican7/retry v0.1.0
	github.com/Sirupsen/logrus v1.4.2
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf
	github.com/alexbrainman/sspi v0.0.0-20190703120000-e580b900e9f5657daa5473021296289be6da2661
	github.com/apcera/gssapi v0.0.0-20190703120000-5fb4217df13b8e6878046fe1e5c10e560e1b86dc
	github.com/armon/circbuf v0.0.0-20190703120000-bbbad097214e2918d8543d5201d12bfd7bca254d
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6
	github.com/asaskevich/govalidator v0.0.0-20190703120000-f9ffefc3facfbe0caee3fea233cbb6e8208f4541
	github.com/auth0/go-jwt-middleware v0.0.0-20170425171159-5493cabe49f7
	github.com/aws/aws-sdk-go v1.20.14
	github.com/bcicen/go-haproxy v0.0.0-20190703120000-ff5824fe38bede761b873cab6e247a530e89236a
	github.com/beorn7/perks v1.0.0
	github.com/bifurcation/mint v0.0.0-20180715133206-93c51c6ce115
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/census-instrumentation/opencensus-proto v0.2.0
	github.com/certifi/gocertifi v0.0.0-20190703120000-ee1a9a0726d2ae45f54118cac878c990d4016ded
	github.com/chai2010/gettext-go v0.0.0-20190703120000-c6fed771bfd517099caf0f7a961671fa8ed08723
	github.com/cheekybits/genny v0.0.0-20170328200008-9127e812e1e9
	github.com/client9/misspell v0.3.4
	github.com/cloudflare/cfssl v0.0.0-20190703120000-56268a613adfed278936377c18b1152d2c4ad5da
	github.com/clusterhq/flocker-go v0.0.0-20190703120000-2b8b7259d3139c96c4a6871031355808ab3fd3b3
	github.com/cockroachdb/cmux v0.0.0-20190703120000-b64f5908f4945f4b11ed4a0a9d3cc1e23350866d
	github.com/codedellemc/goscaleio v0.1.0
	github.com/codegangsta/negroni v1.0.0
	github.com/container-storage-interface/spec v1.1.0
	github.com/containerd/console v0.0.0-20190703120000-84eeaae905fa414d03e07bcd6c8d3f19e7cf180e
	github.com/containerd/containerd v1.2.7
	github.com/containerd/continuity v0.0.0-20190703120000-aaeac12a7ffcd198ae25440a9dff125c2e2703a7
	github.com/containerd/typeurl v0.0.0-20190515163108-7312978f2987
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.0.0-20190703120000-7480240de9749f9a0a5c8614b17f1f03e0c06ab9
	github.com/containers/image v0.0.0-20190703120000-4bc6d24282b115f8b61a6d08470ed42ac7c91392
	github.com/containers/storage v0.0.0-20190703120000-47536c89fcc545a87745e1a1573addc439409165
	github.com/coreos/bbolt v1.3.2
	github.com/coreos/etcd v3.3.10+incompatible
	github.com/coreos/go-iptables v0.0.0-20190703120000-259c8e6a4275d497442c721fa52204d7a58bde8b
	github.com/coreos/go-oidc v2.0.0+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20190703120000-39ca1b05acc7ad1220e09f133283b8859a8b71ab
	github.com/coreos/pkg v0.0.0-20180928190104-97fdf19511ea361ae1c100dd393cc47f8dcfa1e1
	github.com/coreos/rkt v1.30.0
	github.com/cpuguy83/go-md2man v1.0.10
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/d2g/dhcp4 v0.0.0-20190703120000-a1d1b6c41b1ce8a71a5121a9cee31809c4707d9c
	github.com/d2g/dhcp4client v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/daviddengcn/go-colortext v0.0.0-20190703120000-511bcaf42ccd42c38aba7427b6673277bf19e2a1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dnaeon/go-vcr v1.0.1
	github.com/docker/distribution v0.0.0-20190703120000-d4c35485a70df4dce2179bc227b1393a69edb809
	github.com/docker/docker v0.0.0-20190703120000-a9fbbdc8dd8794b20af358382ab780559bca589d
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-metrics v0.0.0-20190703120000-b84716841b82eab644a0c64fc8b42d480e49add5
	github.com/docker/go-units v0.4.0
	github.com/docker/libnetwork v0.5.6
	github.com/docker/libtrust v0.0.0-20190703120000-aabc10ec26b754e797f9028f4589c5b7bd90dc20
	github.com/docker/spdystream v0.0.0-20190703120000-449fdfce4d962303d702fec724ef0ad181c92528
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy v0.0.0-20190703090003-c4fc26588b6ef8af07a191fcb6476387bdd46711
	github.com/elazarl/goproxy/ext v0.0.0-20190703090003-6125c262ffb0
	github.com/emicklei/go-restful v2.9.6+incompatible
	github.com/euank/go-kmsg-parser v2.0.0+incompatible
	github.com/evanphx/json-patch v0.0.0-20190703120000-5858425f75500d40c52783dce87d085a483ce135
	github.com/exponent-io/jsonpath v0.0.0-20190703120000-d6023ce2651d8eafb5c75bb0c7167536102ec9f5
	github.com/fatih/camelcase v1.0.0
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v0.0.0-20190703120000-da3951ba2e9e02bc0e7642150b3e265aed7e1df3
	github.com/getsentry/raven-go v0.0.0-20190703120000-32a13797442ccb601b11761d74232773c1402d14
	github.com/ghodss/yaml v1.0.0
	github.com/globalsign/mgo v0.0.0-20190703120000-eeefdecb41b842af6dc652aaea4026e8403e62df
	github.com/go-acme/lego v2.5.0+incompatible
	github.com/go-kit/kit v0.8.0
	github.com/go-logfmt/logfmt v0.4.0
	github.com/go-openapi/analysis v0.19.3
	github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/jsonpointer v0.19.2
	github.com/go-openapi/jsonreference v0.19.2
	github.com/go-openapi/loads v0.19.2
	github.com/go-openapi/runtime v0.19.0
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/strfmt v0.19.0
	github.com/go-openapi/swag v0.19.3
	github.com/go-openapi/validate v0.19.2
	github.com/go-ozzo/ozzo-validation v3.5.0+incompatible
	github.com/go-stack/stack v1.8.0
	github.com/gocarina/gocsv v0.0.0-20190703120000-a5c9099e2484f1551abb9433885e158610a25f4b
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/gogo/protobuf v1.2.1
	github.com/golang/glog v0.0.0-20190703120000-3c92600d7533018d216b534fe894ad60a1e6d5bf
	github.com/golang/groupcache v0.0.0-20190703120000-02826c3e79038b59d737d3b1c0a1d937f71a4433
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/golangplus/bytes v0.0.0-20160111154220-45c989fe5450
	github.com/golangplus/fmt v0.0.0-20150411045040-2a5d6d7d2995
	github.com/golangplus/testing v0.0.0-20180327235837-af21d9c3145e
	github.com/gonum/blas v0.0.0-20190703120000-37e82626499e1df7c54aeaba0959fd6e7e8dc1e4
	github.com/gonum/diff v0.0.0-20181124234638-500114f11e71
	github.com/gonum/floats v0.0.0-20190703120000-f74b330d45c56584a6ea7a27f5c64ea2900631e9
	github.com/gonum/graph v0.0.0-20190703120000-50b27dea7ebbfb052dfaf91681afc6fde28d8796
	github.com/gonum/integrate v0.0.0-20181209220457-a422b5c0fdf2
	github.com/gonum/internal v0.0.0-20190703120000-e57e4534cf9b3b00ef6c0175f59d8d2d34f60914
	github.com/gonum/lapack v0.0.0-20190703120000-5ed4b826becd1807e09377508f51756586d1a98c
	github.com/gonum/mathext v0.0.0-20181121095525-8a4bf007ea55
	github.com/gonum/matrix v0.0.0-20190703120000-dd6034299e4242c9f0ea36735e6d4264dfcb3f9f
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/google/btree v1.0.0
	github.com/google/cadvisor v0.32.0
	github.com/google/certificate-transparency-go v1.0.21
	github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz v1.0.0
	github.com/google/martian v2.1.0+incompatible
	github.com/google/pprof v0.0.0-20190515194954-54271f7e092f
	github.com/google/uuid v1.1.1
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/googleapis/gnostic v0.3.0
	github.com/gophercloud/gophercloud v0.2.0
	github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v0.0.0-20190703120000-a3acf13e802c358d65f249324d14ed24aac11370
	github.com/gorilla/websocket v1.4.0
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible
	github.com/gregjones/httpcache v0.0.0-20190703120000-787624de3eb7bd915c329cba748687a3b22666a6
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.9.0
	github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/hashicorp/hcl v1.0.0
	github.com/heketi/heketi v9.0.0+incompatible
	github.com/heketi/rest v0.0.0-20180404230133-aa6a65207413
	github.com/heketi/tests v0.0.0-20151005000721-f3775cbcefd6
	github.com/heketi/utils v0.0.0-20170317161834-435bc5bdfa64
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.7
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jimstudt/http-authentication v0.0.0-20140401203705-3eca13d6893a
	github.com/jmespath/go-jmespath v0.0.0-20190703120000-0b12d6b521d83fc7f755e7cfc1b1fbdd35a01a74
	github.com/joho/godotenv v0.0.0-20190703120000-6d367c18edf6ca7fd004efd6863e4c5728fa858e
	github.com/jonboulle/clockwork v0.1.0
	github.com/json-iterator/go v1.1.6
	github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/jteeuwen/go-bindata v0.0.0-20190703120000-a0ff2567cfb70903282db057e799fd826784d41d
	github.com/jtolds/gls v4.20.0+incompatible
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext v0.0.0-20190703120000-8fef92e41e22a70e700a96b29f066cda30ea24ef
	github.com/karrick/godirwalk v1.10.12
	github.com/kisielk/errcheck v1.1.0
	github.com/kisielk/gotool v1.0.0
	github.com/klauspost/cpuid v1.2.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/kr/fs v0.0.0-20190703120000-2788f0dbd16903de03cb8186e5c7d97b69ad387b
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/kr/pretty v0.1.0
	github.com/kr/pty v1.1.5
	github.com/kr/text v0.1.0
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/lestrrat-go/jspointer v0.0.0-20181205001929-82fadba7561c
	github.com/lestrrat-go/jsref v0.0.0-20181205001954-1b590508f37d
	github.com/lestrrat-go/jsschema v0.0.0-20181205002244-5c81c58ffcc3
	github.com/lestrrat-go/jsval v0.0.0-20181205002323-20277e9befc0
	github.com/lestrrat-go/pdebug v0.0.0-20180220043849-39f9a71bcabe
	github.com/lestrrat-go/structinfo v0.0.0-20190212233437-acd51874663b
	github.com/lestrrat/go-jspointer v0.0.0-20190703120000-f4881e611bdbe9fb413a7780721ef8400a1f2341
	github.com/lestrrat/go-jsref v0.0.0-20190703120000-50df7b2d07d799426a9ac43fa24bdb4785f72a54
	github.com/lestrrat/go-jsschema v0.0.0-20190703120000-a6a42341b50d8d7e2a733db922eefaa756321021
	github.com/lestrrat/go-jsval v0.0.0-20181205002323-20277e9befc0
	github.com/lestrrat/go-pdebug v0.0.0-20190703120000-569c97477ae8837e053e5a50bc739e15172b8ebe
	github.com/lestrrat/go-structinfo v0.0.0-20190703120000-8204d40bbcd79eb7603cd4c2c998e60eb2479ded
	github.com/libopenstorage/openstorage v8.0.0+incompatible
	github.com/liggitt/tabwriter v0.0.0-20190703120000-89fcab3d43de07060e4fd4c1547430ed57e87f24
	github.com/lithammer/dedent v1.1.0
	github.com/lpabon/godbc v0.1.1
	github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f
	github.com/lucas-clemente/quic-clients v0.1.0
	github.com/lucas-clemente/quic-go v0.10.2
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced
	github.com/magiconair/properties v1.8.0
	github.com/mailru/easyjson v0.0.0-20190703120000-2f5df55504ebc322e4d52d34df6a1f5b503bf26d
	github.com/marstr/guid v0.0.0-20190703120000-8bdf7d1a087ccc975cf37dd6507da50698fd19ca
	github.com/marten-seemann/qtls v0.2.3
	github.com/mattn/go-shellwords v1.0.5
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/mesos/mesos-go v0.0.9
	github.com/mholt/caddy v1.0.1
	github.com/mholt/certmagic v0.6.2-0.20190624175158-6a42ef9fe8c2
	github.com/miekg/dns v1.1.3
	github.com/mindprince/gonvml v0.0.0-20190703120000-fee913ce8fb235edf54739d259ca0ecc226c7b8a
	github.com/mistifyio/go-zfs v2.1.1+incompatible
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/moby/buildkit v0.0.0-20190703120000-c3a857e3fca0a5cadd44ffd886a977559841aeaa
	github.com/modern-go/concurrent v0.0.0-20190703120000-bacd9c7ef1dd9b15be4a9909b8ac7a4e313eec94
	github.com/modern-go/reflect2 v1.0.1
	github.com/mohae/deepcopy v0.0.0-20190703120000-491d3605edfb866af34a48075bd4355ac1bf46ca
	github.com/mrunalp/fileutils v0.0.0-20190703120000-4ee1cc9a80582a0c75febdd5cfa779ee4361cbca
	github.com/mtrmac/gpgme v0.0.0-20190703120000-b2432428689ca58c2b8e8dea9449d3295cf96fc9
	github.com/munnerz/goautoneg v0.0.0-20190703120000-a547fc61f48d567d5b4ec6f8aee5573d8efce11d
	github.com/mvdan/xurls v1.1.0
	github.com/mwitkow/go-conntrack v0.0.0-20161129095857-cc309e4a2223
	github.com/mxk/go-flowrate v0.0.0-20190703120000-cca7078d478f8520f85629ad7c68962d31ed7682
	github.com/naoina/go-stringutil v0.1.0
	github.com/naoina/toml v0.1.1
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.0.0-20190703120000-7c7775178c25e952571573f44a8df281824cf8e1
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opencontainers/selinux v1.2.2
	github.com/openshift/api v0.0.0-20190703120000-64d243ed05c5e2e1c779db0810323571d5674ccc
	github.com/openshift/client-go v0.0.0-20190703120000-c44a8b61b9f46cd9e802384dfeda0bc9942db68a
	github.com/openshift/library-go v0.0.0-20190702153934-f8abdcd57c
	github.com/openshift/source-to-image v1.1.14
	github.com/pborman/uuid v1.2.0
	github.com/pelletier/go-toml v1.4.0
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.8.1
	github.com/pkg/profile v1.3.0
	github.com/pkg/sftp v0.0.0-20190703120000-4d0e916071f68db74f8a73926335f809396d6b42
	github.com/pmezard/go-difflib v1.0.0
	github.com/pquerna/cachecontrol v0.0.0-20190703120000-0dec1b30a0215bb68605dfc568e8855066c9202d
	github.com/pquerna/ffjson v0.0.0-20190703120000-af8b230fcd2007c7095168ca8ab94c68b60840c6
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.2
	github.com/quobyte/api v0.1.4
	github.com/rancher/go-rancher v0.1.0
	github.com/remyoudompheng/bigfft v0.0.0-20170806203942-52369c62f446
	github.com/robfig/cron v1.2.0
	github.com/rogpeppe/fastuuid v0.0.0-20150106093220-6724a57986af
	github.com/rogpeppe/go-charset v0.0.0-20180617210344-2471d30d28b4
	github.com/rubiojr/go-vhd v0.0.0-20190703120000-0bfd3b39853cdde5762efda92289f14b0ac0491b
	github.com/russross/blackfriday v2.0.0+incompatible
	github.com/satori/go.uuid v1.2.0
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/shurcooL/sanitized_anchor_name v1.0.0
	github.com/sigma/go-inotify v0.0.0-20190703120000-c87b6cf5033d2c6486046f045eeebdc3d910fd38
	github.com/sirupsen/logrus v1.2.0
	github.com/smartystreets/assertions v0.0.0-20180927180507-b2de0cb4f26d
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/afero v1.2.2
	github.com/spf13/cast v1.3.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.0.0
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/storageos/go-api v0.0.0-20190703120000-343b3eff91fcc84b0165e252eb843f5fd720fa4e
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability v0.0.0-20190703120000-e7cb7fa329f456b3855136a2642b197bad7366ba
	github.com/thecodeteam/goscaleio v0.1.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190703120000-89b8d40f7ca833297db804fcb3be53a76d01c238
	github.com/ugorji/go v1.1.7
	github.com/ugorji/go/codec v1.1.7
	github.com/urfave/negroni v1.0.0
	github.com/vishvananda/netlink v0.0.0-20190703120000-b2de5d10e38ecce8607e6b438b6d174f389a004e
	github.com/vishvananda/netns v0.0.0-20190703120000-be1fbeda19366dea804f00efff2dd73a1642fdcc
	github.com/vjeantet/asn1-ber v0.0.0-20190703120000-85041cd0f4769ebf4a5ae600b1e921e630d6aff0
	github.com/vjeantet/ldapserver v0.0.0-20190703120000-5ac58729571e52ae23768e3c270c624d4ee7fa23
	github.com/vmware/govmomi v0.20.1
	github.com/vmware/photon-controller-go-sdk v0.0.0-20190703120000-4a435daef6ccd3d0edaac1161e76f51a70c2589a
	github.com/xanzy/go-cloudstack v2.4.1+incompatible
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415
	github.com/xeipuuv/gojsonschema v1.1.0
	github.com/xiang90/probing v0.0.0-20190703120000-07dd2e8dfe18522e9c447ba95f2fe95262f63bb2
	github.com/xlab/handysort v0.0.0-20150421192137-fb3537ed64a1
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77
	go.etcd.io/bbolt v1.3.2
	go.opencensus.io v0.22.0
	go.uber.org/atomic v1.4.0
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.10.0
	go4.org v0.0.0-20190703120000-03efcb870d84809319ea509714dd6d19a1498483
	golang.org/x/crypto v0.0.0-20190701094942-de0752318171da717af4ce24d0a2e8626afaeb11
	golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522
	golang.org/x/image v0.0.0-20190227222117-0694c2d4d067
	golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/mobile v0.0.0-20190312151609-d3739f865fa6
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/oauth2 v0.0.0-20190604053449-a6bd8cefa1811bd24b86f8902872e4e8225f74c4kb
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190703120000-95c6576299259db960f6c5b9b69ea52422860fce
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20190703120000-9d24e82272b4f38b78bc8cff74fa936d31ccd8ef
	golang.org/x/tools v0.0.0-20190703120000-7f7074d5bcfd282eb16bc382b0bb3da762461985
	gonum.org/v1/gonum v0.0.0-20190703120000-cebdade430ccb61c1feba4878085f6cf8cb3320e
	gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e
	google.golang.org/api v0.7.0
	google.golang.org/appengine v1.6.1
	google.golang.org/genproto v0.0.0-20190703120000-09f6ed296fc66555a25fe4ce95173148778dfa85
	google.golang.org/grpc v1.21.1
	gopkg.in/airbrake/gobrake.v2 v2.0.9
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/mcuadros/go-syslog.v2 v2.2.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/resty.v1 v1.12.0
	gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/warnings.v0 v0.1.2
	gopkg.in/yaml.v2 v2.2.2
	gotest.tools v2.2.0+incompatible
	honnef.co/go/tools v0.0.0-20190418001031-e561f6794a2a
	k8s.io/gengo v0.0.0-20190703120000-51747d6e00da1fc578d5a333a93bb2abcbce7a95
	k8s.io/heapster v1.5.4
	k8s.io/klog v0.0.0-20190703120000-8139d8cb77af419532b33dfa7dd09fbc5f1d344f
	k8s.io/kube-openapi v0.0.0-20190703120000-b3a7cee44a305be0a69e1b9ac03018307287e1b0
	k8s.io/kubernetes v0.0.0-20190703120000-4aa0d2902ed2c342734fb94f6a124c6b558d8fae
	k8s.io/utils v0.0.0-20190703120000-c2654d5206da6b7b6ace12841e8f359bb89b443c
	modernc.org/cc v1.0.0
	modernc.org/golex v1.0.0
	modernc.org/mathutil v1.0.0
	modernc.org/strutil v1.0.0
	modernc.org/xc v1.0.0
	rsc.io/binaryregexp v0.2.0
	sigs.k8s.io/kustomize v2.0.3+incompatible
	sigs.k8s.io/structured-merge-diff v0.0.0-20190703120000-e85c7b244fd2cc57bb829d73a061f93a441e63ce
	sigs.k8s.io/yaml v1.1.0
	vbom.ml/util v0.0.0-20190703120000-db5cfe13f5cc80a4990d98e2e1b0707a4d1a5394
)

replace (
	bitbucket.org/bertimus9/systemstat => bitbucket.org/bertimus9/systemstat v0.0.0-20180207000608-0eeff89b0690
	bitbucket.org/ww/goautoneg => bitbucket.org/ww/goautoneg v0.0.0-20180919145318-75cd24fc2f
	cloud.google.com/go => cloud.google.com/go v0.0.0-20180919145318-3b1ae45394
	contrib.go.opencensus.io/exporter/ocagent => contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/AaronO/go-git-http => github.com/AaronO/go-git-http v0.0.0-20161214145340-1d9485b3a98f
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v0.0.0-20180919145318-da91af5481
	github.com/Azure/go-ansiterm => github.com/Azure/go-ansiterm v0.0.0-20180919145318-d6e3b3328b
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v0.0.0-20180919145318-ea233b6412
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.3.0
	github.com/Azure/go-autorest/autorest/adal => github.com/Azure/go-autorest/autorest/adal v0.1.0
	github.com/Azure/go-autorest/autorest/date => github.com/Azure/go-autorest/autorest/date v0.1.0
	github.com/Azure/go-autorest/autorest/mocks => github.com/Azure/go-autorest/autorest/mocks v0.1.0
	github.com/Azure/go-autorest/autorest/to => github.com/Azure/go-autorest/autorest/to v0.2.0
	github.com/Azure/go-autorest/autorest/validation => github.com/Azure/go-autorest/autorest/validation v0.1.0
	github.com/Azure/go-autorest/logger => github.com/Azure/go-autorest/logger v0.1.0
	github.com/Azure/go-autorest/tracing => github.com/Azure/go-autorest/tracing v0.1.0
	github.com/BurntSushi/toml => github.com/BurntSushi/toml v0.3.1
	github.com/BurntSushi/xgb => github.com/BurntSushi/xgb v0.0.0-20160522181843-27f122750802
	github.com/GoogleCloudPlatform/k8s-cloud-provider => github.com/GoogleCloudPlatform/k8s-cloud-provider v0.0.0-20180919145318-f8e9959051
	github.com/JeffAshton/win_pdh => github.com/JeffAshton/win_pdh v0.0.0-20180919145318-76bb4ee9f0
	github.com/MakeNowJust/heredoc => github.com/MakeNowJust/heredoc v0.0.0-20180919145318-e9091a2610
	github.com/Microsoft/go-winio => github.com/Microsoft/go-winio v0.0.0-20180919145318-97e4973ce5
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.0.0-20180919145318-69ac8d3f7f
	github.com/NYTimes/gziphandler => github.com/NYTimes/gziphandler v0.0.0-20180919145318-56545f4a5d
	github.com/Nvveen/Gotty => github.com/Nvveen/Gotty v0.0.0-20180919145318-cd527374f1
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v0.0.0-20180919145318-8a290539e2
	github.com/PuerkitoBio/urlesc => github.com/PuerkitoBio/urlesc v0.0.0-20180919145318-5bd2802263
	github.com/RangelReale/osin => github.com/openshift/osin v0.0.0-20190702153934-2dc1b43167
	github.com/RangelReale/osincli => github.com/openshift/osincli v0.0.0-20190702153934-fababb0555
	github.com/Rican7/retry => github.com/Rican7/retry v0.0.0-20180919145318-272ad122d6
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
	github.com/alecthomas/template => github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc
	github.com/alecthomas/units => github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf
	github.com/alexbrainman/sspi => github.com/alexbrainman/sspi v0.0.0-20190702153934-e580b900e9
	github.com/apcera/gssapi => github.com/apcera/gssapi v0.0.0-20180919145318-5fb4217df1
	github.com/armon/circbuf => github.com/armon/circbuf v0.0.0-20180919145318-bbbad09721
	github.com/armon/consul-api => github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6
	github.com/asaskevich/govalidator => github.com/asaskevich/govalidator v0.0.0-20180919145318-f9ffefc3fa
	github.com/auth0/go-jwt-middleware => github.com/auth0/go-jwt-middleware v0.0.0-20170425171159-5493cabe49f7
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v0.0.0-20180919145318-81f3829f5a
	github.com/bcicen/go-haproxy => github.com/bcicen/go-haproxy v0.0.0-20190702153934-ff5824fe38
	github.com/beorn7/perks => github.com/beorn7/perks v0.0.0-20180919145318-3ac7bf7a47
	github.com/bifurcation/mint => github.com/bifurcation/mint v0.0.0-20180715133206-93c51c6ce115
	github.com/blang/semver => github.com/blang/semver v0.0.0-20180919145318-b38d23b878
	github.com/boltdb/bolt => github.com/boltdb/bolt v1.3.1
	github.com/cenkalti/backoff => github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/census-instrumentation/opencensus-proto => github.com/census-instrumentation/opencensus-proto v0.2.0
	github.com/certifi/gocertifi => github.com/certifi/gocertifi v0.0.0-20180919145318-ee1a9a0726
	github.com/chai2010/gettext-go => github.com/chai2010/gettext-go v0.0.0-20180919145318-c6fed771bf
	github.com/cheekybits/genny => github.com/cheekybits/genny v0.0.0-20170328200008-9127e812e1e9
	github.com/client9/misspell => github.com/client9/misspell v0.3.4
	github.com/cloudflare/cfssl => github.com/cloudflare/cfssl v0.0.0-20180919145318-56268a613a
	github.com/clusterhq/flocker-go => github.com/clusterhq/flocker-go v0.0.0-20180919145318-2b8b7259d3
	github.com/cockroachdb/cmux => github.com/cockroachdb/cmux v0.0.0-20190702153934-b64f5908f4
	github.com/codedellemc/goscaleio => github.com/codedellemc/goscaleio v0.0.0-20180919145318-20e2ce2cf8
	github.com/codegangsta/negroni => github.com/codegangsta/negroni v1.0.0
	github.com/container-storage-interface/spec => github.com/container-storage-interface/spec v0.0.0-20180919145318-f750e6765f
	github.com/containerd/console => github.com/containerd/console v0.0.0-20180919145318-84eeaae905
	github.com/containerd/containerd => github.com/containerd/containerd v0.0.0-20180919145318-cfd04396dc
	github.com/containerd/continuity => github.com/containerd/continuity v0.0.0-20180919145318-aaeac12a7f
	github.com/containerd/typeurl => github.com/containerd/typeurl v0.0.0-20190515163108-7312978f2987
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.0.0-20180919145318-a7885cb6f8
	github.com/containernetworking/plugins => github.com/containernetworking/plugins v0.0.0-20180919145318-7480240de9
	github.com/containers/image => github.com/openshift/containers-image v0.0.0-20180919145318-4bc6d24282
	github.com/containers/storage => github.com/containers/storage v0.0.0-20190702153934-47536c89fc
	github.com/coreos/bbolt => github.com/coreos/bbolt v0.0.0-20180919145318-48ea1b39c2
	github.com/coreos/etcd => github.com/coreos/etcd v0.0.0-20180919145318-27fc7e2296
	github.com/coreos/go-iptables => github.com/coreos/go-iptables v0.0.0-20180919145318-259c8e6a42
	github.com/coreos/go-oidc => github.com/coreos/go-oidc v0.0.0-20180919145318-065b426bd4
	github.com/coreos/go-semver => github.com/coreos/go-semver v0.0.0-20180919145318-e214231b29
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20180919145318-39ca1b05ac
	github.com/coreos/pkg => github.com/coreos/pkg v0.0.0-20180919145318-97fdf19511
	github.com/coreos/rkt => github.com/coreos/rkt v0.0.0-20180919145318-ec37f3cb64
	github.com/cpuguy83/go-md2man => github.com/cpuguy83/go-md2man v1.0.10
	github.com/cyphar/filepath-securejoin => github.com/cyphar/filepath-securejoin v0.0.0-20180919145318-ae69057f22
	github.com/d2g/dhcp4 => github.com/d2g/dhcp4 v0.0.0-20180919145318-a1d1b6c41b
	github.com/d2g/dhcp4client => github.com/d2g/dhcp4client v0.0.0-20180919145318-6e570ed0a2
	github.com/davecgh/go-spew => github.com/davecgh/go-spew v0.0.0-20180919145318-782f4967f2
	github.com/daviddengcn/go-colortext => github.com/daviddengcn/go-colortext v0.0.0-20180919145318-511bcaf42c
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go v0.0.0-20180919145318-01aeca54eb
	github.com/dnaeon/go-vcr => github.com/dnaeon/go-vcr v1.0.1
	github.com/docker/distribution => github.com/openshift/docker-distribution v0.0.0-20180919145318-d4c35485a7
	github.com/docker/docker => github.com/docker/docker v0.0.0-20190702153934-a9fbbdc8dd
	github.com/docker/go-connections => github.com/docker/go-connections v0.0.0-20180919145318-3ede32e203
	github.com/docker/go-metrics => github.com/docker/go-metrics v0.0.0-20180919145318-b84716841b
	github.com/docker/go-units => github.com/docker/go-units v0.0.0-20180919145318-519db1ee28
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.0.0-20180919145318-a9cd636e37
	github.com/docker/libtrust => github.com/docker/libtrust v0.0.0-20180919145318-aabc10ec26
	github.com/docker/spdystream => github.com/docker/spdystream v0.0.0-20180919145318-449fdfce4d
	github.com/dustin/go-humanize => github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy => github.com/elazarl/goproxy v0.0.0-20180919145318-c4fc26588b
	github.com/elazarl/goproxy/ext => github.com/elazarl/goproxy/ext v0.0.0-20190703090003-6125c262ffb0
	github.com/emicklei/go-restful => github.com/emicklei/go-restful v0.0.0-20180919145318-ff4f55a206
	github.com/euank/go-kmsg-parser => github.com/euank/go-kmsg-parser v0.0.0-20180919145318-5ba4d492e4
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v0.0.0-20190702153934-5858425f75
	github.com/exponent-io/jsonpath => github.com/exponent-io/jsonpath v0.0.0-20180919145318-d6023ce265
	github.com/fatih/camelcase => github.com/fatih/camelcase v0.0.0-20180919145318-f6a740d52f
	github.com/flynn/go-shlex => github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568
	github.com/fsnotify/fsnotify => github.com/fsnotify/fsnotify v0.0.0-20180919145318-c2828203cd
	github.com/fsouza/go-dockerclient => github.com/fsouza/go-dockerclient v0.0.0-20190702153934-da3951ba2e
	github.com/getsentry/raven-go => github.com/getsentry/raven-go v0.0.0-20190702153934-32a1379744
	github.com/ghodss/yaml => github.com/ghodss/yaml v0.0.0-20190702153934-73d445a936
	github.com/globalsign/mgo => github.com/globalsign/mgo v0.0.0-20180919145318-eeefdecb41
	github.com/go-acme/lego => github.com/go-acme/lego v2.5.0+incompatible
	github.com/go-kit/kit => github.com/go-kit/kit v0.8.0
	github.com/go-logfmt/logfmt => github.com/go-logfmt/logfmt v0.4.0
	github.com/go-openapi/analysis => github.com/go-openapi/analysis v0.0.0-20180919145318-c701774f4e
	github.com/go-openapi/errors => github.com/go-openapi/errors v0.0.0-20180919145318-d9664f9fab
	github.com/go-openapi/jsonpointer => github.com/go-openapi/jsonpointer v0.0.0-20180919145318-ef5f0afec3
	github.com/go-openapi/jsonreference => github.com/go-openapi/jsonreference v0.0.0-20180919145318-8483a886a9
	github.com/go-openapi/loads => github.com/go-openapi/loads v0.0.0-20190702153934-a80dea3052
	github.com/go-openapi/runtime => github.com/go-openapi/runtime v0.0.0-20180919145318-231d7876b7
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.0.0-20180919145318-5bae59e25b
	github.com/go-openapi/strfmt => github.com/go-openapi/strfmt v0.0.0-20180919145318-35fe473529
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.0.0-20180919145318-5899d5c5e6
	github.com/go-openapi/validate => github.com/go-openapi/validate v0.0.0-20180919145318-d2eab7d930
	github.com/go-ozzo/ozzo-validation => github.com/go-ozzo/ozzo-validation v0.0.0-20180919145318-106681dbb3
	github.com/go-stack/stack => github.com/go-stack/stack v1.8.0
	github.com/gocarina/gocsv => github.com/gocarina/gocsv v0.0.0-20190702153934-a5c9099e24
	github.com/godbus/dbus => github.com/godbus/dbus v0.0.0-20180919145318-c7fdd8b5cd
	github.com/gogo/protobuf => github.com/gogo/protobuf v0.0.0-20180919145318-342cbe0a04
	github.com/golang/glog => github.com/openshift/golang-glog v0.0.0-20180919145318-3c92600d75
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20180919145318-02826c3e79
	github.com/golang/mock => github.com/golang/mock v0.0.0-20180919145318-bd3c8e81be
	github.com/golang/protobuf => github.com/golang/protobuf v0.0.0-20180919145318-b4deda0973
	github.com/golangplus/bytes => github.com/golangplus/bytes v0.0.0-20160111154220-45c989fe5450
	github.com/golangplus/fmt => github.com/golangplus/fmt v0.0.0-20150411045040-2a5d6d7d2995
	github.com/golangplus/testing => github.com/golangplus/testing v0.0.0-20180327235837-af21d9c3145e
	github.com/gonum/blas => github.com/gonum/blas v0.0.0-20190702153934-37e8262649
	github.com/gonum/diff => github.com/gonum/diff v0.0.0-20181124234638-500114f11e71
	github.com/gonum/floats => github.com/gonum/floats v0.0.0-20190702153934-f74b330d45
	github.com/gonum/graph => github.com/gonum/graph v0.0.0-20190702153934-50b27dea7e
	github.com/gonum/integrate => github.com/gonum/integrate v0.0.0-20181209220457-a422b5c0fdf2
	github.com/gonum/internal => github.com/gonum/internal v0.0.0-20190702153934-e57e4534cf
	github.com/gonum/lapack => github.com/gonum/lapack v0.0.0-20190702153934-5ed4b826be
	github.com/gonum/mathext => github.com/gonum/mathext v0.0.0-20181121095525-8a4bf007ea55
	github.com/gonum/matrix => github.com/gonum/matrix v0.0.0-20190702153934-dd6034299e
	github.com/gonum/stat => github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/google/btree => github.com/google/btree v0.0.0-20180919145318-20236160a4
	github.com/google/cadvisor => github.com/google/cadvisor v0.0.0-20180919145318-8949c822ea
	github.com/google/certificate-transparency-go => github.com/google/certificate-transparency-go v0.0.0-20180919145318-3629d68465
	github.com/google/go-cmp => github.com/google/go-cmp v0.3.0
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20180919145318-24818f796f
	github.com/google/martian => github.com/google/martian v2.1.0+incompatible
	github.com/google/pprof => github.com/google/pprof v0.0.0-20190515194954-54271f7e092f
	github.com/google/uuid => github.com/google/uuid v0.0.0-20180919145318-8c31c18f31
	github.com/googleapis/gax-go/v2 => github.com/googleapis/gax-go/v2 v2.0.5
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.0.0-20180919145318-0c5108395e
	github.com/gophercloud/gophercloud => github.com/gophercloud/gophercloud v0.0.0-20180919145318-c818fa66e4
	github.com/gopherjs/gopherjs => github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1
	github.com/gorilla/context => github.com/gorilla/context v0.0.0-20180919145318-51ce91d2ea
	github.com/gorilla/mux => github.com/gorilla/mux v0.0.0-20190702153934-08e7f807d3
	github.com/gorilla/securecookie => github.com/gorilla/securecookie v0.0.0-20180919145318-e65cf8c5df
	github.com/gorilla/sessions => github.com/gorilla/sessions v0.0.0-20190702153934-a3acf13e80
	github.com/gorilla/websocket => github.com/gorilla/websocket v0.0.0-20180919145318-4201258b82
	github.com/gotestyourself/gotestyourself => github.com/gotestyourself/gotestyourself v2.2.0+incompatible
	github.com/gregjones/httpcache => github.com/gregjones/httpcache v0.0.0-20180919145318-787624de3e
	github.com/grpc-ecosystem/go-grpc-middleware => github.com/grpc-ecosystem/go-grpc-middleware v0.0.0-20180919145318-cfaf5686ec
	github.com/grpc-ecosystem/go-grpc-prometheus => github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20180919145318-2500245aa6
	github.com/grpc-ecosystem/grpc-gateway => github.com/grpc-ecosystem/grpc-gateway v0.0.0-20180919145318-8cc3a55af3
	github.com/hashicorp/go-syslog => github.com/hashicorp/go-syslog v1.0.0
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.0.0-20180919145318-20f1fb78b0
	github.com/hashicorp/hcl => github.com/hashicorp/hcl v0.0.0-20180919145318-d8c773c4cb
	github.com/heketi/heketi => github.com/heketi/heketi v0.0.0-20180919145318-558b29266c
	github.com/heketi/rest => github.com/heketi/rest v0.0.0-20180404230133-aa6a65207413
	github.com/heketi/tests => github.com/heketi/tests v0.0.0-20151005000721-f3775cbcefd6
	github.com/heketi/utils => github.com/heketi/utils v0.0.0-20170317161834-435bc5bdfa64
	github.com/hpcloud/tail => github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo => github.com/imdario/mergo v0.0.0-20180919145318-9316a62528
	github.com/inconshreveable/mousetrap => github.com/inconshreveable/mousetrap v0.0.0-20180919145318-76626ae9c9
	github.com/jimstudt/http-authentication => github.com/jimstudt/http-authentication v0.0.0-20140401203705-3eca13d6893a
	github.com/jmespath/go-jmespath => github.com/jmespath/go-jmespath v0.0.0-20180919145318-0b12d6b521
	github.com/joho/godotenv => github.com/joho/godotenv v0.0.0-20190702153934-6d367c18ed
	github.com/jonboulle/clockwork => github.com/jonboulle/clockwork v0.0.0-20180919145318-72f9bd7c4e
	github.com/json-iterator/go => github.com/json-iterator/go v0.0.0-20180919145318-ab8a2e0c74
	github.com/jstemmer/go-junit-report => github.com/jstemmer/go-junit-report v0.0.0-20190106144839-af01ea7f8024
	github.com/jteeuwen/go-bindata => github.com/jteeuwen/go-bindata v0.0.0-20190702153934-a0ff2567cf
	github.com/jtolds/gls => github.com/jtolds/gls v4.20.0+incompatible
	github.com/julienschmidt/httprouter => github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext => github.com/kardianos/osext v0.0.0-20180919145318-8fef92e41e
	github.com/karrick/godirwalk => github.com/karrick/godirwalk v0.0.0-20180919145318-2de2192f9e
	github.com/kisielk/errcheck => github.com/kisielk/errcheck v1.1.0
	github.com/kisielk/gotool => github.com/kisielk/gotool v1.0.0
	github.com/klauspost/cpuid => github.com/klauspost/cpuid v1.2.0
	github.com/konsorten/go-windows-terminal-sequences => github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/kr/fs => github.com/kr/fs v0.0.0-20180919145318-2788f0dbd1
	github.com/kr/logfmt => github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/kr/pretty => github.com/kr/pretty v0.1.0
	github.com/kr/pty => github.com/kr/pty v1.1.5
	github.com/kr/text => github.com/kr/text v0.1.0
	github.com/kylelemons/godebug => github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/lestrrat-go/jspointer => github.com/lestrrat-go/jspointer v0.0.0-20181205001929-82fadba7561c
	github.com/lestrrat-go/jsref => github.com/lestrrat-go/jsref v0.0.0-20181205001954-1b590508f37d
	github.com/lestrrat-go/jsschema => github.com/lestrrat-go/jsschema v0.0.0-20181205002244-5c81c58ffcc3
	github.com/lestrrat-go/jsval => github.com/lestrrat-go/jsval v0.0.0-20181205002323-20277e9befc0
	github.com/lestrrat-go/pdebug => github.com/lestrrat-go/pdebug v0.0.0-20180220043849-39f9a71bcabe
	github.com/lestrrat-go/structinfo => github.com/lestrrat-go/structinfo v0.0.0-20190212233437-acd51874663b
	github.com/lestrrat/go-jspointer => github.com/lestrrat/go-jspointer v0.0.0-20190702153934-f4881e611b
	github.com/lestrrat/go-jsref => github.com/lestrrat/go-jsref v0.0.0-20190702153934-50df7b2d07
	github.com/lestrrat/go-jsschema => github.com/lestrrat/go-jsschema v0.0.0-20190702153934-a6a42341b5
	github.com/lestrrat/go-jsval => github.com/lestrrat/go-jsval v0.0.0-20181205002323-20277e9befc0
	github.com/lestrrat/go-pdebug => github.com/lestrrat/go-pdebug v0.0.0-20180919145318-569c97477a
	github.com/lestrrat/go-structinfo => github.com/lestrrat/go-structinfo v0.0.0-20180919145318-8204d40bbc
	github.com/libopenstorage/openstorage => github.com/libopenstorage/openstorage v0.0.0-20180919145318-093a0c3888
	github.com/liggitt/tabwriter => github.com/liggitt/tabwriter v0.0.0-20180919145318-89fcab3d43
	github.com/lithammer/dedent => github.com/lithammer/dedent v0.0.0-20180919145318-8478954c3b
	github.com/lpabon/godbc => github.com/lpabon/godbc v0.1.1
	github.com/lucas-clemente/aes12 => github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f
	github.com/lucas-clemente/quic-clients => github.com/lucas-clemente/quic-clients v0.1.0
	github.com/lucas-clemente/quic-go => github.com/lucas-clemente/quic-go v0.10.2
	github.com/lucas-clemente/quic-go-certificates => github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced
	github.com/magiconair/properties => github.com/magiconair/properties v0.0.0-20180919145318-61b492c03c
	github.com/mailru/easyjson => github.com/mailru/easyjson v0.0.0-20180919145318-2f5df55504
	github.com/marstr/guid => github.com/marstr/guid v0.0.0-20180919145318-8bdf7d1a08
	github.com/marten-seemann/qtls => github.com/marten-seemann/qtls v0.2.3
	github.com/mattn/go-shellwords => github.com/mattn/go-shellwords v0.0.0-20180919145318-f8471b0a71
	github.com/matttproud/golang_protobuf_extensions => github.com/matttproud/golang_protobuf_extensions v0.0.0-20180919145318-c12348ce28
	github.com/mesos/mesos-go => github.com/mesos/mesos-go v0.0.0-20180919145318-ff8175bfda
	github.com/mholt/caddy => github.com/caddyserver/caddy v1.0.1
	github.com/mholt/certmagic => github.com/mholt/certmagic v0.6.2-0.20190624175158-6a42ef9fe8c2
	github.com/miekg/dns => github.com/miekg/dns v0.0.0-20180919145318-5a2b9fab83
	github.com/mindprince/gonvml => github.com/mindprince/gonvml v0.0.0-20180919145318-fee913ce8f
	github.com/mistifyio/go-zfs => github.com/mistifyio/go-zfs v0.0.0-20180919145318-1b4ae6fb4e
	github.com/mitchellh/go-homedir => github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-wordwrap => github.com/mitchellh/go-wordwrap v0.0.0-20180919145318-9e67c67572
	github.com/mitchellh/mapstructure => github.com/mitchellh/mapstructure v0.0.0-20180919145318-53818660ed
	github.com/moby/buildkit => github.com/moby/buildkit v0.0.0-20190702153934-c3a857e3fc
	github.com/modern-go/concurrent => github.com/modern-go/concurrent v0.0.0-20180919145318-bacd9c7ef1
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180919145318-94122c33ed
	github.com/mohae/deepcopy => github.com/mohae/deepcopy v0.0.0-20180919145318-491d3605ed
	github.com/mrunalp/fileutils => github.com/mrunalp/fileutils v0.0.0-20180919145318-4ee1cc9a80
	github.com/mtrmac/gpgme => github.com/mtrmac/gpgme v0.0.0-20180919145318-b243242868
	github.com/munnerz/goautoneg => github.com/munnerz/goautoneg v0.0.0-20180919145318-a547fc61f4
	github.com/mvdan/xurls => github.com/mvdan/xurls v1.1.0
	github.com/mwitkow/go-conntrack => github.com/mwitkow/go-conntrack v0.0.0-20161129095857-cc309e4a2223
	github.com/mxk/go-flowrate => github.com/mxk/go-flowrate v0.0.0-20180919145318-cca7078d47
	github.com/naoina/go-stringutil => github.com/naoina/go-stringutil v0.1.0
	github.com/naoina/toml => github.com/naoina/toml v0.1.1
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v0.0.0-20180919145318-53ca7dc85f
	github.com/onsi/gomega => github.com/onsi/gomega v0.0.0-20180919145318-5533ce8a0d
	github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v0.0.0-20180919145318-ac19fd6e74
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v0.0.0-20180919145318-372ad780f6
	github.com/opencontainers/runc => github.com/openshift/opencontainers-runc v0.0.0-20190702153934-7c7775178c
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v0.0.0-20180919145318-02137cd4e5
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v0.0.0-20180919145318-4a2974bf1e
	github.com/openshift/api => github.com/openshift/api v0.0.0-20180919145318-64d243ed05
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20180919145318-8892c0adc0
	github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20180919145318-16a370625b
	github.com/openshift/oauth-server => ../../../../../../staging/src/github.com/openshift/oauth-server
	github.com/openshift/oc => ../../../../../../staging/src/github.com/openshift/oc
	github.com/openshift/openshift-apiserver => ../../../../../../staging/src/github.com/openshift/openshift-apiserver
	github.com/openshift/openshift-controller-manager => ../../../../../../staging/src/github.com/openshift/openshift-controller-manager
	github.com/openshift/sdn => ../../../../../../staging/src/github.com/openshift/sdn
	github.com/openshift/source-to-image => github.com/openshift/source-to-image v0.0.0-20180919145318-3dee73c8b7
	github.com/openshift/template-service-broker => ../../../../../../staging/src/github.com/openshift/template-service-broker
	github.com/pborman/uuid => github.com/pborman/uuid v0.0.0-20180919145318-ca53cad383
	github.com/pelletier/go-toml => github.com/pelletier/go-toml v0.0.0-20180919145318-dba45d427f
	github.com/peterbourgon/diskv => github.com/peterbourgon/diskv v0.0.0-20180919145318-5f041e8faa
	github.com/pkg/errors => github.com/pkg/errors v0.0.0-20180919145318-645ef00459
	github.com/pkg/profile => github.com/pkg/profile v0.0.0-20180919145318-f6fe06335d
	github.com/pkg/sftp => github.com/pkg/sftp v0.0.0-20180919145318-4d0e916071
	github.com/pmezard/go-difflib => github.com/pmezard/go-difflib v0.0.0-20180919145318-5d4384ee4f
	github.com/pquerna/cachecontrol => github.com/pquerna/cachecontrol v0.0.0-20180919145318-0dec1b30a0
	github.com/pquerna/ffjson => github.com/pquerna/ffjson v0.0.0-20180919145318-af8b230fcd
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.0.0-20180919145318-505eaef017
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.0.0-20180919145318-fa8ad6fec3
	github.com/prometheus/common => github.com/prometheus/common v0.0.0-20180919145318-cfeb6f9992
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.0.0-20180919145318-65c1f6f8f0
	github.com/quobyte/api => github.com/quobyte/api v0.0.0-20180919145318-9cfd29338d
	github.com/rancher/go-rancher => github.com/rancher/go-rancher v0.0.0-20180919145318-09693a8743
	github.com/remyoudompheng/bigfft => github.com/remyoudompheng/bigfft v0.0.0-20170806203942-52369c62f446
	github.com/robfig/cron => github.com/robfig/cron v0.0.0-20180919145318-df38d32658
	github.com/rogpeppe/fastuuid => github.com/rogpeppe/fastuuid v0.0.0-20150106093220-6724a57986af
	github.com/rogpeppe/go-charset => github.com/rogpeppe/go-charset v0.0.0-20180617210344-2471d30d28b4
	github.com/rubiojr/go-vhd => github.com/rubiojr/go-vhd v0.0.0-20180919145318-0bfd3b3985
	github.com/russross/blackfriday => github.com/russross/blackfriday v0.0.0-20180919145318-300106c228
	github.com/satori/go.uuid => github.com/satori/go.uuid v0.0.0-20180919145318-f58768cc1a
	github.com/seccomp/libseccomp-golang => github.com/seccomp/libseccomp-golang v0.0.0-20180919145318-1b506fc7c2
	github.com/shurcooL/sanitized_anchor_name => github.com/shurcooL/sanitized_anchor_name v0.0.0-20180919145318-10ef21a441
	github.com/sigma/go-inotify => github.com/sigma/go-inotify v0.0.0-20180919145318-c87b6cf503
	github.com/sirupsen/logrus => github.com/sirupsen/logrus v0.0.0-20190702153934-89742aefa4
	github.com/smartystreets/assertions => github.com/smartystreets/assertions v0.0.0-20180927180507-b2de0cb4f26d
	github.com/smartystreets/goconvey => github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a
	github.com/soheilhy/cmux => github.com/soheilhy/cmux v0.0.0-20180919145318-bb79a83465
	github.com/spf13/afero => github.com/spf13/afero v0.0.0-20180919145318-b28a7effac
	github.com/spf13/cast => github.com/spf13/cast v0.0.0-20180919145318-e31f36ffc9
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20180919145318-c439c4fa09
	github.com/spf13/jwalterweatherman => github.com/spf13/jwalterweatherman v0.0.0-20180919145318-33c24e77fb
	github.com/spf13/pflag => github.com/spf13/pflag v0.0.0-20180919145318-583c0c0531
	github.com/spf13/viper => github.com/spf13/viper v0.0.0-20180919145318-7fb2782df3
	github.com/storageos/go-api => github.com/storageos/go-api v0.0.0-20180919145318-343b3eff91
	github.com/stretchr/objx => github.com/stretchr/objx v0.0.0-20180919145318-1a9d0bb9f5
	github.com/stretchr/testify => github.com/stretchr/testify v0.0.0-20180919145318-c679ae2cc0
	github.com/syndtr/gocapability => github.com/syndtr/gocapability v0.0.0-20180919145318-e7cb7fa329
	github.com/thecodeteam/goscaleio => github.com/thecodeteam/goscaleio v0.1.0
	github.com/tmc/grpc-websocket-proxy => github.com/tmc/grpc-websocket-proxy v0.0.0-20180919145318-89b8d40f7c
	github.com/ugorji/go => github.com/ugorji/go v0.0.0-20180919145318-bdcc60b419
	github.com/ugorji/go/codec => github.com/ugorji/go/codec v1.1.7
	github.com/urfave/negroni => github.com/urfave/negroni v1.0.0
	github.com/vishvananda/netlink => github.com/vishvananda/netlink v0.0.0-20190702153934-b2de5d10e3
	github.com/vishvananda/netns => github.com/vishvananda/netns v0.0.0-20180919145318-be1fbeda19
	github.com/vjeantet/asn1-ber => github.com/vjeantet/asn1-ber v0.0.0-20180919145318-85041cd0f4
	github.com/vjeantet/ldapserver => github.com/vjeantet/ldapserver v0.0.0-20180919145318-5ac5872957
	github.com/vmware/govmomi => github.com/vmware/govmomi v0.0.0-20180919145318-22f74650cf
	github.com/vmware/photon-controller-go-sdk => github.com/vmware/photon-controller-go-sdk v0.0.0-20180919145318-4a435daef6
	github.com/xanzy/go-cloudstack => github.com/xanzy/go-cloudstack v0.0.0-20180919145318-1e2cbf647e
	github.com/xeipuuv/gojsonpointer => github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f
	github.com/xeipuuv/gojsonreference => github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415
	github.com/xeipuuv/gojsonschema => github.com/xeipuuv/gojsonschema v1.1.0
	github.com/xiang90/probing => github.com/xiang90/probing v0.0.0-20180919145318-07dd2e8dfe
	github.com/xlab/handysort => github.com/xlab/handysort v0.0.0-20150421192137-fb3537ed64a1
	github.com/xordataexchange/crypt => github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77
	go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.2
	go.opencensus.io => go.opencensus.io v0.22.0
	go.uber.org/atomic => go.uber.org/atomic v0.0.0-20180919145318-8dc6146f75
	go.uber.org/multierr => go.uber.org/multierr v0.0.0-20180919145318-ddea229ff1
	go.uber.org/zap => go.uber.org/zap v0.0.0-20180919145318-67bc79d13d
	go4.org => github.com/go4org/go4 v0.0.0-20190702153934-03efcb870d
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20180919145318-de07523181
	golang.org/x/exp => golang.org/x/exp v0.0.0-20190510132918-efd6b22b2522
	golang.org/x/image => golang.org/x/image v0.0.0-20190227222117-0694c2d4d067
	golang.org/x/lint => golang.org/x/lint v0.0.0-20190409202823-959b441ac422
	golang.org/x/mobile => golang.org/x/mobile v0.0.0-20190312151609-d3739f865fa6
	golang.org/x/net => golang.org/x/net v0.0.0-20180919145318-65e2d4e150
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20180919145318-a6bd8cefa1
	golang.org/x/sync => golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys => golang.org/x/sys v0.0.0-20180919145318-95c6576299
	golang.org/x/text => golang.org/x/text v0.0.0-20180919145318-b19bf474d3
	golang.org/x/time => golang.org/x/time v0.0.0-20180919145318-9d24e82272
	golang.org/x/tools => golang.org/x/tools v0.0.0-20180919145318-7f7074d5bc
	gonum.org/v1/gonum => github.com/gonum/gonum v0.0.0-20190702153934-cebdade430
	gonum.org/v1/netlib => gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e
	google.golang.org/api => google.golang.org/api v0.0.0-20180919145318-583d854617
	google.golang.org/appengine => github.com/golang/appengine v0.0.0-20190702153934-12d5545dc1
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180919145318-09f6ed296f
	google.golang.org/grpc => google.golang.org/grpc v0.0.0-20180919145318-168a6198bc
	gopkg.in/airbrake/gobrake.v2 => gopkg.in/airbrake/gobrake.v2 v2.0.9
	gopkg.in/alecthomas/kingpin.v2 => gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/asn1-ber.v1 => gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d
	gopkg.in/check.v1 => gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/fsnotify.v1 => gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/gcfg.v1 => gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 => gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.1
	gopkg.in/ldap.v2 => gopkg.in/ldap.v2 v2.5.1
	gopkg.in/mcuadros/go-syslog.v2 => gopkg.in/mcuadros/go-syslog.v2 v2.2.1
	gopkg.in/natefinch/lumberjack.v2 => gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/resty.v1 => gopkg.in/resty.v1 v1.12.0
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/tomb.v1 => gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/warnings.v0 => gopkg.in/warnings.v0 v0.1.2
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2
	gotest.tools => gotest.tools v2.2.0+incompatible
	honnef.co/go/tools => honnef.co/go/tools v0.0.0-20190418001031-e561f6794a2a
	k8s.io/api => ../api
	k8s.io/apiextensions-apiserver => ../apiextensions-apiserver
	k8s.io/apimachinery => ../apimachinery
	k8s.io/apiserver => ../apiserver
	k8s.io/cli-runtime => ../cli-runtime
	k8s.io/client-go => ../client-go
	k8s.io/cloud-provider => ../cloud-provider
	k8s.io/cluster-bootstrap => ../cluster-bootstrap
	k8s.io/code-generator => ../code-generator
	k8s.io/component-base => ../component-base
	k8s.io/csi-api => ../csi-api
	k8s.io/csi-translation-lib => ../csi-translation-lib
	k8s.io/gengo => github.com/kubernetes/gengo v0.0.0-20190702153934-51747d6e00
	k8s.io/heapster => k8s.io/heapster v0.0.0-20180919145318-c2ac40f1ad
	k8s.io/klog => github.com/kubernetes/klog v0.0.0-20190702153934-8139d8cb77
	k8s.io/kube-aggregator => ../kube-aggregator
	k8s.io/kube-controller-manager => ../kube-controller-manager
	k8s.io/kube-openapi => github.com/kubernetes/kube-openapi v0.0.0-20190702153934-b3a7cee44a
	k8s.io/kube-proxy => ../kube-proxy
	k8s.io/kube-scheduler => ../kube-scheduler
	k8s.io/kubelet => ../kubelet
	k8s.io/kubernetes => github.com/openshift/kubernetes v0.0.0-20180919145318-3a0658d1a4
	k8s.io/metrics => ../metrics
	k8s.io/node-api => ../node-api
	k8s.io/sample-apiserver => ../sample-apiserver
	k8s.io/sample-cli-plugin => ../sample-cli-plugin
	k8s.io/sample-controller => ../sample-controller
	k8s.io/utils => github.com/kubernetes/utils v0.0.0-20190702153934-c2654d5206
	modernc.org/cc => modernc.org/cc v1.0.0
	modernc.org/golex => modernc.org/golex v1.0.0
	modernc.org/mathutil => modernc.org/mathutil v1.0.0
	modernc.org/strutil => modernc.org/strutil v1.0.0
	modernc.org/xc => modernc.org/xc v1.0.0
	rsc.io/binaryregexp => rsc.io/binaryregexp v0.2.0
	sigs.k8s.io/kustomize => sigs.k8s.io/kustomize v0.0.0-20180919145318-a6f6514412
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20180919145318-e85c7b244f
	sigs.k8s.io/yaml => github.com/kubernetes-sigs/yaml v0.0.0-20190702153934-4e761d0940
	vbom.ml/util => vbom.ml/util v0.0.0-20180919145318-db5cfe13f5
)
