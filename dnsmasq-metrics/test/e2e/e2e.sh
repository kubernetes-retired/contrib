#!/bin/sh
# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Runs the e2e test

set -e

if ! which docker >/dev/null; then
  echo "docker executable not found"
  exit 1
fi

if ! which awk >/dev/null; then
  echo "awk executable not found"
  exit 1
fi

if [ ! -e bin/amd64/dnsmasq-metrics ]; then
  echo "dnsmasq-metrics not found (need to build?)"
  exit 1
fi

uuid=`date +%s`
image_tag="kubernetes-contrib-dnsmasq-metrics-e2e-${uuid}"
output_dir=`mktemp -d`
e2e_dir=test/e2e

if [ "$CLEANUP" != 'no' ]; then
  cleanup() {
    echo "Removing ${output_dir}"
    rm -r ${output_dir}
  }
  trap cleanup EXIT
fi
echo "Output to ${output_dir} (set env CLEANUP=no to disable cleanup)"

echo "Building image"
docker build \
       -f ${e2e_dir}/Dockerfile.e2e \
       -t ${image_tag} \
       . >> ${output_dir}/docker.log
echo "Running tests"
docker run --rm=true ${image_tag} > ${output_dir}/e2e.log
echo "Removing image"
docker rmi ${image_tag} >> ${output_dir}/docker.log

cat ${output_dir}/e2e.log | awk '
/END metrics ====/{ inMetrics = 0 }
{
  if (inMetrics) {
    print($0)      
  }
}
/BEGIN metrics ====/ { inMetrics = 1 }
' > ${output_dir}/metrics.log

# Validate results.
errors=0

max_size=`grep kubedns_dnsmasq_max_size ${output_dir}/metrics.log | awk '{print $2}'`
hits=`grep kubedns_dnsmasq_hits ${output_dir}/metrics.log | awk '{print $2}'`

ok_errors=`grep kubedns_probe_ok_errors ${output_dir}/metrics.log | awk '{print $2}'`
nxdomain_errors=`grep kubedns_probe_nxdomain_errors ${output_dir}/metrics.log | awk '{print $2}'`
notpresent_errors=`grep kubedns_probe_notpresent_errors ${output_dir}/metrics.log | awk '{print $2}'`

die() {
  echo "Failed: " "$@"
  exit 1
}

[ -z "${max_size}" ] && die "missing max_size"
[ -z "${hits}" ] && die "missing hits"
[ -z "${ok_errors}" ] && die "missing ok_errors"
[ -z "${nxdomain_errors}" ] && die "missing nxdomain_errors"
[ -z "${notpresent_errors}" ] && die "missing notpresent_errors"

if [ "${max_size}" -ne 1337 ]; then
  echo "Failed: expected max_size == 1337, got ${max_size}"
  errors=$(( $errors + 1))
fi

if [ "${hits}" -lt 100 ]; then
  echo "Failed: expected hits > 100, got ${hits}"
  errors=$(( $errors + 1))
fi

if [ "${ok_errors}" -ne 0 ]; then
  echo "Failed: expected ok_errors = 0, got ${ok_errors}"
  errors=$(( $errors + 1))
fi

if [ "${nxdomain_errors}" -lt 5 ]; then
  echo "Failed: expected nxdomain_errors > 5, got ${nxdomain_errors}"
  errors=$(( $errors + 1))
fi

if [ "${notpresent_errors}" -lt 5 ]; then
  echo "Failed: expected notpresent_errors > 5, got ${notpresent_errors}"
  errors=$(( $errors + 1))
fi

if [ "${errors}" = 0 ]; then
  echo "Tests passed"
fi

exit ${errors}
