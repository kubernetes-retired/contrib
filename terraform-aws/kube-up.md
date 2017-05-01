#Kube-up algorithm
1. Generate tokens
    KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
    KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
2. Detect master image
3. Detect worker image
4. Detect root device
5. Find Kubernetes release tars
6. Make temp dir
7. Create bootstrap script
8. Upload server tars and bootstrap script to S3
9. Create master/worker IAM profiles
10. Generate basicauth admin:randompassword
11. Generate random bearer token
12. Generate SSH key $AWS_SSH_KEY if it doesn't already exist
13. Generate $AWS_SSH_KEY_FINGERPRINT from $AWS_SSH_KEY
14. Import SSH public key to AWS
15. Set up VPC
    1. Get VPC ID:
            $AWS_CMD describe-vpcs \
            --filters Name=tag:Name,Values=kubernetes-vpc \
            Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
            --query Vpcs[].VpcId

    2. Create VPC if VPC ID wasn't found:
            VPC_ID=$($AWS_CMD create-vpc --cidr-block ${VPC_CIDR} --query Vpc.VpcId)
            $AWS_CMD modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-support '{"Value": true}' > $LOG
            $AWS_CMD modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-hostnames '{"Value": true}' > $LOG
            add-tag $VPC_ID Name kubernetes-vpc
            add-tag $VPC_ID KubernetesCluster ${CLUSTER_ID}
16. Create DHCP option set
  1. Get DHCP_OPTION_SET_ID:
            DHCP_OPTION_SET_ID=$($AWS_CMD create-dhcp-options --dhcp-configuration \
            Key=domain-name,Values=${AWS_REGION}.compute.internal Key=domain-name-servers,\
            Values=AmazonProvidedDNS --query DhcpOptions.DhcpOptionsId)
  2. Tag DHCP option set:
            $AWS_CMD create-tags --resources $DHCP_OPTION_SET_ID --tags \
            Key=Name,Value=kubernetes-dhcp-option-set
            $AWS_CMD create-tags --resources $DHCP_OPTION_SET_ID --tags \
            Key=KubernetesCluster,Value=${CLUSTER_ID}
  3. `$AWS_CMD associate-dhcp-options --dhcp-options-id ${DHCP_OPTION_SET_ID} --vpc-id ${VPC_ID}`
17. Set up subnet
  1. Get $SUBNET_ID:
            $AWS_CMD describe-subnets \
           --filters Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
                     Name=availabilityZone,Values=${ZONE} \
                     Name=vpc-id,Values=${VPC_ID} \
           --query Subnets[].SubnetId
  2. Create subnet if it doesn't already exist:
            SUBNET_ID=$($AWS_CMD create-subnet --cidr-block ${SUBNET_CIDR} --vpc-id $VPC_ID \
            --availability-zone ${ZONE} --query Subnet.SubnetId)
            $AWS_CMD create-tags --resources $SUBNET_ID --tags \
            Key=KubernetesCluster,Value=${CLUSTER_ID}
18. Attach Internet gateway
  1. Get $IGW_ID:
            $AWS_CMD describe-internet-gateways \
            --filters Name=attachment.vpc-id,Values=${VPC_ID} \
            --query InternetGateways[].InternetGatewayId
  2. If not already existant, create Internet gateway $IGW_ID:
            IGW_ID=$($AWS_CMD create-internet-gateway --query InternetGateway.InternetGatewayId)
            $AWS_CMD attach-internet-gateway --internet-gateway-id $IGW_ID --vpc-id $VPC_ID
  3. Associate route table:
            ROUTE_TABLE_ID=$($AWS_CMD describe-route-tables \
            --filters Name=vpc-id,Values=${VPC_ID} \
            Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
            --query RouteTables[].RouteTableId)
            if [[ -z "${ROUTE_TABLE_ID}" ]]; then
              echo "Creating route table"
              ROUTE_TABLE_ID=$($AWS_CMD create-route-table \
              --vpc-id=${VPC_ID} \
              --query RouteTable.RouteTableId)
              $AWS_CMD create-tags --resources $ROUTE_TABLE_ID --tags \
              Key=KubernetesCluster,Value=$CLUSTER_ID
            fi

            $AWS_CMD associate-route-table --route-table-id $ROUTE_TABLE_ID --subnet-id $SUBNET_ID
            $AWS_CMD create-route --route-table-id $ROUTE_TABLE_ID --destination-cidr-block 0.0.0.0/0 --gateway-id $IGW_ID
19. Create security groups
            MASTER_SG_ID=$(get_security_group_id "kubernetes-master-${CLUSTER_ID}")
            if [[ -z "${MASTER_SG_ID}" ]]; then
              echo "Creating master security group."
              create-security-group "kubernetes-master-${CLUSTER_ID}" \
              "Kubernetes security group applied to master nodes"
            fi
            NODE_SG_ID=$(get_security_group_id "kubernetes-minion-${CLUSTER_ID}")
            if [[ -z "${NODE_SG_ID}" ]]; then
              echo "Creating minion security group."
              create-security-group "kubernetes-minion-${CLUSTER_ID}" \
              "Kubernetes security group applied to minion nodes"
            fi
            MASTER_SG_ID=$(get_security_group_id "kubernetes-master-${CLUSTER_ID}")
            NODE_SG_ID=$(get_security_group_id "kubernetes-minion-${CLUSTER_ID}")
