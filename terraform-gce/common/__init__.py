import os.path


def write_asset(filename, content):
    dirname = 'assets'
    if not os.path.exists(dirname):
        os.makedirs(dirname)

    with open(os.path.join(dirname, filename), 'wt') as f:
        f.write(content)


def write_instance_env(is_master=False):
    with open('assets/env.yaml', 'wt') as f:
        f.write("""ENV_TIMESTAMP: "{0}"
INSTANCE_PREFIX: "kubernetes"
NODE_INSTANCE_PREFIX: "kubernetes-node"
CLUSTER_IP_RANGE: "10.244.0.0/16"
SERVER_BINARY_TAR_URL: "TODO"
SERVER_BINARY_TAR_HASH: "TODO"
SERVICE_CLUSTER_IP_RANGE: "10.0.0.0/16"
KUBERNETES_MASTER_NAME: "kubernetes-master"
ALLOCATE_NODE_CIDRS: true
ENABLE_CLUSTER_MONITORING: true
ENABLE_L7_LOADBALANCING: "glbc"
ENABLE_CLUSTER_LOGGING: true
ENABLE_CLUSTER_UI: true
ENABLE_NODE_LOGGING: true
LOGGING_DESTINATION: "gcp"
ELASTICSEARCH_LOGGING_REPLICAS: 1
ENABLE_CLUSTER_DNS: true
ENABLE_CLUSTER_REGISTRY: false
CLUSTER_REGISTRY_DISK: ""
CLUSTER_REGISTRY_DISK_SIZE: ""
DNS_REPLICAS: 1
DNS_SERVER_IP: "10.0.0.10"
DNS_DOMAIN: "cluster.local"
KUBELET_TOKEN: "TODO"
KUBE_PROXY_TOKEN: "TODO"
ADMISSION_CONTROL: "NamespaceLifecycle,LimitRanger,ServiceAccount,ResourceQuota,PersistentVolumeLabel"
MASTER_IP_RANGE: "10.246.0.0/24"
RUNTIME_CONFIG: ""
CA_CERT: "TODO: Base64 of ca.crt"
KUBELET_CERT: "TODO: Base64 of kubelet.crt"
KUBELET_KEY: "TODO: Base64 of kubelet.key"
NETWORK_PROVIDER: "none"
HAIRPIN_MODE: "promiscuous-bridge"
OPENCONTRAIL_TAG: "R2.20"
OPENCONTRAIL_KUBERNETES_TAG: "master"
OPENCONTRAIL_PUBLIC_SUBNET: "10.1.0.0/16"
E2E_STORAGE_TEST_ENVIRONMENT: false
KUBE_IMAGE_TAG: "TODO"
KUBE_DOCKER_REGISTRY: ""
KUBE_ADDON_REGISTRY: "gcr.io/google_containers"
MULTIZONE: ""
NON_MASQUERADE_CIDR: ""
""".format('2016-03-25T21:36:42+0000'))  # TODO: Compute date
        if is_master:
            f.write("""KUBERNETES_MASTER: true
KUBE_USER: "admin"
KUBE_PASSWORD: "TODO: Insert basicauth password"
KUBE_BEARER_TOKEN: "TODO: Insert bearer token"
MASTER_CERT: "TODO: Insert b64 of master cert"
MASTER_KEY: "TODO: Insert b64 of master key"
KUBECFG_CERT: "TODO: Insert b64 of kubecfg cert"
KUBECFG_KEY: "TODO: Insert b64 of kubecfg key"
KUBELET_APISERVER: "kubernetes-master"
ENABLE_MANIFEST_URL: false
MANIFEST_URL: ""
MANIFEST_URL_HEADER: ""
NUM_NODES: "TODO: Insert number of nodes"
""")
        else:
            f.write("""KUBERNETES_MASTER: false
ZONE: "TODO: Insert zone"
EXTRA_DOCKER_OPTS: ""
""")
            # CoreOS-only env vars
            f.write("""KUBE_MANIFESTS_TAR_URL: "TODO:"
KUBE_MANIFESTS_TAR_HASH: "TODO"
KUBERNETES_CONTAINER_RUNTIME: "docker"
RKT_VERSION: ""
RKT_PATH: ""
KUBERNETES_CONFIGURE_CBR0: true
""")
