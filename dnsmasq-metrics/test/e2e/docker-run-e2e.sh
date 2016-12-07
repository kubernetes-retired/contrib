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

# Docker side of the E2E test for dnsmasq-metrics. This runs an
# instance of dnsmasq and dnsmasq-metrics, sends some DNS queries to
# the daemon and dumps the output of the prometheus /metrics URI.
run() {
  echo RUN "$@"
  "$@"
}

dnsmasq="/usr/sbin/dnsmasq"
dnsmasq_metrics="/dnsmasq-metrics"
dnsmasq_port=10053

dig_out="/tmp/dig.out"
dnsmasq_out="/tmp/dnsmasq.out"
dnsmasq_metrics_out="/tmp/dnsmasq_metrics.out"
metrics_out="/tmp/metrics.out"

curl="/usr/bin/curl"
dig="/usr/bin/dig"

sleep_interval=10

run ${dnsmasq} \
  -q -k \
  -a 127.0.0.1 \
  -p ${dnsmasq_port} \
  -c 1337 \
  -8 - \
  -A "/ok.local/1.2.3.4" \
  -A "/nxdomain.local/" \
  2> ${dnsmasq_out} &
dnsmasq_pid=$!
echo "dnsmasq_pid=${dnsmasq_pid}"

run ${dnsmasq_metrics} \
  --dnsmasq-port ${dnsmasq_port} -v 4 \
  --probe "ok,127.0.0.1:${dnsmasq_port},ok.local,1" \
  --probe "nxdomain,127.0.0.1:${dnsmasq_port},nx.local,1" \
  --probe "notpresent,127.0.0.1:$((dnsmasq_port + 1)),notpresent.local,1" \
  2> ${dnsmasq_metrics_out} &
dnsmasq_metrics_pid=$!
echo "dnsmasq_metrics_pid=${dnsmasq_metrics_pid}"

# Do a bunch of digs to make sure the cache is hit.
for i in `seq 100`; do
  ${dig} @127.0.0.1 -p ${dnsmasq_port} localhost 1>>${dig_out}
done

kill -SIGUSR1 ${dnsmasq_pid}

# Give dnsmasq_metrics some time to updates its metrics.
echo "Waiting ${sleep_interval} seconds"
sleep ${sleep_interval}

${curl} localhost:10054/metrics 2>/dev/null \
  | grep dns \
  | grep -v \# \
  | sort \
  > ${metrics_out}

# Dump output
echo
echo "BEGIN dnsmasq ===="
cat ${dnsmasq_out}
echo "END dnsmasq ===="
echo
echo "BEGIN dnsmasq_metrics ===="
cat ${dnsmasq_metrics_out}
echo "END dnsmasq_metrics ===="
echo
echo "BEGIN metrics ===="
cat ${metrics_out}
echo "END metrics ===="
