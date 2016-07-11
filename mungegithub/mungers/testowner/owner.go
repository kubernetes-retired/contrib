/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package testowner

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/gregjones/httpcache"
)

var tagRegex = regexp.MustCompile(`\[.*?\]|\{.*?\}`)
var whiteSpaceRegex = regexp.MustCompile(`\s+`)

// Turn a test name into a canonical form (without tags, lowercase, etc.)
func normalize(name string) string {
	tagLess := tagRegex.ReplaceAll([]byte(name), []byte(""))
	squeezed := whiteSpaceRegex.ReplaceAll(tagLess, []byte(" "))
	return strings.ToLower(strings.TrimSpace(string(squeezed)))
}

// OwnerList uses a map to get owners for a given test name.
type OwnerList struct {
	mapping map[string]string
}

// TestOwner returns the owner for a test, or the empty string if none is found.
func (o *OwnerList) TestOwner(testName string) string {
	name := normalize(testName)
	owner, _ := o.mapping[name]
	return owner
}

// NewOwnerList constructs an OwnerList given a mapping from test names to test owners.
func NewOwnerList(mapping map[string]string) *OwnerList {
	list := OwnerList{}
	list.mapping = make(map[string]string)
	for input, output := range mapping {
		list.mapping[normalize(input)] = output
	}
	return &list
}

// NewOwnerListFromCsv constructs an OwnerList given a CSV file that includes
// 'owner' and 'test name' columns.
func NewOwnerListFromCsv(r io.Reader) (*OwnerList, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	mapping := make(map[string]string)
	ownerCol := -1
	nameCol := -1
	for _, record := range records {
		if ownerCol == -1 || nameCol == -1 {
			for col, val := range record {
				switch strings.ToLower(val) {
				case "owner":
					ownerCol = col
				case "test name":
					nameCol = col
				}
			}
		} else {
			mapping[record[nameCol]] = record[ownerCol]
		}
	}
	return NewOwnerList(mapping), nil
}

// HTTPOwnerList maps test names to owners, loading the mapping from a given
// URL and reloading as necessary when it expries from the cache.
type HTTPOwnerList struct {
	url       string
	client    http.Client
	ownerList *OwnerList
}

// NewHTTPOwnerList creates a HTTPOwnerList given a URL to a CSV
// file containing owner mapping information.
func NewHTTPOwnerList(url string) (*HTTPOwnerList, error) {
	// httpcache respects
	transport := httpcache.NewMemoryCacheTransport()
	ownerList := &HTTPOwnerList{
		url:    url,
		client: http.Client{Transport: transport},
	}
	err := ownerList.reload()
	if err != nil {
		return nil, err
	}
	return ownerList, nil
}

// TestOwner returns the owner for a test, or the empty string if none is found.
func (o *HTTPOwnerList) TestOwner(testName string) string {
	err := o.reload()
	if err != nil {
		glog.Errorf("Unable to reload test owners at %s: %v", o.url, err)
		// Process using the previous data.
	}
	return o.ownerList.TestOwner(testName)
}

func (o *HTTPOwnerList) reload() error {
	resp, err := o.client.Get(o.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return errors.New("error status " + resp.Status)
	}
	ownerList, err := NewOwnerListFromCsv(resp.Body)
	if err != nil {
		return err
	}
	if resp.Header.Get(httpcache.XFromCache) == "" {
		glog.Info("Fetched new test owners from %s", o.url)
	}
	o.ownerList = ownerList
	return nil
}
