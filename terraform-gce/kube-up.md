#Kube-up Algorithm for GCE
1. Detect active GCE project
2. Generate Kube basicauth, which ends up in /srv/kubernetes/basic_auth.csv:
        KUBE_USER=admin
        KUBE_PASSWORD=$(python -c 'import string,random; print("".join(random.SystemRandom().choice(string.ascii_letters + string.digits) for _ in range(16)))')
3. Generate Kube bearer token:
        KUBE_BEARER_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
4. Find release tars:
        SERVER_BINARY_TAR="${KUBE_ROOT}/server/kubernetes-server-linux-amd64.tar.gz"
        if [[ ! -f "${SERVER_BINARY_TAR}" ]]; then
          SERVER_BINARY_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-server-linux-amd64.tar.gz"
        fi
        if [[ ! -f "${SERVER_BINARY_TAR}" ]]; then
          echo "!!! Cannot find kubernetes-server-linux-amd64.tar.gz" >&2
          exit 1
        fi

        SALT_TAR="${KUBE_ROOT}/server/kubernetes-salt.tar.gz"
        if [[ ! -f "${SALT_TAR}" ]]; then
          SALT_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-salt.tar.gz"
        fi
        if [[ ! -f "${SALT_TAR}" ]]; then
          echo "!!! Cannot find kubernetes-salt.tar.gz" >&2
          exit 1
        fi

        # This tarball is only used by Ubuntu Trusty.
        KUBE_MANIFESTS_TAR=
        if [[ "${KUBE_OS_DISTRIBUTION:-}" == "trusty" || "${KUBE_OS_DISTRIBUTION:-}" == "coreos" ]]; then
          KUBE_MANIFESTS_TAR="${KUBE_ROOT}/server/kubernetes-manifests.tar.gz"
          if [[ ! -f "${KUBE_MANIFESTS_TAR}" ]]; then
            KUBE_MANIFESTS_TAR="${KUBE_ROOT}/_output/release-tars/kubernetes-manifests.tar.gz"
          fi
          if [[ ! -f "${KUBE_MANIFESTS_TAR}" ]]; then
            echo "!!! Cannot find kubernetes-manifests.tar.gz" >&2
            exit 1
          fi
        fi