20. Configure security groups:
            $AWS_CMD authorize-security-group-ingress --group-id "${MASTER_SG_ID}" --source-group \
            ${MASTER_SG_ID} --protocol all
            $AWS_CMD authorize-security-group-ingress --group-id "${NODE_SG_ID}" --source-group \
            ${NODE_SG_ID} --protocol all
            $AWS_CMD authorize-security-group-ingress --group-id "${MASTER_SG_ID}" --source-group \
            ${NODE_SG_ID} --protocol all
            $AWS_CMD authorize-security-group-ingress --group-id "${NODE_SG_ID}" --source-group \
            ${MASTER_SG_ID} --protocol all
            $AWS_CMD authorize-security-group-ingress --group-id "${MASTER_SG_ID}" \
            --protocol tcp --port 22 --cidr 0.0.0.0/0
            $AWS_CMD authorize-security-group-ingress --group-id "${NODE_SG_ID}" \
            --protocol tcp --port 22 --cidr 0.0.0.0/0
            $AWS_CMD authorize-security-group-ingress --group-id "${MASTER_SG_ID}" \
            --protocol tcp --port 443 --cidr 0.0.0.0/0
21. Start master:
  1. Ensure master persistent volume:
            local name=${MASTER_NAME}-pd
            if [[ -z "${MASTER_DISK_ID}" ]]; then
              MASTER_DISK_ID=`$AWS_CMD describe-volumes \
              --filters Name=availability-zone,Values=${ZONE} \
              Name=tag:Name,Values=${name} \
              Name=tag:KubernetesCluster,Values=${CLUSTER_ID} \
              --query Volumes[].VolumeId`
            fi
            if [[ -z "${MASTER_DISK_ID}" ]]; then
              MASTER_DISK_ID=`$AWS_CMD create-volume --availability-zone ${ZONE} \
              --volume-type ${MASTER_DISK_TYPE} --size ${MASTER_DISK_SIZE} --query VolumeId`
              $AWS_CMD create-tags --resources $MASTER_DISK_ID --tags \
              Key=NAME,Value=${name}
              $AWS_CMD create-tags --resources $MASTER_DISK_ID --tags \
              Key=KubernetesCluster,Value=${CLUSTER_ID}
            fi
  2. Ensure master elastic IP
            if [[ -n "${MASTER_DISK_ID:-}" ]]; then
              $AWS_CMD describe-tags --filters Name=resource-id,Values=${MASTER_DISK_ID} \
              Name=key,Values=${TAG_KEY_MASTER_IP} \
              --query Tags[].Value
            fi
            if [[ -z "${KUBE_MASTER_IP:-}" ]]; then
              # Check if MASTER_RESERVED_IP looks like an IPv4 address
              # Note that we used to only allocate an elastic IP when MASTER_RESERVED_IP=auto
              # So be careful changing the IPV4 test, to be sure that 'auto' => 'allocate'
              if [[ "${MASTER_RESERVED_IP}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                KUBE_MASTER_IP="${MASTER_RESERVED_IP}"
              else
                KUBE_MASTER_IP=`$AWS_CMD allocate-address --domain vpc --query PublicIp`
              fi

              # We can't tag elastic ips.  Instead we put the tag on the persistent disk.
              # It is a little weird, perhaps, but it sort of makes sense...
              # The master mounts the master PD, and whoever mounts the master PD should also
              # have the master IP
              $AWS_CMD create-tags --resources $MASTER_DISK_ID --tags \
              Key=${TAG_KEY_MASTER_IP},Value=${KUBE_MASTER_IP}
            fi
  3. Create certificates
            create-certs "${KUBE_MASTER_IP}" "${MASTER_INTERNAL_IP}"
  4. Write master environment
            local master=true
            local file="${KUBE_TEMP}/master-kube-env.yaml"

            rm -f ${file}
            cat >$file <<EOF
            ENV_TIMESTAMP: $(yaml-quote $(date -u +%Y-%m-%dT%T%z))
            INSTANCE_PREFIX: $(yaml-quote ${INSTANCE_PREFIX})
            NODE_INSTANCE_PREFIX: $(yaml-quote ${NODE_INSTANCE_PREFIX})
            CLUSTER_IP_RANGE: $(yaml-quote ${CLUSTER_IP_RANGE:-10.244.0.0/16})
            SERVER_BINARY_TAR_URL: $(yaml-quote ${SERVER_BINARY_TAR_URL})
            SERVER_BINARY_TAR_HASH: $(yaml-quote ${SERVER_BINARY_TAR_HASH})
            SALT_TAR_URL: $(yaml-quote ${SALT_TAR_URL})
            SALT_TAR_HASH: $(yaml-quote ${SALT_TAR_HASH})
            SERVICE_CLUSTER_IP_RANGE: $(yaml-quote ${SERVICE_CLUSTER_IP_RANGE})
            KUBERNETES_MASTER_NAME: $(yaml-quote ${MASTER_NAME})
            ALLOCATE_NODE_CIDRS: $(yaml-quote ${ALLOCATE_NODE_CIDRS:-false})
            ENABLE_CLUSTER_MONITORING: $(yaml-quote ${ENABLE_CLUSTER_MONITORING:-none})
            ENABLE_L7_LOADBALANCING: $(yaml-quote ${ENABLE_L7_LOADBALANCING:-none})
            ENABLE_CLUSTER_LOGGING: $(yaml-quote ${ENABLE_CLUSTER_LOGGING:-false})
            ENABLE_CLUSTER_UI: $(yaml-quote ${ENABLE_CLUSTER_UI:-false})
            ENABLE_NODE_LOGGING: $(yaml-quote ${ENABLE_NODE_LOGGING:-false})
            LOGGING_DESTINATION: $(yaml-quote ${LOGGING_DESTINATION:-})
            ELASTICSEARCH_LOGGING_REPLICAS: $(yaml-quote ${ELASTICSEARCH_LOGGING_REPLICAS:-})
            ENABLE_CLUSTER_DNS: $(yaml-quote ${ENABLE_CLUSTER_DNS:-false})
            ENABLE_CLUSTER_REGISTRY: $(yaml-quote ${ENABLE_CLUSTER_REGISTRY:-false})
            CLUSTER_REGISTRY_DISK: $(yaml-quote ${CLUSTER_REGISTRY_DISK:-})
            CLUSTER_REGISTRY_DISK_SIZE: $(yaml-quote ${CLUSTER_REGISTRY_DISK_SIZE:-})
            DNS_REPLICAS: $(yaml-quote ${DNS_REPLICAS:-})
            DNS_SERVER_IP: $(yaml-quote ${DNS_SERVER_IP:-})
            DNS_DOMAIN: $(yaml-quote ${DNS_DOMAIN:-})
            KUBELET_TOKEN: $(yaml-quote ${KUBELET_TOKEN:-})
            KUBE_PROXY_TOKEN: $(yaml-quote ${KUBE_PROXY_TOKEN:-})
            ADMISSION_CONTROL: $(yaml-quote ${ADMISSION_CONTROL:-})
            MASTER_IP_RANGE: $(yaml-quote ${MASTER_IP_RANGE})
            RUNTIME_CONFIG: $(yaml-quote ${RUNTIME_CONFIG})
            CA_CERT: $(yaml-quote ${CA_CERT_BASE64:-})
            KUBELET_CERT: $(yaml-quote ${KUBELET_CERT_BASE64:-})
            KUBELET_KEY: $(yaml-quote ${KUBELET_KEY_BASE64:-})
            NETWORK_PROVIDER: $(yaml-quote ${NETWORK_PROVIDER:-})
            HAIRPIN_MODE: $(yaml-quote ${HAIRPIN_MODE:-})
            OPENCONTRAIL_TAG: $(yaml-quote ${OPENCONTRAIL_TAG:-})
            OPENCONTRAIL_KUBERNETES_TAG: $(yaml-quote ${OPENCONTRAIL_KUBERNETES_TAG:-})
            OPENCONTRAIL_PUBLIC_SUBNET: $(yaml-quote ${OPENCONTRAIL_PUBLIC_SUBNET:-})
            E2E_STORAGE_TEST_ENVIRONMENT: $(yaml-quote ${E2E_STORAGE_TEST_ENVIRONMENT:-})
            KUBE_IMAGE_TAG: $(yaml-quote ${KUBE_IMAGE_TAG:-})
            KUBE_DOCKER_REGISTRY: $(yaml-quote ${KUBE_DOCKER_REGISTRY:-})
            KUBE_ADDON_REGISTRY: $(yaml-quote ${KUBE_ADDON_REGISTRY:-})
            MULTIZONE: $(yaml-quote ${MULTIZONE:-})
            NON_MASQUERADE_CIDR: $(yaml-quote ${NON_MASQUERADE_CIDR:-})
            EOF
            if [ -n "${KUBELET_PORT:-}" ]; then
              cat >>$file <<EOF
            KUBELET_PORT: $(yaml-quote ${KUBELET_PORT})
            EOF
            fi
            if [ -n "${KUBE_APISERVER_REQUEST_TIMEOUT:-}" ]; then
              cat >>$file <<EOF
            KUBE_APISERVER_REQUEST_TIMEOUT: $(yaml-quote ${KUBE_APISERVER_REQUEST_TIMEOUT})
            EOF
            fi
            if [ -n "${TERMINATED_POD_GC_THRESHOLD:-}" ]; then
              cat >>$file <<EOF
            TERMINATED_POD_GC_THRESHOLD: $(yaml-quote ${TERMINATED_POD_GC_THRESHOLD})
            EOF
            fi
            if [[ "${OS_DISTRIBUTION}" == "trusty" ]]; then
              cat >>$file <<EOF
            KUBE_MANIFESTS_TAR_URL: $(yaml-quote ${KUBE_MANIFESTS_TAR_URL})
            KUBE_MANIFESTS_TAR_HASH: $(yaml-quote ${KUBE_MANIFESTS_TAR_HASH})
            EOF
            fi
            if [ -n "${TEST_CLUSTER:-}" ]; then
              cat >>$file <<EOF
            TEST_CLUSTER: $(yaml-quote ${TEST_CLUSTER})
            EOF
            fi
            if [ -n "${KUBELET_TEST_ARGS:-}" ]; then
              cat >>$file <<EOF
            KUBELET_TEST_ARGS: $(yaml-quote ${KUBELET_TEST_ARGS})
            EOF
            fi
            if [ -n "${KUBELET_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            KUBELET_TEST_LOG_LEVEL: $(yaml-quote ${KUBELET_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${DOCKER_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            DOCKER_TEST_LOG_LEVEL: $(yaml-quote ${DOCKER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${ENABLE_CUSTOM_METRICS:-}" ]; then
              cat >>$file <<EOF
            ENABLE_CUSTOM_METRICS: $(yaml-quote ${ENABLE_CUSTOM_METRICS})
            EOF
            fi
            if [[ "${master}" == "true" ]]; then
              # Master-only env vars.
              cat >>$file <<EOF
            KUBERNETES_MASTER: $(yaml-quote "true")
            KUBE_USER: $(yaml-quote ${KUBE_USER})
            KUBE_PASSWORD: $(yaml-quote ${KUBE_PASSWORD})
            KUBE_BEARER_TOKEN: $(yaml-quote ${KUBE_BEARER_TOKEN})
            MASTER_CERT: $(yaml-quote ${MASTER_CERT_BASE64:-})
            MASTER_KEY: $(yaml-quote ${MASTER_KEY_BASE64:-})
            KUBECFG_CERT: $(yaml-quote ${KUBECFG_CERT_BASE64:-})
            KUBECFG_KEY: $(yaml-quote ${KUBECFG_KEY_BASE64:-})
            KUBELET_APISERVER: $(yaml-quote ${KUBELET_APISERVER:-})
            ENABLE_MANIFEST_URL: $(yaml-quote ${ENABLE_MANIFEST_URL:-false})
            MANIFEST_URL: $(yaml-quote ${MANIFEST_URL:-})
            MANIFEST_URL_HEADER: $(yaml-quote ${MANIFEST_URL_HEADER:-})
            NUM_NODES: $(yaml-quote ${NUM_NODES})
            EOF
            if [ -n "${APISERVER_TEST_ARGS:-}" ]; then
              cat >>$file <<EOF
            APISERVER_TEST_ARGS: $(yaml-quote ${APISERVER_TEST_ARGS})
            EOF
            fi
            if [ -n "${APISERVER_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            APISERVER_TEST_LOG_LEVEL: $(yaml-quote ${APISERVER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${CONTROLLER_MANAGER_TEST_ARGS:-}" ]; then
              cat >>$file <<EOF
            CONTROLLER_MANAGER_TEST_ARGS: $(yaml-quote ${CONTROLLER_MANAGER_TEST_ARGS})
            EOF
            fi
            if [ -n "${CONTROLLER_MANAGER_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            CONTROLLER_MANAGER_TEST_LOG_LEVEL: $(yaml-quote ${CONTROLLER_MANAGER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${SCHEDULER_TEST_ARGS:-}" ]; then
              cat >>$file <<EOF
            SCHEDULER_TEST_ARGS: $(yaml-quote ${SCHEDULER_TEST_ARGS})
            EOF
            fi
            if [ -n "${SCHEDULER_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            SCHEDULER_TEST_LOG_LEVEL: $(yaml-quote ${SCHEDULER_TEST_LOG_LEVEL})
            EOF
            fi
            else
            # Node-only env vars.
            cat >>$file <<EOF
            KUBERNETES_MASTER: $(yaml-quote "false")
            ZONE: $(yaml-quote ${ZONE})
            EXTRA_DOCKER_OPTS: $(yaml-quote ${EXTRA_DOCKER_OPTS:-})
            EOF
            if [ -n "${KUBEPROXY_TEST_ARGS:-}" ]; then
              cat >>$file <<EOF
            KUBEPROXY_TEST_ARGS: $(yaml-quote ${KUBEPROXY_TEST_ARGS})
            EOF
            fi
            if [ -n "${KUBEPROXY_TEST_LOG_LEVEL:-}" ]; then
              cat >>$file <<EOF
            KUBEPROXY_TEST_LOG_LEVEL: $(yaml-quote ${KUBEPROXY_TEST_LOG_LEVEL})
            EOF
            fi
            fi
            if [ -n "${NODE_LABELS:-}" ]; then
              cat >>$file <<EOF
            NODE_LABELS: $(yaml-quote ${NODE_LABELS})
            EOF
            fi
            if [[ "${OS_DISTRIBUTION}" == "coreos" ]]; then
              # CoreOS-only env vars. TODO(yifan): Make them available on other distros.
              cat >>$file <<EOF
            KUBE_MANIFESTS_TAR_URL: $(yaml-quote ${KUBE_MANIFESTS_TAR_URL})
            KUBE_MANIFESTS_TAR_HASH: $(yaml-quote ${KUBE_MANIFESTS_TAR_HASH})
            KUBERNETES_CONTAINER_RUNTIME: $(yaml-quote ${CONTAINER_RUNTIME:-docker})
            RKT_VERSION: $(yaml-quote ${RKT_VERSION:-})
            RKT_PATH: $(yaml-quote ${RKT_PATH:-})
            KUBERNETES_CONFIGURE_CBR0: $(yaml-quote ${KUBERNETES_CONFIGURE_CBR0:-true})
            EOF
            fi
  5. Write master user data:
            (
              # We pipe this to the ami as a startup script in the user-data field. Requires a
              # compatible ami
              echo "#! /bin/bash"
              echo "mkdir -p /var/cache/kubernetes-install"
              echo "cd /var/cache/kubernetes-install"

              echo "cat > kube_env.yaml << __EOF_MASTER_KUBE_ENV_YAML"
              cat ${KUBE_TEMP}/master-kube-env.yaml
              echo "AUTO_UPGRADE: 'true'"
              # TODO: get rid of these exceptions / harmonize with common or GCE
              echo "DOCKER_STORAGE: $(yaml-quote ${DOCKER_STORAGE:-})"
              echo "API_SERVERS: $(yaml-quote ${MASTER_INTERNAL_IP:-})"
              echo "__EOF_MASTER_KUBE_ENV_YAML"
              echo ""
              echo "wget -O bootstrap ${BOOTSTRAP_SCRIPT_URL}"
              echo "chmod +x bootstrap"
              echo "mkdir -p /etc/kubernetes"
              echo "mv kube_env.yaml /etc/kubernetes"
              echo "mv bootstrap /etc/kubernetes/"
              echo "cat > /etc/rc.local << EOF_RC_LOCAL"
              echo "#!/bin/sh -e"
              # We want to be sure that we don't pass an argument to bootstrap
              echo "/etc/kubernetes/bootstrap"
              echo "exit 0"
              echo "EOF_RC_LOCAL"
              echo "/etc/kubernetes/bootstrap"
            ) > "${KUBE_TEMP}/master-user-data"

            # Compress the data to fit under the 16KB limit (cloud-init accepts compressed data)
            gzip "${KUBE_TEMP}/master-user-data"
  6. Start master
            master_id=$($AWS_CMD run-instances \
            --image-id $AWS_IMAGE \
            --iam-instance-profile Name=$IAM_PROFILE_MASTER \
            --instance-type $MASTER_SIZE \
            --subnet-id $SUBNET_ID \
            --private-ip-address $MASTER_INTERNAL_IP \
            --key-name ${AWS_SSH_KEY_NAME} \
            --security-group-ids ${MASTER_SG_ID} \
            --associate-public-ip-address \
            --block-device-mappings "${MASTER_BLOCK_DEVICE_MAPPINGS}" \
            --user-data fileb://${KUBE_TEMP}/master-user-data.gz \
            --query Instances[].InstanceId)
            $AWS_CMD create-tags --resources $master_id --tags \
            Key=Name,Value=$MASTER_NAME
            $AWS_CMD create-tags --resources $master_id --tags \
            Key=Role,Value=$MASTER_TAG
            $AWS_CMD create-tags --resources $master_id --tags \
            Key=KubernetesCluster,Value=$CLUSTER_ID

            local attempt=0
            while true; do
              echo -n Attempt "$(($attempt+1))" to check for master node
              local ip=$(get_instance_public_ip ${master_id})
              if [[ -z "${ip}" ]]; then
                if (( attempt > 30 )); then
                  echo
                  echo -e "${color_red}master failed to start. Your cluster is unlikely" >&2
                  echo "to work correctly. Please run ./cluster/kube-down.sh and re-create the" >&2
                  echo -e "cluster. (sorry!)${color_norm}" >&2
                  exit 1
                fi
              else
                # We are not able to add an elastic ip, a route or volume to the instance until that instance is in "running" state.
                wait-for-instance-state ${master_id} "running"

                KUBE_MASTER=${MASTER_NAME}
                echo -e " ${color_green}[master running]${color_norm}"

                attach-ip-to-instance ${KUBE_MASTER_IP} ${master_id}

                # This is a race between instance start and volume attachment.  There appears to be no way to start an AWS instance with a volume attached.
                # To work around this, we wait for volume to be ready in setup-master-pd.sh
                echo "Attaching persistent data volume (${MASTER_DISK_ID}) to master"
                $AWS_CMD attach-volume --volume-id ${MASTER_DISK_ID} --device /dev/sdb --instance-id ${master_id}

                sleep 10
                $AWS_CMD create-route --route-table-id $ROUTE_TABLE_ID --destination-cidr-block ${MASTER_IP_RANGE} --instance-id $master_id > $LOG

                break
              fi
              echo -e " ${color_yellow}[master not working yet]${color_norm}"
              attempt=$(($attempt+1))
              sleep 10
            done
  7. Write kubeconfig
            local kubectl="${KUBE_ROOT}/cluster/kubectl.sh"

            export KUBECONFIG=${KUBECONFIG:-$DEFAULT_KUBECONFIG}
            # KUBECONFIG determines the file we write to, but it may not exist yet
            if [[ ! -e "${KUBECONFIG}" ]]; then
            mkdir -p $(dirname "${KUBECONFIG}")
            touch "${KUBECONFIG}"
            fi
            local cluster_args=(
            "--server=${KUBE_SERVER:-https://${KUBE_MASTER_IP}}"
            )
            if [[ -z "${CA_CERT:-}" ]]; then
            cluster_args+=("--insecure-skip-tls-verify=true")
            else
            cluster_args+=(
            "--certificate-authority=${CA_CERT}"
            "--embed-certs=true"
            )
            fi

            local user_args=()
            if [[ ! -z "${KUBE_BEARER_TOKEN:-}" ]]; then
            user_args+=(
            "--token=${KUBE_BEARER_TOKEN}"
            )
            elif [[ ! -z "${KUBE_USER:-}" && ! -z "${KUBE_PASSWORD:-}" ]]; then
            user_args+=(
            "--username=${KUBE_USER}"
            "--password=${KUBE_PASSWORD}"
            )
            fi
            if [[ ! -z "${KUBE_CERT:-}" && ! -z "${KUBE_KEY:-}" ]]; then
            user_args+=(
            "--client-certificate=${KUBE_CERT}"
            "--client-key=${KUBE_KEY}"
            "--embed-certs=true"
            )
            fi

            "${kubectl}" config set-cluster "${CONTEXT}" "${cluster_args[@]}"
            if [[ -n "${user_args[@]:-}" ]]; then
            "${kubectl}" config set-credentials "${CONTEXT}" "${user_args[@]}"
            fi
            "${kubectl}" config set-context "${CONTEXT}" --cluster="${CONTEXT}" --user="${CONTEXT}"
            "${kubectl}" config use-context "${CONTEXT}"  --cluster="${CONTEXT}"

            # If we have a bearer token, also create a credential entry with basic auth
            # so that it is easy to discover the basic auth password for your cluster
            # to use in a web browser.
            if [[ ! -z "${KUBE_BEARER_TOKEN:-}" && ! -z "${KUBE_USER:-}" && ! -z "${KUBE_PASSWORD:-}" ]]; then
            "${kubectl}" config set-credentials "${CONTEXT}-basic-auth" "--username=${KUBE_USER}" "--password=${KUBE_PASSWORD}"
            fi
  8. Start minions
    1. Write node env
            local master=false
            local file=${KUBE_TEMP}/node-kube-env.yaml

            build-runtime-config

            rm -f ${file}
            cat >$file <<EOF
            ENV_TIMESTAMP: $(yaml-quote $(date -u +%Y-%m-%dT%T%z))
            INSTANCE_PREFIX: $(yaml-quote ${INSTANCE_PREFIX})
            NODE_INSTANCE_PREFIX: $(yaml-quote ${NODE_INSTANCE_PREFIX})
            CLUSTER_IP_RANGE: $(yaml-quote ${CLUSTER_IP_RANGE:-10.244.0.0/16})
            SERVER_BINARY_TAR_URL: $(yaml-quote ${SERVER_BINARY_TAR_URL})
            SERVER_BINARY_TAR_HASH: $(yaml-quote ${SERVER_BINARY_TAR_HASH})
            SALT_TAR_URL: $(yaml-quote ${SALT_TAR_URL})
            SALT_TAR_HASH: $(yaml-quote ${SALT_TAR_HASH})
            SERVICE_CLUSTER_IP_RANGE: $(yaml-quote ${SERVICE_CLUSTER_IP_RANGE})
            KUBERNETES_MASTER_NAME: $(yaml-quote ${MASTER_NAME})
            ALLOCATE_NODE_CIDRS: $(yaml-quote ${ALLOCATE_NODE_CIDRS:-false})
            ENABLE_CLUSTER_MONITORING: $(yaml-quote ${ENABLE_CLUSTER_MONITORING:-none})
            ENABLE_L7_LOADBALANCING: $(yaml-quote ${ENABLE_L7_LOADBALANCING:-none})
            ENABLE_CLUSTER_LOGGING: $(yaml-quote ${ENABLE_CLUSTER_LOGGING:-false})
            ENABLE_CLUSTER_UI: $(yaml-quote ${ENABLE_CLUSTER_UI:-false})
            ENABLE_NODE_LOGGING: $(yaml-quote ${ENABLE_NODE_LOGGING:-false})
            LOGGING_DESTINATION: $(yaml-quote ${LOGGING_DESTINATION:-})
            ELASTICSEARCH_LOGGING_REPLICAS: $(yaml-quote ${ELASTICSEARCH_LOGGING_REPLICAS:-})
            ENABLE_CLUSTER_DNS: $(yaml-quote ${ENABLE_CLUSTER_DNS:-false})
            ENABLE_CLUSTER_REGISTRY: $(yaml-quote ${ENABLE_CLUSTER_REGISTRY:-false})
            CLUSTER_REGISTRY_DISK: $(yaml-quote ${CLUSTER_REGISTRY_DISK:-})
            CLUSTER_REGISTRY_DISK_SIZE: $(yaml-quote ${CLUSTER_REGISTRY_DISK_SIZE:-})
            DNS_REPLICAS: $(yaml-quote ${DNS_REPLICAS:-})
            DNS_SERVER_IP: $(yaml-quote ${DNS_SERVER_IP:-})
            DNS_DOMAIN: $(yaml-quote ${DNS_DOMAIN:-})
            KUBELET_TOKEN: $(yaml-quote ${KUBELET_TOKEN:-})
            KUBE_PROXY_TOKEN: $(yaml-quote ${KUBE_PROXY_TOKEN:-})
            ADMISSION_CONTROL: $(yaml-quote ${ADMISSION_CONTROL:-})
            MASTER_IP_RANGE: $(yaml-quote ${MASTER_IP_RANGE})
            RUNTIME_CONFIG: $(yaml-quote ${RUNTIME_CONFIG})
            CA_CERT: $(yaml-quote ${CA_CERT_BASE64:-})
            KUBELET_CERT: $(yaml-quote ${KUBELET_CERT_BASE64:-})
            KUBELET_KEY: $(yaml-quote ${KUBELET_KEY_BASE64:-})
            NETWORK_PROVIDER: $(yaml-quote ${NETWORK_PROVIDER:-})
            HAIRPIN_MODE: $(yaml-quote ${HAIRPIN_MODE:-})
            OPENCONTRAIL_TAG: $(yaml-quote ${OPENCONTRAIL_TAG:-})
            OPENCONTRAIL_KUBERNETES_TAG: $(yaml-quote ${OPENCONTRAIL_KUBERNETES_TAG:-})
            OPENCONTRAIL_PUBLIC_SUBNET: $(yaml-quote ${OPENCONTRAIL_PUBLIC_SUBNET:-})
            E2E_STORAGE_TEST_ENVIRONMENT: $(yaml-quote ${E2E_STORAGE_TEST_ENVIRONMENT:-})
            KUBE_IMAGE_TAG: $(yaml-quote ${KUBE_IMAGE_TAG:-})
            KUBE_DOCKER_REGISTRY: $(yaml-quote ${KUBE_DOCKER_REGISTRY:-})
            KUBE_ADDON_REGISTRY: $(yaml-quote ${KUBE_ADDON_REGISTRY:-})
            MULTIZONE: $(yaml-quote ${MULTIZONE:-})
            NON_MASQUERADE_CIDR: $(yaml-quote ${NON_MASQUERADE_CIDR:-})
            EOF
            if [ -n "${KUBELET_PORT:-}" ]; then
            cat >>$file <<EOF
            KUBELET_PORT: $(yaml-quote ${KUBELET_PORT})
            EOF
            fi
            if [ -n "${KUBE_APISERVER_REQUEST_TIMEOUT:-}" ]; then
            cat >>$file <<EOF
            KUBE_APISERVER_REQUEST_TIMEOUT: $(yaml-quote ${KUBE_APISERVER_REQUEST_TIMEOUT})
            EOF
            fi
            if [ -n "${TERMINATED_POD_GC_THRESHOLD:-}" ]; then
            cat >>$file <<EOF
            TERMINATED_POD_GC_THRESHOLD: $(yaml-quote ${TERMINATED_POD_GC_THRESHOLD})
            EOF
            fi
            if [[ "${OS_DISTRIBUTION}" == "trusty" ]]; then
            cat >>$file <<EOF
            KUBE_MANIFESTS_TAR_URL: $(yaml-quote ${KUBE_MANIFESTS_TAR_URL})
            KUBE_MANIFESTS_TAR_HASH: $(yaml-quote ${KUBE_MANIFESTS_TAR_HASH})
            EOF
            fi
            if [ -n "${TEST_CLUSTER:-}" ]; then
            cat >>$file <<EOF
            TEST_CLUSTER: $(yaml-quote ${TEST_CLUSTER})
            EOF
            fi
            if [ -n "${KUBELET_TEST_ARGS:-}" ]; then
            cat >>$file <<EOF
            KUBELET_TEST_ARGS: $(yaml-quote ${KUBELET_TEST_ARGS})
            EOF
            fi
            if [ -n "${KUBELET_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            KUBELET_TEST_LOG_LEVEL: $(yaml-quote ${KUBELET_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${DOCKER_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            DOCKER_TEST_LOG_LEVEL: $(yaml-quote ${DOCKER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${ENABLE_CUSTOM_METRICS:-}" ]; then
            cat >>$file <<EOF
            ENABLE_CUSTOM_METRICS: $(yaml-quote ${ENABLE_CUSTOM_METRICS})
            EOF
            fi
            if [[ "${master}" == "true" ]]; then
            # Master-only env vars.
            cat >>$file <<EOF
            KUBERNETES_MASTER: $(yaml-quote "true")
            KUBE_USER: $(yaml-quote ${KUBE_USER})
            KUBE_PASSWORD: $(yaml-quote ${KUBE_PASSWORD})
            KUBE_BEARER_TOKEN: $(yaml-quote ${KUBE_BEARER_TOKEN})
            MASTER_CERT: $(yaml-quote ${MASTER_CERT_BASE64:-})
            MASTER_KEY: $(yaml-quote ${MASTER_KEY_BASE64:-})
            KUBECFG_CERT: $(yaml-quote ${KUBECFG_CERT_BASE64:-})
            KUBECFG_KEY: $(yaml-quote ${KUBECFG_KEY_BASE64:-})
            KUBELET_APISERVER: $(yaml-quote ${KUBELET_APISERVER:-})
            ENABLE_MANIFEST_URL: $(yaml-quote ${ENABLE_MANIFEST_URL:-false})
            MANIFEST_URL: $(yaml-quote ${MANIFEST_URL:-})
            MANIFEST_URL_HEADER: $(yaml-quote ${MANIFEST_URL_HEADER:-})
            NUM_NODES: $(yaml-quote ${NUM_NODES})
            EOF
            if [ -n "${APISERVER_TEST_ARGS:-}" ]; then
            cat >>$file <<EOF
            APISERVER_TEST_ARGS: $(yaml-quote ${APISERVER_TEST_ARGS})
            EOF
            fi
            if [ -n "${APISERVER_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            APISERVER_TEST_LOG_LEVEL: $(yaml-quote ${APISERVER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${CONTROLLER_MANAGER_TEST_ARGS:-}" ]; then
            cat >>$file <<EOF
            CONTROLLER_MANAGER_TEST_ARGS: $(yaml-quote ${CONTROLLER_MANAGER_TEST_ARGS})
            EOF
            fi
            if [ -n "${CONTROLLER_MANAGER_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            CONTROLLER_MANAGER_TEST_LOG_LEVEL: $(yaml-quote ${CONTROLLER_MANAGER_TEST_LOG_LEVEL})
            EOF
            fi
            if [ -n "${SCHEDULER_TEST_ARGS:-}" ]; then
            cat >>$file <<EOF
            SCHEDULER_TEST_ARGS: $(yaml-quote ${SCHEDULER_TEST_ARGS})
            EOF
            fi
            if [ -n "${SCHEDULER_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            SCHEDULER_TEST_LOG_LEVEL: $(yaml-quote ${SCHEDULER_TEST_LOG_LEVEL})
            EOF
            fi
            else
            # Node-only env vars.
            cat >>$file <<EOF
            KUBERNETES_MASTER: $(yaml-quote "false")
            ZONE: $(yaml-quote ${ZONE})
            EXTRA_DOCKER_OPTS: $(yaml-quote ${EXTRA_DOCKER_OPTS:-})
            EOF
            if [ -n "${KUBEPROXY_TEST_ARGS:-}" ]; then
            cat >>$file <<EOF
            KUBEPROXY_TEST_ARGS: $(yaml-quote ${KUBEPROXY_TEST_ARGS})
            EOF
            fi
            if [ -n "${KUBEPROXY_TEST_LOG_LEVEL:-}" ]; then
            cat >>$file <<EOF
            KUBEPROXY_TEST_LOG_LEVEL: $(yaml-quote ${KUBEPROXY_TEST_LOG_LEVEL})
            EOF
            fi
            fi
            if [ -n "${NODE_LABELS:-}" ]; then
            cat >>$file <<EOF
            NODE_LABELS: $(yaml-quote ${NODE_LABELS})
            EOF
            fi
            if [[ "${OS_DISTRIBUTION}" == "coreos" ]]; then
            # CoreOS-only env vars. TODO(yifan): Make them available on other distros.
            cat >>$file <<EOF
            KUBE_MANIFESTS_TAR_URL: $(yaml-quote ${KUBE_MANIFESTS_TAR_URL})
            KUBE_MANIFESTS_TAR_HASH: $(yaml-quote ${KUBE_MANIFESTS_TAR_HASH})
            KUBERNETES_CONTAINER_RUNTIME: $(yaml-quote ${CONTAINER_RUNTIME:-docker})
            RKT_VERSION: $(yaml-quote ${RKT_VERSION:-})
            RKT_PATH: $(yaml-quote ${RKT_PATH:-})
            KUBERNETES_CONFIGURE_CBR0: $(yaml-quote ${KUBERNETES_CONFIGURE_CBR0:-true})
            EOF
            fi
    2. Start minions with user data
            (
              # We pipe this to the ami as a startup script in the user-data field.  Requires a compatible ami
              echo "#! /bin/bash"
              echo "mkdir -p /var/cache/kubernetes-install"
              echo "cd /var/cache/kubernetes-install"
              echo "cat > kube_env.yaml << __EOF_KUBE_ENV_YAML"
              cat ${KUBE_TEMP}/node-kube-env.yaml
              echo "AUTO_UPGRADE: 'true'"
              # TODO: get rid of these exceptions / harmonize with common or GCE
              echo "DOCKER_STORAGE: $(yaml-quote ${DOCKER_STORAGE:-})"
              echo "API_SERVERS: $(yaml-quote ${MASTER_INTERNAL_IP:-})"
              echo "__EOF_KUBE_ENV_YAML"
              echo ""
              echo "wget -O bootstrap ${BOOTSTRAP_SCRIPT_URL}"
              echo "chmod +x bootstrap"
              echo "mkdir -p /etc/kubernetes"
              echo "mv kube_env.yaml /etc/kubernetes"
              echo "mv bootstrap /etc/kubernetes/"
              echo "cat > /etc/rc.local << EOF_RC_LOCAL"
              echo "#!/bin/sh -e"
              # We want to be sure that we don't pass an argument to bootstrap
              echo "/etc/kubernetes/bootstrap"
              echo "exit 0"
              echo "EOF_RC_LOCAL"
              echo "/etc/kubernetes/bootstrap"
            ) > "${KUBE_TEMP}/node-user-data"

            # Compress the data to fit under the 16KB limit (cloud-init accepts compressed data)
            gzip "${KUBE_TEMP}/node-user-data"

            local public_ip_option
            if [[ "${ENABLE_NODE_PUBLIC_IP}" == "true" ]]; then
              public_ip_option="--associate-public-ip-address"
            else
              public_ip_option="--no-associate-public-ip-address"
            fi
            local spot_price_option
            if [[ -n "${NODE_SPOT_PRICE:-}" ]]; then
              spot_price_option="--spot-price ${NODE_SPOT_PRICE}"
            else
              spot_price_option=""
            fi
            ${AWS_ASG_CMD} create-launch-configuration \
                --launch-configuration-name ${ASG_NAME} \
                --image-id $KUBE_NODE_IMAGE \
                --iam-instance-profile ${IAM_PROFILE_NODE} \
                --instance-type $NODE_SIZE \
                --key-name ${AWS_SSH_KEY_NAME} \
                --security-groups ${NODE_SG_ID} \
                ${public_ip_option} \
                ${spot_price_option} \
                --block-device-mappings "${NODE_BLOCK_DEVICE_MAPPINGS}" \
                --user-data "fileb://${KUBE_TEMP}/node-user-data.gz"

            echo "Creating autoscaling group"
            ${AWS_ASG_CMD} create-auto-scaling-group \
                --auto-scaling-group-name ${ASG_NAME} \
                --launch-configuration-name ${ASG_NAME} \
                --min-size ${NUM_NODES} \
                --max-size ${NUM_NODES} \
                --vpc-zone-identifier ${SUBNET_ID} \
                --tags ResourceId=${ASG_NAME},ResourceType=auto-scaling-group,Key=Name,Value=${NODE_INSTANCE_PREFIX} \
                       ResourceId=${ASG_NAME},ResourceType=auto-scaling-group,Key=Role,Value=${NODE_TAG} \
                       ResourceId=${ASG_NAME},ResourceType=auto-scaling-group,Key=KubernetesCluster,Value=${CLUSTER_ID}

            # Wait for the minions to be running
            # TODO(justinsb): This is really not needed any more
            local attempt=0
            local max_attempts=30
            # Spot instances are slower to launch
            if [[ -n "${NODE_SPOT_PRICE:-}" ]]; then
              max_attempts=90
            fi
            while true; do
              find-running-minions > $LOG
              if [[ ${#NODE_IDS[@]} == ${NUM_NODES} ]]; then
                echo -e " ${color_green}${#NODE_IDS[@]} minions started; ready${color_norm}"
                break
              fi

              if (( attempt > max_attempts )); then
                echo
                echo "Expected number of minions did not start in time"
                echo
                echo -e "${color_red}Expected number of minions failed to start.  Your cluster is unlikely" >&2
                echo "to work correctly. Please run ./cluster/kube-down.sh and re-create the" >&2
                echo -e "cluster. (sorry!)${color_norm}" >&2
                exit 1
              fi

              echo -e " ${color_yellow}${#NODE_IDS[@]} minions started; waiting${color_norm}"
              attempt=$(($attempt+1))
              sleep 10
            done
  9. Wait for master
            find-master-pd
            if [[ -n "${MASTER_DISK_ID:-}" ]]; then
              KUBE_MASTER_IP=$(get-tag ${MASTER_DISK_ID} ${TAG_KEY_MASTER_IP})
            fi
            KUBE_MASTER=${MASTER_NAME}
            if [[ -z "${KUBE_MASTER_IP:-}" ]]; then
              echo "Could not detect Kubernetes master node IP.  Make sure you've launched a cluster with 'kube-up.sh'"
              exit 1
            fi
            until $(curl --insecure --user ${KUBE_USER}:${KUBE_PASSWORD} --max-time 5 \
              --fail --output $LOG --silent https://${KUBE_MASTER_IP}/healthz); do
              printf "."
              sleep 2
            done
  10. Check cluster
            sleep 5

            detect-nodes > $LOG

            # Don't bail on errors, we want to be able to print some info.
            set +e

            # Basic sanity checking
            # TODO(justinsb): This is really not needed any more
            local rc # Capture return code without exiting because of errexit bash option
            for (( i=0; i<${#KUBE_NODE_IP_ADDRESSES[@]}; i++)); do
              # Make sure docker is installed and working.
              local attempt=0
              while true; do
                local minion_ip=${KUBE_NODE_IP_ADDRESSES[$i]}
                echo -n "Attempt $(($attempt+1)) to check Docker on node @ ${minion_ip} ..."
                local output=`check-minion ${minion_ip}`
                echo $output
                if [[ "${output}" != "working" ]]; then
                  if (( attempt > 9 )); then
                    echo
                    echo -e "${color_red}Your cluster is unlikely to work correctly." >&2
                    echo "Please run ./cluster/kube-down.sh and re-create the" >&2
                    echo -e "cluster. (sorry!)${color_norm}" >&2
                    exit 1
                  fi
                else
                  break
                fi
                attempt=$(($attempt+1))
                sleep 30
              done
            done
