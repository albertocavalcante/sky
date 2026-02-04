# BUILD file in multi-dialect project
cc_library(
    name = "lib",
    srcs = glob(include = ["*.cc"]),
)

cc_binary(
    name = "app",
    deps = [":lib"],
)
