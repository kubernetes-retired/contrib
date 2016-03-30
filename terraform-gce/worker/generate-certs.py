#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', type=int, help='Specify node number')
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

if not os.path.exists('assets/certificates'):
    os.makedirs('assets/certificates')
os.chdir('assets/certificates')

print(os.listdir('.'))

with file('worker{0}-worker.json'.format(args.node_num), 'wt') as f:
    f.write("""{{
  "CN": "node{0}.staging.realtimemusic.com",
  "hosts": [
    "127.0.0.1",
    "staging-node{0}"
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
""".format(args.node_num,))

subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server worker{0}-worker.json | '
    'cfssljson -bare worker{0}-worker-client'.format(args.node_num),
    shell=True)
