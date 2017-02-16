/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"
	"strings"
	"testing"
)

func TestCollectStats_InValidInput(t *testing.T) {
	R, err := CollectStats("")
	if err == nil {
		t.Fail()
	}
	if R != nil {
		t.Fail()
	}
	if !strings.Contains(err.Error(), COLLECTSTATS_INVALID_INPUT) {
		t.Fail()
	}
}

func TestCollectStats_ServerNotReachable(t *testing.T) {

	R, Err := CollectStats("localhost:6379")

	if Err == nil {
		t.Fail()
	}

	if R != nil {
		t.Fail()
	}

	if !strings.Contains(Err.Error(), COLLECTSTATS_SERVER_NOT_REACHABLE) {
		t.Fail()
	}
}

func TestCollectStatsAll_NilInput(t *testing.T) {

	Servers := CollectStatsAll([]string{})

	if Servers != nil {
		t.Fail()
	}
}

func TestCollectStatsAll_NoReachable(t *testing.T) {

	Servers := CollectStatsAll([]string{"localhost:6379"})

	if Servers != nil {
		t.Fail()
	}
}

func TestParseResponseNullInput(t *testing.T) {

	var R Redis

	resultBool := R.ParseResponse("")

	t.Logf("ParseResponse =%v", resultBool)

	if resultBool {
		t.Fail()
	}
}

func TestParseResponseValid(t *testing.T) {

	var R Redis

	sampleInput := `# Replication\r\nrole:slave\r\nmaster_host:172.31.10.90\r\nmaster_port:6381\r\nmaster_link_status:up\r\nmaster_last_io_seconds_ago:9\r\nmaster_sync_in_progress:0\r\nslave_repl_offset:44983\r\nslave_priority:100\r\nslave_read_only:1\r\nconnected_slaves:0\r\nmaster_repl_offset:0\r\nrepl_backlog_active:0\r\nrepl_backlog_size:1048576\r\nrepl_backlog_first_byte_offset:0\r\nrepl_backlog_histlen:0\r\n`
	if !R.ParseResponse(sampleInput) {
		t.Fail()
	}

	t.Logf("Valid ParseResult = %v", R)

	if R.Role != "slave" {
		t.Fail()
	}
	if R.Priority != 100 {
		t.Fail()
	}
	if R.LastUpdated != 9 {
		t.Fail()
	}
	if R.SyncBytes != 44983 {
		t.Fail()
	}

	t.Logf("R=%v", R)

}

func TestFindNxtMaster_Valid(t *testing.T) {

	var Slaves []*Redis
	var maxSyncbytes int64

	for i := 0; i < 3; i++ {
		var r Redis
		r.MasterHost = "127.0.0.1"
		r.MasterPort = "6313"
		r.Priority = 100
		r.SyncBytes = int64(1000 + i)
		r.EndPoint = fmt.Sprint("127.0.0.1:%d", 6314+i)
		Slaves = append(Slaves, &r)
		maxSyncbytes = r.SyncBytes //Record max sync bytes clearly the last slave will have max syncbytes
	}
	//It should have selected the slaves with Maximum sync bytes
	OldMaster, NewMaster := FindNxtMaster(Slaves)

	//New master cannot be nil, it should have selected a valid master
	if NewMaster == nil {
		t.Errorf("NewMaster cannot be NIL\n")
		t.Fail()
	}

	if OldMaster != nil {
		t.Errorf("Old Master should be Nil")
		t.Fail()
	}

	//if selected slave is not of maxsync bytes then fail
	if NewMaster.SyncBytes != maxSyncbytes {
		t.Errorf("Selected master is not maxSync bytes NewMaster=%v MaxSyncBytes=%v\n", NewMaster.SyncBytes, maxSyncbytes)
		t.Fail()
	}
}

func TestFindNxtMaster_NilInput(t *testing.T) {

	var Slaves []*Redis
	OldMaster, NewMaster := FindNxtMaster(Slaves)

	if NewMaster != nil || OldMaster != nil {
		t.Fail()
	}
}

func TestFindNxtMaster_WithMaster(t *testing.T) {

	var Slaves []*Redis

	for i := 0; i < 3; i++ {
		var r Redis
		r.MasterHost = "127.0.0.1"
		r.MasterPort = "6313"
		r.Priority = 100
		r.SyncBytes = int64(1000 - i)
		r.EndPoint = fmt.Sprint("127.0.0.1:%d", 6314+i)
		r.MasterHost = "127.0.0.1"
		r.MasterPort = "6319"
		r.Role = REDIS_ROLE_SLAVE
		r.MasterLinkStatus = true
		Slaves = append(Slaves, &r)
	}

	var master Redis
	master.EndPoint = "127.0.0.1:6319"
	master.SyncBytes = 1001
	master.Role = REDIS_ROLE_MASTER
	Slaves = append(Slaves, &master)

	OldMaster, NewMaster := FindNxtMaster(Slaves)

	//Should have detected that there is a valid master and should not select a new master
	if NewMaster != nil {
		t.Errorf("There is no new master to be selected, should have been nil")
		t.Fail()
	}

	if OldMaster == nil {
		t.Errorf("Should detected the old master as valid, it cannot be nil")
		t.Fail()
	}
}

func TestFindNxtMaster_WithMisconfiguredMaster(t *testing.T) {

	var Slaves []*Redis

	for i := 0; i < 3; i++ {
		var r Redis
		r.MasterHost = "127.0.0.1"
		r.MasterPort = "6313"
		r.Priority = 100
		r.SyncBytes = int64(1000 - i)
		r.EndPoint = fmt.Sprint("127.0.0.1:%d", 6314+i)
		r.MasterHost = "127.0.0.1"
		r.MasterPort = "6319"
		r.Role = REDIS_ROLE_SLAVE
		r.MasterLinkStatus = true
		Slaves = append(Slaves, &r)
	}

	var m1, m2 Redis
	m1.EndPoint = "127.0.0.1:6319"
	m1.SyncBytes = 1001
	m1.Role = REDIS_ROLE_MASTER
	Slaves = append(Slaves, &m1)

	m2.EndPoint = "127.0.0.1:6316"
	m2.SyncBytes = 1100
	m2.Role = REDIS_ROLE_MASTER
	Slaves = append(Slaves, &m2)

	OldMaster, NewMaster := FindNxtMaster(Slaves)

	if OldMaster != nil {
		t.Errorf("Old master should be nil, as its misconfigued")
		t.Fail()
	}

	//Cannot be nill
	if NewMaster == nil {
		t.Errorf("NewMaster cannot be nil")
		t.Fail()
	}
	if NewMaster.SyncBytes != 1100 {
		t.Errorf("NewMaster SyncBytes Exp=1100 Obtained=%d", NewMaster.SyncBytes)
		t.Fail()
	}
}
