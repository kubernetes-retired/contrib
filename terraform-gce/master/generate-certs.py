#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', type=int, help='Specify node number')
cl_parser.add_argument('dns_address', help='Specify app\'s DNS address')
cl_parser.add_argument('region', help='Specify AWS region')
cl_parser.add_argument('public_ip', help='Specify node public IP')
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

os.chdir('assets/certificates')

with file('master{0}-master.json'.format(args.node_num), 'wt') as f:
    f.write("""{{
  "CN": "master{0}.{1}",
  "hosts": [
    "{1}",
    "{2}",
    "10.3.0.1",
    "127.0.0.1",
    "localhost"
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
""".format(
            args.node_num, args.dns_address, args.public_ip, args.region
            ))

subprocess.check_call(
    'cfssl gencert -initca=true ca-csr.json | cfssljson -bare ca -',
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master{0}-master.json | '
    'cfssljson -bare master{0}-master-peer'.format(args.node_num),
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master{0}-master.json | '
    'cfssljson -bare master{0}-master-client'.format(args.node_num),
    shell=True)
