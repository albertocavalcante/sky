# Tilt extension module
# Uses Tilt builtins in a .star file

def setup_database():
    """Set up the database resource."""
    docker_build(
        ref="postgres:custom",
        context="./database",
    )
    k8s_yaml("k8s/database.yaml")
    k8s_resource(
        workload="database",
        port_forwards=["5432:5432"],
    )

def read_config(path):
    """Read and parse a config file."""
    return read_json(path, default={})

# Helper to run shell commands
def shell(cmd):
    """Run a shell command and return output."""
    return local(cmd, quiet=True)