5. Upload server tars to gs://kubernetes-staging-$hash-eu/kubernetes-devel:
        SERVER_BINARY_TAR_URL=
        SERVER_BINARY_TAR_HASH=
        SALT_TAR_URL=
        SALT_TAR_HASH=
        KUBE_MANIFESTS_TAR_URL=
        KUBE_MANIFESTS_TAR_HASH=

        local project_hash
        if which md5 > /dev/null 2>&1; then
        project_hash=$(md5 -q -s "$PROJECT")
        else
        project_hash=$(echo -n "$PROJECT" | md5sum | awk '{ print $1 }')
        fi

        # This requires 1 million projects before the probability of collision is 50%
        # that's probably good enough for now :P
        project_hash=${project_hash:0:10}

        set-preferred-region

        SERVER_BINARY_TAR_HASH=$(sha1sum-file "${SERVER_BINARY_TAR}")
        SALT_TAR_HASH=$(sha1sum-file "${SALT_TAR}")
        if [[ "${OS_DISTRIBUTION}" == "trusty" || "${OS_DISTRIBUTION}" == "coreos" ]]; then
        KUBE_MANIFESTS_TAR_HASH=$(sha1sum-file "${KUBE_MANIFESTS_TAR}")
        fi

        local server_binary_tar_urls=()
        local salt_tar_urls=()
        local kube_manifest_tar_urls=()

        for region in "${PREFERRED_REGION[@]}"; do
          suffix="-${region}"
          if [[ "${suffix}" == "-us" ]]; then
            suffix=""
          fi
          local staging_bucket="gs://kubernetes-staging-${project_hash}${suffix}"

          # Ensure the buckets are created
          if ! gsutil ls "${staging_bucket}" > /dev/null 2>&1 ; then
            echo "Creating ${staging_bucket}"
            gsutil mb -l "${region}" "${staging_bucket}"
          fi

          local staging_path="${staging_bucket}/${INSTANCE_PREFIX}-devel"

          echo "+++ Staging server tars to Google Storage: ${staging_path}"
          local server_binary_gs_url="${staging_path}/${SERVER_BINARY_TAR##*/}"
          local salt_gs_url="${staging_path}/${SALT_TAR##*/}"
          copy-to-staging "${staging_path}" "${server_binary_gs_url}" "${SERVER_BINARY_TAR}" "${SERVER_BINARY_TAR_HASH}"
          copy-to-staging "${staging_path}" "${salt_gs_url}" "${SALT_TAR}" "${SALT_TAR_HASH}"

          # Convert from gs:// URL to an https:// URL
          server_binary_tar_urls+=("${server_binary_gs_url/gs:\/\//https://storage.googleapis.com/}")
          salt_tar_urls+=("${salt_gs_url/gs:\/\//https://storage.googleapis.com/}")

          if [[ "${OS_DISTRIBUTION}" == "trusty" || "${OS_DISTRIBUTION}" == "coreos" ]]; then
            local kube_manifests_gs_url="${staging_path}/${KUBE_MANIFESTS_TAR##*/}"
            copy-to-staging "${staging_path}" "${kube_manifests_gs_url}" "${KUBE_MANIFESTS_TAR}" "${KUBE_MANIFESTS_TAR_HASH}"
            # Convert from gs:// URL to an https:// URL
            kube_manifests_tar_urls+=("${kube_manifests_gs_url/gs:\/\//https://storage.googleapis.com/}")
          fi
        done

        if [[ "${OS_DISTRIBUTION}" == "trusty" || "${OS_DISTRIBUTION}" == "coreos" ]]; then
          # TODO: Support fallback .tar.gz settings on CoreOS/Trusty
          SERVER_BINARY_TAR_URL="${server_binary_tar_urls[0]}"
          SALT_TAR_URL="${salt_tar_urls[0]}"
          KUBE_MANIFESTS_TAR_URL="${kube_manifests_tar_urls[0]}"
        else
          SERVER_BINARY_TAR_URL=$(join_csv "${server_binary_tar_urls[@]}")
          SALT_TAR_URL=$(join_csv "${salt_tar_urls[@]}")
        fi
6. Set number of node instance groups:
        local defaulted_max_instances_per_mig=${MAX_INSTANCES_PER_MIG:-1000}

        if [[ ${defaulted_max_instances_per_mig} -le "0" ]]; then
          echo "MAX_INSTANCES_PER_MIG cannot be negative. Assuming default 1000"
          defaulted_max_instances_per_mig=1000
        fi
        export NUM_MIGS=$(((${NUM_NODES} + ${defaulted_max_instances_per_mig} - 1) / ${defaulted_max_instances_per_mig}))
7. Run kube-down if existing Kubernetes-created resources detected
8. Create network (default: 'default') if not already existing
9. Create firewall rules for network:
        if ! gcloud compute firewall-rules --project "${PROJECT}" describe "${NETWORK}-default-internal" &>/dev/null; then
          gcloud compute firewall-rules create "${NETWORK}-default-internal" \
            --project "${PROJECT}" \
            --network "${NETWORK}" \
            --source-ranges "10.0.0.0/8" \
            --allow "tcp:1-65535,udp:1-65535,icmp" &
        fi

        if ! gcloud compute firewall-rules describe --project "${PROJECT}" "${NETWORK}-default-ssh" &>/dev/null; then
          gcloud compute firewall-rules create "${NETWORK}-default-ssh" \
            --project "${PROJECT}" \
            --network "${NETWORK}" \
            --source-ranges "0.0.0.0/0" \
            --allow "tcp:22" &
        fi
10. Write cluster name (default: 'Kubernetes'):
        cat >"${KUBE_TEMP}/cluster-name.txt" << EOF
        ${CLUSTER_NAME}
        EOF
