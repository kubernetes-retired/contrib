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

import unittest

import filters


class HelperTest(unittest.TestCase):
    def test_timestamp(self):
        self.assertEqual('2016-04-19 21:22', filters.do_timestamp(1461100940))

    def test_duration(self):
        for duration, expected in {
            3.56: '3.56s',
            13.6: '13s',
            78.2: '1m18s',
            60 * 62 + 3: '1h2m',
        }.iteritems():
            self.assertEqual(expected, filters.do_duration(duration))

    def test_linkify_safe(self):
        self.assertEqual('&lt;a&gt;',
                         str(filters.do_linkify_stacktrace('<a>', '3')))

    def test_linkify(self):
        linked = str(filters.do_linkify_stacktrace(
            "/go/src/k8s.io/kubernetes/test/example.go:123", 'VERSION'))
        self.assertIn('<a href="https://github.com/kubernetes/kubernetes/blob/'
                      'VERSION/test/example.go#L123">', linked)

    def test_slugify(self):
        self.assertEqual('k8s-test-foo', filters.do_slugify('[k8s] Test Foo'))


if __name__ == '__main__':
    unittest.main()
