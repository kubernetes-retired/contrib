#!/bin/dash

# Copyright 2017 The Kubernetes Authors.
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

_term() {
  iptables -D -t filter -I KUBE-METADATA-SERVER -j ACCEPT
  iptables -D -t nat -I PREROUTING -p tcp -d 169.254.169.254 --dport 80 -j DNAT --to-destination 127.0.0.1:2020
  kill -TERM "$child" 2>/dev/null
  exit
}

# Forward traffic to nginx.
iptables -t nat -I PREROUTING -p tcp -d 169.254.169.254 --dport 80 -j DNAT --to-destination 127.0.0.1:2020
iptables -t filter -I KUBE-METADATA-SERVER -j ACCEPT

# Clean up the iptables rule if we're exiting gracefully.
trap _term SIGTERM

# Run nginx in the foreground.
nginx -g 'daemon off;'
child=$!
wait $child
