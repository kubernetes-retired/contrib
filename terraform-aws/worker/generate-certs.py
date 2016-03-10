#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('private_ip', help='Specify node private IP')
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

os.chdir('assets/certificates')

print(os.listdir('.'))

with file('worker1-worker.json', 'wt') as f:
    f.write("""{{
  "CN": "worker1.staging.realtimemusic.com",
  "hosts": [
    "{0}",
    "ip-{1}.eu-central-1.compute.internal",
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
""".format(args.private_ip, args.private_ip.replace('.', '-')))

# cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=client-server worker1-worker.json | cfssljson -bare worker1-worker-client

subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server worker1-worker.json | '
    'cfssljson -bare worker1-worker-client',
    shell=True)
