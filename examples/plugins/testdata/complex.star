"""A more complex Starlark file for testing."""

load(":example.bzl", "greet", "add")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Constants
VERSION = "1.0.0"
DEBUG = False

def _impl(ctx):
    """Rule implementation."""
    output = ctx.actions.declare_file(ctx.label.name + ".out")
    ctx.actions.write(output, "content")
    return [DefaultInfo(files = depset([output]))]

my_rule = rule(
    implementation = _impl,
    attrs = {
        "srcs": attr.label_list(allow_files = True),
        "deps": attr.label_list(),
        "data": attr.label_list(allow_files = True),
    },
)

def my_macro(name, srcs = [], deps = [], **kwargs):
    """A macro that wraps my_rule.

    Args:
        name: Target name.
        srcs: Source files.
        deps: Dependencies.
        **kwargs: Additional arguments.
    """
    my_rule(
        name = name,
        srcs = srcs,
        deps = deps,
        **kwargs
    )

def compute(x, y, z, operation = "add"):
    """Perform a computation.

    Args:
        x: First operand.
        y: Second operand.
        z: Third operand.
        operation: The operation to perform.

    Returns:
        The result.
    """
    if operation == "add":
        return x + y + z
    elif operation == "multiply":
        return x * y * z
    else:
        fail("Unknown operation: " + operation)

# Some calls
result1 = greet("World")
result2 = add(1, 2)
result3 = compute(1, 2, 3)

# Print statements (should be flagged by no-print lint)
print("Debug:", result1)
print("Result:", result2)
