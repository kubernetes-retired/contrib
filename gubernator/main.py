#!/usr/bin/env python
#
# Copyright 2016 Google Inc.
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
#

import functools
import json
import re
import os
import urllib
import zlib

import webapp2
import jinja2

from google.appengine.api import memcache

import defusedxml.ElementTree as ET
import cloudstorage as gcs

import filters

BUCKET_WHITELIST = {
    'kubernetes-jenkins',
}

DEFAULT_JOBS = {
    'kubernetes-jenkins/logs/': {
        'kubelet-gce-e2e-ci',
        'kubernetes-build',
        'kubernetes-e2e-gce',
        'kubernetes-e2e-gce-scalability',
        'kubernetes-e2e-gce-slow',
        'kubernetes-e2e-gke',
        'kubernetes-e2e-gke-slow',
        'kubernetes-kubemark-5-gce',
        'kubernetes-kubemark-500-gce',
        'kubernetes-test-go',
    }
}

JINJA_ENVIRONMENT = jinja2.Environment(
    loader=jinja2.FileSystemLoader(os.path.dirname(__file__) + '/templates'),
    extensions=['jinja2.ext.autoescape'],
    trim_blocks=True,
    autoescape=True)

filters.register(JINJA_ENVIRONMENT.filters)


def pad_numbers(s):
    """Modify a string to make its numbers suitable for natural sorting."""
    return re.sub(r'\d+', lambda m: m.group(0).rjust(16, '0'), s)


def memcache_memoize(prefix, exp=60 * 60, neg_exp=60):
    """Decorate a function to memoize its results using memcache.

    The function must take a single string as input, and return a pickleable
    type.

    Args:
        prefix: A prefix for memcache keys to use for memoization.
        exp: How long to memoized values, in seconds.
        neg_exp: How long to memoize falsey values, in seconds
    Returns:
        A decorator closure to wrap the function.
    """
    # setting the namespace based on the current version prevents different
    # versions from sharing cache values -- meaning there's no need to worry
    # about incompatible old key/value pairs
    namespace = os.environ['CURRENT_VERSION_ID']
    def wrapper(func):
        @functools.wraps(func)
        def wrapped(arg):
            key = prefix + arg
            data = memcache.get(key, namespace=namespace)
            if data is not None:
                return data
            else:
                data = func(arg)
                if data:
                    memcache.add(key, data, exp, namespace=namespace)
                else:
                    memcache.add(key, data, neg_exp, namespace=namespace)
                return data
        return wrapped
    return wrapper


@memcache_memoize('gs://')
def gcs_read(path):
    """Read a file from GCS. Returns None on errors."""
    try:
        with gcs.open(path) as f:
            return f.read()
    except gcs.errors.Error:
        return None


@memcache_memoize('gs-ls://', exp=60)
def gcs_ls(path):
    """Enumerate files in a GCS directory. Returns a list of FileStats."""
    return list(gcs.listbucket(path, delimiter='/'))


@memcache_memoize('build-details://', exp=60 * 60 * 4)
def build_details(build_dir):
    started = gcs_read(build_dir + '/started.json')
    finished = gcs_read(build_dir + '/finished.json')
    if not (started and finished):
        return
    started = json.loads(started)
    finished = json.loads(finished)
    failures = []
    for n in xrange(1, 99):
        junit = gcs_read(
            '%s/artifacts/junit_%02d.xml' % (build_dir, n))
        if junit is None:
            break
        failures.extend(parse_junit(decompress(junit)))
    return started, finished, failures


def decompress(data):
    """Decompress data if GZIP-compressed, but pass normal data thorugh."""
    if data.startswith('\x1f\x8b'):  # gzip magic
        return zlib.decompress(data, 15 | 16)
    return data


def parse_junit(xml):
    """Generate failed tests as a series of (name, duration, text) tuples."""
    for child in ET.fromstring(xml):
        name = child.attrib['name']
        time = float(child.attrib['time'])
        failed = False
        skipped = False
        text = None
        for param in child:
            if param.tag == 'skipped':
                skipped = True
                text = param.text
            elif param.tag == 'failure':
                failed = True
                text = param.text
        if failed:
            yield name, time, text


class RenderingHandler(webapp2.RequestHandler):
    """Base class for Handlers that render Jinja templates."""
    def render(self, template, context):
        """Render a context dictionary using a given template."""
        template = JINJA_ENVIRONMENT.get_template(template)
        self.response.write(template.render(context))


class IndexHandler(RenderingHandler):
    """Render the index."""
    def get(self):
        self.render("index.html", {'jobs': DEFAULT_JOBS})


class BuildHandler(RenderingHandler):
    """Show information about a Build and its failing tests."""
    def get(self, bucket, prefix, job, build):
        if bucket not in BUCKET_WHITELIST:
            self.error(404)
            return
        job_dir = '/%s/%s%s/' % (bucket, prefix, job)
        build_dir = job_dir + build
        details = build_details(build_dir)
        if not details:
            self.response.write("Unable to load build details from %s"
                                % job_dir)
            self.error(404)
            return
        started, finished, failures = details
        commit = started['version'].split('+')[-1]
        self.render('build.html', dict(
            job_dir=job_dir, build_dir=build_dir, job=job, build=build,
            commit=commit, started=started, finished=finished,
            failures=failures))


class BuildListHandler(RenderingHandler):
    """Show a list of Builds for a Job."""
    def get(self, bucket, prefix, job):
        if bucket not in BUCKET_WHITELIST:
            self.error(404)
            return
        job_dir = '/%s/%s%s/' % (bucket, prefix, job)
        fstats = gcs_ls(job_dir)
        fstats.sort(key=lambda f: pad_numbers(f.filename), reverse=True)
        self.render('build_list.html', dict(job=job, job_dir=job_dir, fstats=fstats))


class JobListHandler(RenderingHandler):
    """Show a list of Jobs in a directory."""
    def get(self, bucket, prefix):
        if bucket not in BUCKET_WHITELIST:
            self.error(404)
            return
        jobs_dir = '/%s/%s/' % (bucket, prefix)
        fstats = gcs_ls(jobs_dir)
        fstats.sort()
        self.render('job_list.html', dict(jobs_dir=jobs_dir, fstats=fstats))


app = webapp2.WSGIApplication([
    (r'/', IndexHandler),
    (r'/jobs/([-\w]+)/(.*[-\w])/?$', JobListHandler),
    (r'/builds/([-\w]+)/(.*/)?([^/]+)/?', BuildListHandler),
    (r'/build/([-\w]+)/(.*/)?([^/]+)/(\d+)/?', BuildHandler),
], debug=True)

if os.environ.get('SERVER_SOFTWARE','').startswith('Development'):
    # inject some test data so there's a page with some content
    import tarfile
    tf = tarfile.open('test_data/kube_results.tar.gz')
    prefix = '/kubernetes-jenkins/logs/kubernetes-soak-continuous-e2e-gce/'
    for member in tf:
        if member.isfile():
            with gcs.open(prefix + member.name, 'w') as f:
                f.write(tf.extractfile(member).read())
    with gcs.open(prefix + '5044/started.json', 'w') as f:
        f.write('test')  # So JobHandler has more than one item.
