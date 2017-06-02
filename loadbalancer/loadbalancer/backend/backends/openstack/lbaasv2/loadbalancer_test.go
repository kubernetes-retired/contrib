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

package lbaasv2

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	fake "github.com/rackspace/gophercloud/openstack/networking/v2/common"
	th "github.com/rackspace/gophercloud/testhelper"
	"github.com/rackspace/gophercloud/testhelper/client"
)

// PoolsListBody contains the canned body of a pool list response.
const PoolsListBody = `
{
	"pools":[
	         {
			"lb_algorithm":"ROUND_ROBIN",
			"protocol":"HTTP",
			"description":"",
			"healthmonitor_id": "466c8345-28d8-4f84-a246-e04380b0461d",
			"members":[{"id": "53306cda-815d-4354-9fe4-59e09da9c3c5"}],
			"listeners":[{"id": "2a280670-c202-4b0b-a562-34077415aabf"}],
			"loadbalancers":[{"id": "79e05663-7f03-45d2-a092-8b94062f22ab"}],
			"id":"72741b06-df4d-4715-b142-276b6bce75ab",
			"name":"web",
			"admin_state_up":true,
			"tenant_id":"83657cfcdfe44cd5920adaf26c48ceea",
			"provider": "haproxy"
		}
	]
}
`

// MembersListBody contains the canned body of a member list response.
const MembersListBody = `
{
	"members":[
		{
			"id": "2a280670-c202-4b0b-a562-34077415aabf",
			"address": "10.0.2.10",
			"weight": 5,
			"name": "web",
			"subnet_id": "1981f108-3c48-48d2-b908-30f7d28532c9",
			"tenant_id": "2ffc6e22aae24e4795f87155d24c896f",
			"admin_state_up":true,
			"protocol_port": 80
		}
	]
}
`

// HandlePoolListSuccessfully sets up the test server to respond to a pool List request.
func handlePoolListSuccessfully(t *testing.T) {
	th.Mux.HandleFunc("/v2.0/lbaas/pools", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", client.TokenID)

		w.Header().Add("Content-Type", "application/json")
		r.ParseForm()
		marker := r.Form.Get("marker")
		switch marker {
		case "":
			fmt.Fprintf(w, PoolsListBody)
		case "45e08a3e-a78f-4b40-a229-1e7e23eee1ab":
			fmt.Fprintf(w, `{ "pools": [] }`)
		default:
			t.Fatalf("/v2.0/lbaas/pools invoked with unexpected marker=[%s]", marker)
		}
	})
}

func HandleMemberListSuccessfully(t *testing.T) {
	th.Mux.HandleFunc("/v2.0/lbaas/pools/332abe93-f488-41ba-870b-2ac66be7f853/members", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", client.TokenID)

		w.Header().Add("Content-Type", "application/json")
		r.ParseForm()
		marker := r.Form.Get("marker")
		switch marker {
		case "":
			fmt.Fprintf(w, MembersListBody)
		case "45e08a3e-a78f-4b40-a229-1e7e23eee1ab":
			fmt.Fprintf(w, `{ "members": [] }`)
		default:
			t.Fatalf("/v2.0/lbaas/pools/332abe93-f488-41ba-870b-2ac66be7f853/members invoked with unexpected marker=[%s]", marker)
		}
	})
}

func TestGetPoolIDFromName(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	handlePoolListSuccessfully(t)

	lbaasControl := LBaaSController{
		compute:  fake.ServiceClient(),
		network:  fake.ServiceClient(),
		subnetID: os.Getenv("OS_SUBNET_ID"),
	}
	poolID, err := lbaasControl.getPoolIDFromName("web")
	if err != nil {
		t.Errorf("Error is: %v ", err)
	}

	expPoolID := "72741b06-df4d-4715-b142-276b6bce75ab"
	if poolID != expPoolID {
		t.Errorf("Error: Wrong pool ID. Expected: %v. Actual %v", expPoolID, poolID)
	}
}

func TestGetMemberIDFromIP(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	HandleMemberListSuccessfully(t)

	lbaasControl := LBaaSController{
		compute:  fake.ServiceClient(),
		network:  fake.ServiceClient(),
		subnetID: os.Getenv("OS_SUBNET_ID"),
	}
	memberID, err := lbaasControl.getMemberIDFromIP("332abe93-f488-41ba-870b-2ac66be7f853", "web")
	if err != nil {
		t.Errorf("Error is: %v ", err)
	}

	expMemberID := "2a280670-c202-4b0b-a562-34077415aabf"
	if memberID != expMemberID {
		t.Errorf("Error: Wrong member ID. Expected: %v. Actual %v", expMemberID, memberID)
	}
}
