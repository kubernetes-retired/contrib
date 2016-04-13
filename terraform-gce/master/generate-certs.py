#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('dns_address', help='Specify app\'s DNS address')
cl_parser.add_argument('region', help='Specify GCE region')
cl_parser.add_argument('public_ip', help='Specify node public IP')
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

if not os.path.exists('assets/certificates'):
    os.makedirs('assets/certificates')
os.chdir('assets/certificates')

with file('master.json', 'wt') as f:
    f.write("""{{
  "CN": "master.{0}",
  "hosts": [
    "{0}",
    "{1}",
    "staging-master",
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
            args.dns_address, args.public_ip, args.region
            ))

subprocess.check_call(
    'cfssl gencert -initca=true ca-csr.json | cfssljson -bare ca -',
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master.json | '
    'cfssljson -bare master-peer',
    shell=True)
subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server master.json | '
    'cfssljson -bare master-client',
    shell=True)
