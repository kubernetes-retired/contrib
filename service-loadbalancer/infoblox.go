/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	infoblox_nextIpFunction = "func:nextavailableip:"
)

// store infoblox api data and allow for actions against api
type infobloxController struct {
	user         string
	password     string
	baseEndpoint string
	dnsSubDomain string
	subnet       string
}

type infoBloxHost struct {
	Ref string `json:"_ref"`
}

type infoBloxHostCreate struct {
	Name string        `json:"name"`
	Ips  []infoBloxIps `json:"ipv4addrs,ar1ray"`
}

type infoBloxIps struct {
	Address string `json:"ipv4addr"`
}

// newInfobloxController creates a new infoBloxController from the given config
func newInfobloxController(user, password, baseURL, dnsSubDomain, subnet string) *infobloxController {
	ibc := infobloxController{
		user:         user,
		password:     password,
		baseEndpoint: baseURL,
		dnsSubDomain: dnsSubDomain,
		subnet:       subnet,
	}
	return &ibc
}

// get current dns entry
func (infoblx *infobloxController) getHost(name string) (host []infoBloxHost, err error) {
	client, req := getHTTPClientRequest(infoblx.password,
		fmt.Sprintf("%s/record:host?name~=%s.%s", infoblx.baseEndpoint, name, infoblx.dnsSubDomain), "GET", nil)
	resp, err := client.Do(req)

	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	// read the body response
	bodyBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return
	}

	body := string(bodyBytes)

	// parse body to object
	host, err = infoblx.parseHost(body)

	return
}

// delete dns entry in infoblox
func (infoblx *infobloxController) deleteHost(name string) (int, error) {
	// get all hosts
	hosts, err := infoblx.getHost(name)

	if err != nil {
		return 0, err
	}

	for _, host := range hosts {
		client, req := getHTTPClientRequest(infoblx.password, fmt.Sprintf("%s/%s", infoblx.baseEndpoint, host.Ref), "DELETE", nil)
		client.Do(req)
	}

	return len(hosts), nil
}

// create dns entry by getting next available ip
func (infoblx *infobloxController) createHostNextIP(name string) (int, error) {
	records, err := infoblx.createHost(name, fmt.Sprintf("%s%s", infoblox_nextIpFunction, infoblx.subnet))
	return records, err
}

// create dns entry in infoblox using provided ip address
func (infoblx *infobloxController) createHost(name, ip string) (int, error) {
	//first check if it already exists, if so, don't create again
	hosts, _ := infoblx.getHost(name)

	if len(hosts) > 0 {
		return 0, nil
	}

	ips := []infoBloxIps{}
	ips = append(ips, infoBloxIps{Address: ip})

	// create the object to post in body
	bodyObj := infoBloxHostCreate{
		Name: fmt.Sprintf("%s.%s", name, infoblx.dnsSubDomain),
		Ips:  ips,
	}

	s, _ := json.Marshal(bodyObj)
	body := bytes.NewBuffer(s)

	client, req := getHTTPClientRequest(infoblx.password, fmt.Sprintf("%s/record:host", infoblx.baseEndpoint), "POST", body)
	resp, err := client.Do(req)

	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return 1, nil
}

// parse body to infoBloxHost object
func (infoblx *infobloxController) parseHost(jsonString string) (hosts []infoBloxHost, err error) {
	var ifbxArray []infoBloxHost
	dec := json.NewDecoder(strings.NewReader(jsonString))
	err = dec.Decode(&ifbxArray)

	if err != nil {
		return
	}

	// if returned the single dns entry, if got more than one, return nothing
	if len(ifbxArray) > 0 {
		hosts = ifbxArray
	}

	return
}

// generates a httpClient & httpRequest
func getHTTPClientRequest(password, url, httpRequestType string, httpBody io.Reader) (client *http.Client, request *http.Request) {
	// !!!!!!!! RUH_RHRO !!!!!!!!!!!!!!!
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Transport: tr}
	request, _ = http.NewRequest(httpRequestType, url, httpBody)
	request.Header.Add("Authorization", fmt.Sprintf("Basic %s", password)) //TODO
	return
}
