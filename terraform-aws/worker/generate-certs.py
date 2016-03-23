#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', type=int, help='Specify node number')
cl_parser.add_argument('private_ip', help='Specify node private IP')
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

os.chdir('assets/certificates')

print(os.listdir('.'))

with file('worker{0}-worker.json'.format(args.node_num), 'wt') as f:
    f.write("""{{
  "CN": "worker{0}.staging.realtimemusic.com",
  "hosts": [
    "{1}",
    "ip-{2}.eu-central-1.compute.internal",
    "127.0.0.1"
  ],
  "key": {{
    "algo": "rsa",
    "size": 2048
  }},
  "names": [
    {{
      "C": "DE",
      "L": "Germany",
      "ST": ""
    }}
  ]
}}
""".format(args.node_num, args.private_ip, args.private_ip.replace('.', '-')))

subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server worker{0}-worker.json | '
    'cfssljson -bare worker{0}-worker-client'.format(args.node_num),
    shell=True)
