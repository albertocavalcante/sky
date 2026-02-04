# BUILD file in multi-dialect project
# Note: This is a test fixture, not real build rules
# Using allow_empty since no actual source files exist

cc_library(
    name = "lib",
    srcs = glob(["*.cc"], allow_empty = True),
)

cc_binary(
    name = "app",
    deps = [":lib"],
)
