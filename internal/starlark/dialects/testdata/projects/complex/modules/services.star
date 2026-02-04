# Services module
def setup_services():
    docker_build("worker", "./worker")
    k8s_yaml("k8s/worker.yaml")
