#!/usr/bin/env python
import os.path
import subprocess
import argparse
import shutil


cl_parser = argparse.ArgumentParser()
args = cl_parser.parse_args()

os.chdir(os.path.abspath(os.path.dirname(__file__)))

if not os.path.exists('assets/certificates'):
    os.makedirs('assets/certificates')
os.chdir('assets/certificates')

print(os.listdir('.'))

with file('worker.json', 'wt') as f:
    f.write("""{
  "CN": "node.staging.realtimemusic.com",
  "hosts": [
    "127.0.0.1",
    "staging-node"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "DE",
      "L": "Germany",
      "ST": ""
    }
  ]
}
""")

subprocess.check_call(
    'cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json '
    '-profile=client-server worker.json | '
    'cfssljson -bare worker-client', shell=True)