11. Create master instance:
        gcloud compute firewall-rules create "kubernetes-master-https" \
          --project "${PROJECT}" \
          --network "${NETWORK}" \
          --target-tags "${MASTER_TAG}" \
          --allow tcp:443 &

        # We have to make sure the disk is created before creating the master VM, so
        # run this in the foreground.
        gcloud compute disks create "${MASTER_NAME}-pd" \
          --project "${PROJECT}" \
          --zone "${ZONE}" \
          --type "${MASTER_DISK_TYPE}" \
          --size "${MASTER_DISK_SIZE}"

        # Create disk for cluster registry if enabled
        if [[ "${ENABLE_CLUSTER_REGISTRY}" == true && -n "${CLUSTER_REGISTRY_DISK}" ]]; then
          gcloud compute disks create "${CLUSTER_REGISTRY_DISK}" \
            --project "${PROJECT}" \
            --zone "${ZONE}" \
            --type "${CLUSTER_REGISTRY_DISK_TYPE_GCE}" \
            --size "${CLUSTER_REGISTRY_DISK_SIZE}" &
        fi

        # Generate a bearer token for this cluster. We push this separately
        # from the other cluster variables so that the client (this
        # computer) can forget it later. This should disappear with
        # http://issue.k8s.io/3168
        KUBELET_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
        KUBE_PROXY_TOKEN=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)

        # Reserve the master's IP so that it can later be transferred to another VM
        # without disrupting the kubelets. IPs are associated with regions, not zones,
        # so extract the region name, which is the same as the zone but with the final
        # dash and characters trailing the dash removed.
        local REGION=${ZONE%-*}
        create-static-ip "${MASTER_NAME}-ip" "${REGION}"

        MASTER_RESERVED_IP=$(gcloud compute addresses describe "${MASTER_NAME}-ip" \
          --project "${PROJECT}" \
          --region "${REGION}" -q --format yaml | awk '/^address:/ { print $2 }')

        create-certs "${MASTER_RESERVED_IP}"

        local preemptible_master=""
        if [[ "${PREEMPTIBLE_MASTER:-}" == "true" ]]; then
          preemptible_master="--preemptible --maintenance-policy TERMINATE"
        fi

        write-master-env
        gcloud compute instances create "${MASTER_NAME}" \
        --address ${MASTER_RESERVED_IP} \
        --project "${PROJECT}" \
        --zone "${ZONE}" \
        --machine-type "${MASTER_SIZE}" \
        --image-project="${MASTER_IMAGE_PROJECT}" \
        --image "${MASTER_IMAGE}" \
        --tags "${MASTER_TAG}" \
        --network "${NETWORK}" \
        --scopes "storage-ro,compute-rw,monitoring,logging-write" \
        --can-ip-forward \
        --metadata-from-file \
        "kube-env=${KUBE_TEMP}/master-kube-env.yaml,user-data=${KUBE_ROOT}/cluster/gce/coreos/master.yaml,configure-node=${KUBE_ROOT}/cluster/gce/coreos/configure-node.sh,configure-kubelet=${KUBE_ROOT}/cluster/gce/coreos/configure-kubelet.sh,cluster-name=${KUBE_TEMP}/cluster-name.txt" \
        --disk "name=${MASTER_NAME}-pd,device-name=master-pd,mode=rw,boot=no,auto-delete=no" \
        ${preemptible_master}
12. Create nodes firewall:
          # Create a single firewall rule for all minions.
          create-firewall-rule "${NODE_TAG}-all" "${CLUSTER_IP_RANGE}" "${NODE_TAG}" &

          # Report logging choice (if any).
          if [[ "${ENABLE_NODE_LOGGING-}" == "true" ]]; then
          echo "+++ Logging using Fluentd to ${LOGGING_DESTINATION:-unknown}"
          fi

          # Wait for last batch of jobs
          kube::util::wait-for-jobs || {
          echo -e "${color_red}Some commands failed.${color_norm}" >&2
          }
