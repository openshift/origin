workspace(name = "com_github_bazelbuild_buildtools")

# 0.5.5
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "51bc38fd56e61ffd83f455e3083858ca3cc307d76fa1387b2ae2baae379afcc2",
    strip_prefix = "rules_go-71cdb6fd5f887d215bdbe0e4d1eb137278b09c39",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/archive/71cdb6fd5f887d215bdbe0e4d1eb137278b09c39.tar.gz",
        "https://github.com/bazelbuild/rules_go/archive/71cdb6fd5f887d215bdbe0e4d1eb137278b09c39.tar.gz",
    ],
)

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_rules_dependencies",
    "go_register_toolchains",
    "go_repository",
)

go_rules_dependencies()

go_register_toolchains()

# used for build.proto
http_archive(
    name = "io_bazel",
    sha256 = "255e1199c0876b9a8cc02d5ea569b6cfe1901d30428355817b7606ddecb04c15",
    strip_prefix = "bazel-0.8.0",
    urls = [
        "http://mirror.bazel.build/github.com/bazelbuild/bazel/archive/0.8.0.tar.gz",
        "https://github.com/bazelbuild/bazel/archive/0.8.0.tar.gz",
    ],
)
