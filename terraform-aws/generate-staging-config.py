#!/usr/bin/env python3
import jinja2.exceptions
import sys
import subprocess
import argparse
import os.path


def _error(msg):
    sys.stderr.write('{}\n'.format(msg))
    sys.exit(1)


class InstanceBase:
    def __init__(self, number):
        self.number = number


class Master(InstanceBase):
    def __init__(self, number):
        super().__init__(number)
        self.private_ip = '172.31.29.{}'.format(110 + number)


class Worker(InstanceBase):
    def __init__(self, number):
        super().__init__(number)
        self.private_ip = '172.31.30.{}'.format(110 + number)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument(
    'num_masters', type=int, help='Specify number of master nodes')
cl_parser.add_argument(
    'num_workers', type=int, help='Specify number of worker nodes')
cl_parser.add_argument('region', help='Specify AWS region')
cl_parser.add_argument('public_ip', help='Specify app public IP')
cl_parser.add_argument('dns_address', help='Specify DNS address')
cl_parser.add_argument('access_key_id', help='Specify AWS access key ID')
cl_parser.add_argument(
    'secret_access_key', help='Specify AWS secret access key')
cl_parser.add_argument('key_name', help='Specify AWS SSH key name')
args = cl_parser.parse_args()

master_instances = [
    Master(1),
    Master(2),
]
worker_instances = [
    Worker(1),
    Worker(2),
]

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
            access_key_id=args.access_key_id,
            secret_access_key=args.secret_access_key,
            region=args.region,
            aws_key_name=args.key_name,
        ).dump(f)

master_private_ips = [x.private_ip for x in master_instances]
for i, master in enumerate(master_instances):
    private_ip = master_private_ips[i]
    subprocess.check_call([
        './master/generate-assets.py',
        str(i + 1),
        args.dns_address,
        args.region,
        args.public_ip,
        private_ip,
    ] + list(set(master_private_ips).difference([private_ip])))
for i, worker in enumerate(worker_instances):
    subprocess.check_call([
        './worker/generate-assets.py',
        str(i + 1),
        worker.private_ip,
    ] + master_private_ips)
