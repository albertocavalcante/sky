# Custom Starlark file using custom dialect builtins
# Tests the comprehensive builtin definitions

# Use minimal builtin
greeting = hello("World")

# Use comprehensive builtins
func_minimal()
result = func_required_param("test")
result2 = func_optional_param()
result3 = func_mixed_params("req1", 42, optional1=False)

# Use variadic functions
func_variadic("first", "second", "third")
func_kwargs("first", key1="value1", key2="value2")

# Use types
simple = SimpleType()
with_methods = TypeWithMethods()
value = with_methods.get("key", "default")
with_methods.set("key", "value")
all_keys = with_methods.keys()

# Use globals
print(VERSION)
if DEBUG:
    print("Debug mode enabled")

# Use modules
formatted = util.format("Hello {name}", name="World")
print(util.UTIL_VERSION)

# Use nested modules
nested.deep.deep_func()

# Test deprecated function (should show warning)
func_deprecated()  # deprecated

# Complex types
func_complex_types(
    nested_list=[[1, 2], [3, 4]],
    dict_of_lists={"a": ["x", "y"]},
    union_type="string_value",
)