13. Create nodes template:
        echo "Creating minions."

        # TODO(zmerlynn): Refactor setting scope flags.
        local scope_flags=
        if [ -n "${NODE_SCOPES}" ]; then
          scope_flags="--scopes ${NODE_SCOPES}"
        else
          scope_flags="--no-scopes"
        fi

        write-node-env

        local template_name="${NODE_INSTANCE_PREFIX}-template"

        # For master on trusty, we support running nodes on ContainerVM or trusty.
        if [[ "${OS_DISTRIBUTION}" == "trusty" ]] && \
           [[ "${NODE_IMAGE}" == container* ]]; then
          source "${KUBE_ROOT}/cluster/gce/debian/helper.sh"
        fi

        create-node-template "kubernetes-minion-template" "--scopes ${NODE_SCOPES}" \
          "kube-env=${KUBE_TEMP}/node-kube-env.yaml" \
          "user-data=${KUBE_ROOT}/cluster/gce/coreos/node.yaml" \
          "configure-node=${KUBE_ROOT}/cluster/gce/coreos/configure-node.sh" \
          "configure-kubelet=${KUBE_ROOT}/cluster/gce/coreos/configure-kubelet.sh" \
          "cluster-name=${KUBE_TEMP}/cluster-name.txt"
14. Create node instances
        local template_name="kubernetes-minion-template"

        local instances_per_mig=$(((${NUM_NODES} + ${NUM_MIGS} - 1) / ${NUM_MIGS}))
        local last_mig_size=$((${NUM_NODES} - (${NUM_MIGS} - 1) * ${instances_per_mig}))

        #TODO: parallelize this loop to speed up the process
        for ((i=1; i<${NUM_MIGS}; i++)); do
          gcloud compute instance-groups managed \
            create "kubernetes-minion-group-$i" \
            --project "${PROJECT}" \
            --zone "${ZONE}" \
            --base-instance-name kubernetes-minion \
            --size "${instances_per_mig}" \
            --template "$template_name" || true;
            gcloud compute instance-groups managed wait-until-stable \
            "${NODE_INSTANCE_PREFIX}-group-$i" \
            --zone "${ZONE}" \
            --project "${PROJECT}" || true;
        done

        # TODO: We don't add a suffix for the last group to keep backward compatibility when there's only one MIG.
        # We should change it at some point, but note #18545 when changing this.
        gcloud compute instance-groups managed \
        create "${NODE_INSTANCE_PREFIX}-group" \
        --project "${PROJECT}" \
        --zone "${ZONE}" \
        --base-instance-name "${NODE_INSTANCE_PREFIX}" \
        --size "${last_mig_size}" \
        --template "$template_name" || true;
        gcloud compute instance-groups managed wait-until-stable \
        "${NODE_INSTANCE_PREFIX}-group" \
        --zone "${ZONE}" \
        --project "${PROJECT}" || true;
15. Optionally create autoscaler
        # Create autoscaler for nodes if requested
        if [[ "${ENABLE_NODE_AUTOSCALER}" == "true" ]]; then
          local metrics=""
          # Current usage
          metrics+="--custom-metric-utilization metric=custom.cloudmonitoring.googleapis.com/kubernetes.io/cpu/node_utilization,"
          metrics+="utilization-target=${TARGET_NODE_UTILIZATION},utilization-target-type=GAUGE "
          metrics+="--custom-metric-utilization metric=custom.cloudmonitoring.googleapis.com/kubernetes.io/memory/node_utilization,"
          metrics+="utilization-target=${TARGET_NODE_UTILIZATION},utilization-target-type=GAUGE "

          # Reservation
          metrics+="--custom-metric-utilization metric=custom.cloudmonitoring.googleapis.com/kubernetes.io/cpu/node_reservation,"
          metrics+="utilization-target=${TARGET_NODE_UTILIZATION},utilization-target-type=GAUGE "
          metrics+="--custom-metric-utilization metric=custom.cloudmonitoring.googleapis.com/kubernetes.io/memory/node_reservation,"
          metrics+="utilization-target=${TARGET_NODE_UTILIZATION},utilization-target-type=GAUGE "

          echo "Creating node autoscalers."

          local max_instances_per_mig=$(((${AUTOSCALER_MAX_NODES} + ${NUM_MIGS} - 1) / ${NUM_MIGS}))
          local last_max_instances=$((${AUTOSCALER_MAX_NODES} - (${NUM_MIGS} - 1) * ${max_instances_per_mig}))
          local min_instances_per_mig=$(((${AUTOSCALER_MIN_NODES} + ${NUM_MIGS} - 1) / ${NUM_MIGS}))
          local last_min_instances=$((${AUTOSCALER_MIN_NODES} - (${NUM_MIGS} - 1) * ${min_instances_per_mig}))

          for ((i=1; i<${NUM_MIGS}; i++)); do
            gcloud compute instance-groups managed set-autoscaling "${NODE_INSTANCE_PREFIX}-group-$i" --zone "${ZONE}" --project "${PROJECT}" \
                --min-num-replicas "${min_instances_per_mig}" --max-num-replicas "${max_instances_per_mig}" ${metrics} || true
          done
          gcloud compute instance-groups managed set-autoscaling "${NODE_INSTANCE_PREFIX}-group" --zone "${ZONE}" --project "${PROJECT}" \
            --min-num-replicas "${last_min_instances}" --max-num-replicas "${last_max_instances}" ${metrics} || true
        fi
