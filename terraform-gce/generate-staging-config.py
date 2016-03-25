#!/usr/bin/env python3
import jinja2.exceptions
import sys
import subprocess
import argparse
import os.path
import urllib.request


def _error(msg):
    sys.stderr.write('{}\n'.format(msg))
    sys.exit(1)


class InstanceBase:
    def __init__(self, number):
        self.number = number


class Master(InstanceBase):
    def __init__(self, number, public_ip):
        super().__init__(number)
        self.public_ip = public_ip


class Worker(InstanceBase):
    def __init__(self, number):
        super().__init__(number)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument(
    'num_workers', type=int, help='Specify number of worker nodes')
cl_parser.add_argument('project', help='Specify GCE project')
cl_parser.add_argument('region', help='Specify GCE region')
cl_parser.add_argument('zone', help='Specify GCE zone')
cl_parser.add_argument('public_ip', help='Specify app public IP')
cl_parser.add_argument('master_public_ip', help='Specify master public IP')
cl_parser.add_argument('dns_address', help='Specify DNS address')
args = cl_parser.parse_args()

master_instances = [Master(1, args.master_public_ip)]
worker_instances = [Worker(i+1) for i in range(args.num_workers)]

env = jinja2.Environment(
    loader=jinja2.FileSystemLoader('templates'),
    undefined=jinja2.StrictUndefined,
)
try:
    template = env.get_template('staging.tf')
except jinja2.exceptions.TemplateSyntaxError as err:
    _error(err)
else:
    with open('staging.tf', 'wt') as f:
        template.stream(
            master_instances=master_instances,
            worker_instances=worker_instances,
            region=args.region,
            zone=args.zone,
            project=args.project,
            master_public_ip=args.master_public_ip,
        ).dump(f)

with urllib.request.urlopen('https://discovery.etcd.io/new?size={}'.format(
    len(master_instances)
)) \
        as response:
    discovery_url = response.read().decode()

for i, master in enumerate(master_instances):
    subprocess.check_call([
        './master/generate-assets.py',
        str(i + 1),
        args.dns_address,
        args.region,
        discovery_url,
        master.public_ip,
    ])
for i, worker in enumerate(worker_instances):
    subprocess.check_call([
        './worker/generate-assets.py',
        str(i + 1),
    ])
