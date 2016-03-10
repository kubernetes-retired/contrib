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

with file('master1-master.json', 'wt') as f:
    f.write("""{{
  "CN": "master1.staging.realtimemusic.com",
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

subprocess.check_call(
    'cfssl gencert -initca=true ca-csr.json | cfssljson -bare ca -',
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master1-master.json | '
    'cfssljson -bare master1-master-peer',
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master1-master.json | '
    'cfssljson -bare master1-master-client',
    shell=True)
