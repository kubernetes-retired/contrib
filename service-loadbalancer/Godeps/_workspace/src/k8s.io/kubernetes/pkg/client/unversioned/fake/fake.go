/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This is made a separate package and should only be imported by tests, because
// it imports testapi
package fake

import (
	"net/http"
	"net/url"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

func CreateHTTPClient(roundTripper func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(roundTripper),
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// RESTClient provides a fake RESTClient interface.
type RESTClient struct {
	Client *http.Client
	Codec  runtime.Codec
	Req    *http.Request
	Resp   *http.Response
	Err    error
}

func (c *RESTClient) Get() *unversioned.Request {
	return unversioned.NewRequest(c, "GET", &url.URL{Host: "localhost"}, *testapi.Default.GroupVersion(), c.Codec, nil)
}

func (c *RESTClient) Put() *unversioned.Request {
	return unversioned.NewRequest(c, "PUT", &url.URL{Host: "localhost"}, *testapi.Default.GroupVersion(), c.Codec, nil)
}

func (c *RESTClient) Patch(_ api.PatchType) *unversioned.Request {
	return unversioned.NewRequest(c, "PATCH", &url.URL{Host: "localhost"}, *testapi.Default.GroupVersion(), c.Codec, nil)
}

func (c *RESTClient) Post() *unversioned.Request {
	return unversioned.NewRequest(c, "POST", &url.URL{Host: "localhost"}, *testapi.Default.GroupVersion(), c.Codec, nil)
}

func (c *RESTClient) Delete() *unversioned.Request {
	return unversioned.NewRequest(c, "DELETE", &url.URL{Host: "localhost"}, *testapi.Default.GroupVersion(), c.Codec, nil)
}

func (c *RESTClient) Do(req *http.Request) (*http.Response, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	c.Req = req
	if c.Client != nil {
		return c.Client.Do(req)
	}
	return c.Resp, nil
}
