# Tilt Starlark builtins
# Python stub format compatible with tilt-dev/starlark-lsp
"""Tilt Starlark builtins for local Kubernetes development."""

from typing import Any, Dict, List, Optional, Union


class Blob:
    """Binary large object - file contents or command output."""

    def __str__(self) -> str:
        """Convert to string."""
        ...


class LiveUpdateStep:
    """A step in the live update process."""
    ...


class PortForward:
    """Port forward specification."""
    local_port: int
    container_port: int


class OsModule:
    """Operating system utilities."""

    def getcwd(self) -> str:
        """Get current working directory."""
        ...

    def getenv(self, key: str, default: Optional[str] = None) -> str:
        """Get environment variable."""
        ...

    def environ(self) -> Dict[str, str]:
        """Get all environment variables."""
        ...


# Global modules
os: OsModule
"""Operating system utilities."""

config: Dict[str, Any]
"""Tilt configuration settings."""


def docker_build(
    ref: str,
    context: str = ".",
    dockerfile: str = "Dockerfile",
    build_args: Optional[Dict[str, str]] = None,
    live_update: Optional[List[LiveUpdateStep]] = None,
    ignore: Optional[List[str]] = None,
    **kwargs,
) -> None:
    """
    Build a Docker image from a Dockerfile.

    This is the most common way to build images in Tilt.

    Args:
        ref: Image reference (e.g., 'myimage:latest')
        context: Build context path
        dockerfile: Path to Dockerfile
        build_args: Build arguments
        live_update: Live update rules
        ignore: Patterns to ignore
        **kwargs: Additional arguments
    """
    ...


def k8s_yaml(
    yaml: Union[str, List[str], Blob],
    allow_duplicates: bool = False,
) -> None:
    """
    Deploy Kubernetes YAML manifests.

    Args:
        yaml: YAML content or file paths
        allow_duplicates: Allow duplicate resource names
    """
    ...


def k8s_resource(
    workload: str,
    port_forwards: Optional[List[Union[str, int, PortForward]]] = None,
    resource_deps: Optional[List[str]] = None,
    labels: Optional[List[str]] = None,
) -> None:
    """
    Configure a Kubernetes resource.

    Args:
        workload: Workload name
        port_forwards: Port forward specifications
        resource_deps: Resource dependencies
        labels: Labels for grouping
    """
    ...


def local_resource(
    name: str,
    cmd: Union[str, List[str]],
    serve_cmd: Optional[Union[str, List[str]]] = None,
    deps: Optional[List[str]] = None,
    resource_deps: Optional[List[str]] = None,
) -> None:
    """
    Run a command on the host machine.

    Args:
        name: Resource name
        cmd: Command to run
        serve_cmd: Long-running serve command
        deps: File dependencies
        resource_deps: Resource dependencies
    """
    ...


def local(
    cmd: Union[str, List[str]],
    quiet: bool = False,
    echo_off: bool = False,
) -> Blob:
    """
    Run a local command and return output.

    Args:
        cmd: Command to run
        quiet: Suppress output
        echo_off: Don't echo command

    Returns:
        Command output as Blob
    """
    ...


def read_file(
    path: str,
    default: Optional[str] = None,
) -> Blob:
    """
    Read file contents as a Blob.

    Args:
        path: File path
        default: Default if file not found

    Returns:
        File contents as Blob
    """
    ...


def read_json(
    path: str,
    default: Any = None,
) -> Any:
    """
    Read and parse a JSON file.

    Args:
        path: File path
        default: Default if file not found

    Returns:
        Parsed JSON data
    """
    ...


def read_yaml(
    path: str,
    default: Any = None,
) -> Any:
    """
    Read and parse a YAML file.

    Args:
        path: File path
        default: Default if file not found

    Returns:
        Parsed YAML data
    """
    ...


def helm(
    chart: str,
    name: Optional[str] = None,
    namespace: Optional[str] = None,
    values: Optional[List[str]] = None,
    set: Optional[List[str]] = None,
) -> Blob:
    """
    Deploy a Helm chart.

    Args:
        chart: Chart path or name
        name: Release name
        namespace: Kubernetes namespace
        values: Values files
        set: Set values on command line

    Returns:
        Rendered YAML as Blob
    """
    ...


def load_dynamic(path: str) -> None:
    """
    Dynamically load a Tiltfile extension.

    Args:
        path: Path to extension
    """
    ...
