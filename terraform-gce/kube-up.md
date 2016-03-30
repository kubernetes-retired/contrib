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
