"""Example Starlark file for testing plugins."""

load("@rules_go//go:def.bzl", "go_library", "go_test")

def greet(name):
    """Greet someone by name.

    Args:
        name: The name to greet.

    Returns:
        A greeting string.
    """
    return "Hello, " + name + "!"

def add(a, b):
    """Add two numbers.

    Args:
        a: First number.
        b: Second number.

    Returns:
        The sum of a and b.
    """
    return a + b

# A simple variable
MESSAGE = "Welcome to Sky"

# A list of items
ITEMS = [
    "item1",
    "item2",
    "item3",
]

# A dictionary
CONFIG = {
    "debug": True,
    "level": 3,
}
