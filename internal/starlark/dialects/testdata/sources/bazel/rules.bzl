# Sample .bzl file for testing Bazel bzl dialect
# Uses Bazel rule authoring builtins

def _my_rule_impl(ctx):
    """Implementation of my_rule."""
    output = ctx.actions.declare_file(ctx.label.name + ".out")
    ctx.actions.run(
        outputs = [output],
        inputs = ctx.files.srcs,
        executable = ctx.executable._tool,
        arguments = [f.path for f in ctx.files.srcs] + [output.path],
    )
    return [DefaultInfo(files = depset([output]))]

my_rule = rule(
    implementation = _my_rule_impl,
    attrs = {
        "srcs": attr.label_list(allow_files = True),
        "_tool": attr.label(
            default = "//tools:processor",
            executable = True,
            cfg = "exec",
        ),
    },
)

def my_library(name, srcs = [], deps = [], **kwargs):
    """Macro wrapping cc_library with defaults."""
    cc_library(
        name = name,
        srcs = srcs,
        deps = deps + ["//common:base"],
        copts = ["-Wall", "-Werror"],
        **kwargs
    )
