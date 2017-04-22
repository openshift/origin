package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = ["provider_test.go"],
    library = ":go_default_library",
    tags = ["automanaged"],
    deps = [
        "//pkg/api:go_default_library",
        "//pkg/apis/extensions:go_default_library",
        "//pkg/securitycontextconstraints/util:go_default_library",
        "//vendor:k8s.io/apimachinery/pkg/apis/meta/v1",
        "//vendor:k8s.io/apimachinery/pkg/util/diff",
        "//vendor:k8s.io/apimachinery/pkg/util/validation/field",
    ],
)

go_library(
    name = "go_default_library",
    srcs = [
        "provider.go",
        "types.go",
    ],
    tags = ["automanaged"],
    deps = [
        "//pkg/api:go_default_library",
        "//pkg/apis/extensions:go_default_library",
        "//pkg/security/podsecuritypolicy/sysctl:go_default_library",
        "//pkg/securitycontextconstraints/capabilities:go_default_library",
        "//pkg/securitycontextconstraints/group:go_default_library",
        "//pkg/securitycontextconstraints/seccomp:go_default_library",
        "//pkg/securitycontextconstraints/selinux:go_default_library",
        "//pkg/securitycontextconstraints/user:go_default_library",
        "//pkg/securitycontextconstraints/util:go_default_library",
        "//vendor:k8s.io/apimachinery/pkg/util/validation/field",
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
    srcs = [
        ":package-srcs",
        "//pkg/securitycontextconstraints/capabilities:all-srcs",
        "//pkg/securitycontextconstraints/group:all-srcs",
        "//pkg/securitycontextconstraints/seccomp:all-srcs",
        "//pkg/securitycontextconstraints/selinux:all-srcs",
        "//pkg/securitycontextconstraints/user:all-srcs",
        "//pkg/securitycontextconstraints/util:all-srcs",
    ],
    tags = ["automanaged"],
)