16. Verify that/wait until cluster is up

## Configure Masters (by running configure-node as part of cloud-config)
1. Mount master drive
          if [[ ! -e /dev/disk/by-id/google-master-pd ]]; then
            return
          fi
          device_info=$(ls -l /dev/disk/by-id/google-master-pd)
          relative_path=${device_info##* }
          device_path="/dev/disk/by-id/${relative_path}"

          # Format and mount the disk, create directories on it for all of the master's
          # persistent data, and link them to where they're used.
          echo "Mounting master-pd"
          mkdir -p /mnt/master-pd
          safe_format_and_mount=${SALT_DIR}/salt/helpers/safe_format_and_mount
          chmod +x ${safe_format_and_mount}
          ${safe_format_and_mount} -m "mkfs.ext4 -F" "${device_path}" /mnt/master-pd &>/var/log/master-pd-mount.log || \
          { echo "!!! master-pd mount failed, review /var/log/master-pd-mount.log !!!"; return 1; }
          # Contains all the data stored in etcd
          mkdir -m 700 -p /mnt/master-pd/var/etcd
          # Contains the dynamically generated apiserver auth certs and keys
          mkdir -p /mnt/master-pd/srv/kubernetes
          # Contains the cluster's initial config parameters and auth tokens
          mkdir -p /mnt/master-pd/srv/salt-overlay
          # Directory for kube-apiserver to store SSH key (if necessary)
          mkdir -p /mnt/master-pd/srv/sshproxy

          ln -s -f /mnt/master-pd/var/etcd /var/etcd
          ln -s -f /mnt/master-pd/srv/kubernetes /srv/kubernetes
          ln -s -f /mnt/master-pd/srv/sshproxy /srv/sshproxy
          ln -s -f /mnt/master-pd/srv/salt-overlay /srv/salt-overlay

          # This is a bit of a hack to get around the fact that salt has to run after the
          # PD and mounted directory are already set up. We can't give ownership of the
          # directory to etcd until the etcd user and group exist, but they don't exist
          # until salt runs if we don't create them here. We could alternatively make the
          # permissions on the directory more permissive, but this seems less bad.
          if ! id etcd &>/dev/null; then
            useradd -s /sbin/nologin -d /var/etcd etcd
          fi
          chown -R etcd /mnt/master-pd/var/etcd
          chgrp -R etcd /mnt/master-pd/var/etcd
2. Create Salt master auth
          if [[ ! -e /srv/kubernetes/ca.crt ]]; then
            if  [[ ! -z "${CA_CERT:-}" ]] && [[ ! -z "${MASTER_CERT:-}" ]] && [[ ! -z "${MASTER_KEY:-}" ]]; then
              mkdir -p /srv/kubernetes
              (umask 077;
                echo "${CA_CERT}" | base64 -d > /srv/kubernetes/ca.crt;
                echo "${MASTER_CERT}" | base64 -d > /srv/kubernetes/server.cert;
                echo "${MASTER_KEY}" | base64 -d > /srv/kubernetes/server.key;
                # Kubecfg cert/key are optional and included for backwards compatibility.
                # TODO(roberthbailey): Remove these two lines once GKE no longer requires
                # fetching clients certs from the master VM.
                echo "${KUBECFG_CERT:-}" | base64 -d > /srv/kubernetes/kubecfg.crt;
                echo "${KUBECFG_KEY:-}" | base64 -d > /srv/kubernetes/kubecfg.key)
            fi
          fi
          if [ ! -e "${BASIC_AUTH_FILE}" ]; then
            mkdir -p /srv/salt-overlay/salt/kube-apiserver
            (umask 077;
              echo "${KUBE_PASSWORD},${KUBE_USER},admin" > "${BASIC_AUTH_FILE}")
          fi
          if [ ! -e "${KNOWN_TOKENS_FILE}" ]; then
            mkdir -p /srv/salt-overlay/salt/kube-apiserver
            (umask 077;
              echo "${KUBE_BEARER_TOKEN},admin,admin" > "${KNOWN_TOKENS_FILE}";
              echo "${KUBELET_TOKEN},kubelet,kubelet" >> "${KNOWN_TOKENS_FILE}";
              echo "${KUBE_PROXY_TOKEN},kube_proxy,kube_proxy" >> "${KNOWN_TOKENS_FILE}")

            # Generate tokens for other "service accounts".  Append to known_tokens.
            #
            # NB: If this list ever changes, this script actually has to
            # change to detect the existence of this file, kill any deleted
            # old tokens and add any new tokens (to handle the upgrade case).
            local -r service_accounts=("system:scheduler" "system:controller_manager" "system:logging" "system:monitoring")
            for account in "${service_accounts[@]}"; do
              token=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
              echo "${token},${account},${account}" >> "${KNOWN_TOKENS_FILE}"
            done
          fi
3. load-master-components-images
          ${SALT_DIR}/install.sh ${KUBE_BIN_TAR}
          ${SALT_DIR}/salt/kube-master-addons/kube-master-addons.sh

          # Get the image tags.
          KUBE_APISERVER_DOCKER_TAG=$(cat ${KUBE_BIN_DIR}/kube-apiserver.docker_tag)
          KUBE_CONTROLLER_MANAGER_DOCKER_TAG=$(cat ${KUBE_BIN_DIR}/kube-controller-manager.docker_tag)
          KUBE_SCHEDULER_DOCKER_TAG=$(cat ${KUBE_BIN_DIR}/kube-scheduler.docker_tag)
4. configure-master-components
configure-admission-controls
          configure-etcd
          configure-etcd-events
          configure-kube-apiserver
          configure-kube-scheduler
          configure-kube-controller-manager
          configure-master-addons
5. configure-logging
          if [[ "${LOGGING_DESTINATION}" == "gcp" ]];then
            echo "Configuring fluentd-gcp"
            # fluentd-gcp
            evaluate-manifest ${MANIFESTS_DIR}/fluentd-gcp.yaml /etc/kubernetes/manifests/fluentd-gcp.yaml
          elif [[ "${LOGGING_DESTINATION}" == "elasticsearch" ]];then
            echo "Configuring fluentd-es"
            # fluentd-es
            evaluate-manifest ${MANIFESTS_DIR}/fluentd-es.yaml /etc/kubernetes/manifests/fluentd-es.yaml
          fi

## Configure Nodes (by running configure-node as part of cloud-config)
1. configure-kube-proxy
          echo "Configuring kube-proxy"
          mkdir -p /var/lib/kube-proxy
          evaluate-manifest ${MANIFESTS_DIR}/kubeproxy-config.yaml /var/lib/kube-proxy/kubeconfig
2. configure-logging
        if [[ "${LOGGING_DESTINATION}" == "gcp" ]];then
          echo "Configuring fluentd-gcp"
          # fluentd-gcp
          evaluate-manifest ${MANIFESTS_DIR}/fluentd-gcp.yaml /etc/kubernetes/manifests/fluentd-gcp.yaml
        elif [[ "${LOGGING_DESTINATION}" == "elasticsearch" ]];then
          echo "Configuring fluentd-es"
          # fluentd-es
          evaluate-manifest ${MANIFESTS_DIR}/fluentd-es.yaml /etc/kubernetes/manifests/fluentd-es.yaml
        fi
