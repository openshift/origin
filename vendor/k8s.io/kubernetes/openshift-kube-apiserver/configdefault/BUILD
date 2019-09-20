load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "admission_config.go",
        "kubecontrolplane_default.go",
    ],
    importpath = "k8s.io/kubernetes/openshift-kube-apiserver/configdefault",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/kube-apiserver/app/options:go_default_library",
        "//openshift-kube-apiserver/admission/namespaceconditions:go_default_library",
        "//openshift-kube-apiserver/kubeadmission:go_default_library",
        "//staging/src/k8s.io/apiserver/pkg/admission:go_default_library",
        "//staging/src/k8s.io/client-go/informers:go_default_library",
        "//staging/src/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/github.com/openshift/api/kubecontrolplane/v1:go_default_library",
        "//vendor/github.com/openshift/library-go/pkg/apiserver/admission/admissiontimeout:go_default_library",
        "//vendor/github.com/openshift/library-go/pkg/config/configdefaults:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
