# Sample BUILD file for testing Bazel dialect
# Uses Bazel-specific builtins

package(default_visibility = ["//visibility:public"])

cc_library(
    name = "mylib",
    srcs = glob(["src/**/*.cc"]),
    hdrs = glob(["include/**/*.h"]),
    deps = [
        "//third_party:absl",
        "@com_google_protobuf//:protobuf",
    ],
    copts = ["-std=c++17"],
)

cc_binary(
    name = "myapp",
    srcs = ["main.cc"],
    deps = [":mylib"],
)

# Use select for conditional deps
cc_library(
    name = "platform_lib",
    srcs = select({
        "@platforms//os:linux": ["linux_impl.cc"],
        "@platforms//os:macos": ["macos_impl.cc"],
        "//conditions:default": ["generic_impl.cc"],
    }),
)

# Test package name function
_package = package_name()
_repo = repository_name()
