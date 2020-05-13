load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["eviction_store.go"],
    importmap = "k8s.io/kubernetes/vendor/github.com/p0lyn0mial/ttl-cache",
    importpath = "github.com/p0lyn0mial/ttl-cache",
    visibility = ["//visibility:public"],
    deps = ["//staging/src/k8s.io/apimachinery/pkg/util/clock:go_default_library"],
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
