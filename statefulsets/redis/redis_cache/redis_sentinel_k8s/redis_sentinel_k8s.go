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
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"encoding/json"
	log "github.com/golang/glog"
	"github.com/mediocregopher/radix.v2/redis"
	"time"
)

//Constants to be used in the program
const (
	COLLECTSTATS_INVALID_INPUT        = "Invalid Endpoint"
	COLLECTSTATS_SERVER_NOT_REACHABLE = "Redis Server Not Reachable"
	REDIS_ROLE_MASTER                 = "master"
	REDIS_ROLE_SLAVE                  = "slave"
	LOOKUP_RETIRES                    = 10
)

//Redis This type will contain the parsed info of each and every redis-instance we are going to operate on
type Redis struct {
	EndPoint         string        //End point of the redis-server
	Role             string        //Role of this redis-sever Master or Slave
	LastUpdated      int           // When did last sync happened seconds
	MasterDownSince  int           //Since how long the master is not reachable?
	SyncBytes        int64         //How much of data did this sync
	MasterHost       string        //Masters ip addres
	MasterPort       string        //Master port
	Priority         int           //Slave priority
	MasterLinkStatus bool          //true means up false mean down
	Client           *redis.Client //Redis client
}

//RedisSlaves Array of redis we need this to make sorting easier
type RedisSlaves []*Redis

func (rs RedisSlaves) Len() int { return len(rs) }
func (rs RedisSlaves) Swap(i int, j int) {
	var tmp *Redis
	tmp = rs[i]
	rs[i] = rs[j]
	rs[j] = tmp
}

func (rs RedisSlaves) Less(i int, j int) bool {

	//Choose the slave with least priority
	if rs[i].Priority != 0 && rs[j].Priority != 0 {
		if rs[i].Priority < rs[j].Priority {
			return true
		}
	}

	//Choose the slave with maximum SyncBytes
	if rs[i].SyncBytes > rs[j].SyncBytes {
		return true
	}

	//Choose the slave with least Updated time
	if rs[i].LastUpdated < rs[j].LastUpdated {
		return true
	}

	return false
}

//ParseResponse This function will convert the text output of 'info replication' and populate fields in the type R
func (R *Redis) ParseResponse(Res string) bool {

	res := strings.Split(Res, "\\r\\n")
	if len(res) == 1 {
		log.Errorf("ParseResponse(): Invalid Redis-server response. Nothing to parse")
		return false
	}

	for _, field := range res {

		kv := strings.Split(field, ":")
		if len(kv) == 2 {
			switch kv[0] {
			case "role":
				R.Role = kv[1]
			case "master_host":
				R.MasterHost = kv[1]
			case "master_port":
				R.MasterPort = kv[1]
			case "slave_repl_offset":
				i, err := strconv.Atoi(kv[1])
				if err == nil {
					R.SyncBytes = int64(i)
				}
			case "master_repl_offset":
				i, err := strconv.Atoi(kv[1])
				if err == nil && i > 0 {
					R.SyncBytes = int64(i)
				}
			case "master_link_down_since_seconds":
				i, err := strconv.Atoi(kv[1])
				if err == nil {
					R.MasterDownSince = i
				}
			case "master_link_status":
				if kv[1] == "on" {
					R.MasterLinkStatus = true
				} else {
					R.MasterLinkStatus = false
				}
			case "master_last_io_seconds_ago":
				i, err := strconv.Atoi(kv[1])
				if err == nil {
					R.LastUpdated = i
				}
			case "slave_priority":
				i, err := strconv.Atoi(kv[1])
				if err == nil {
					R.Priority = i
				}
			}
		}
	}
	fmt.Printf("R=%v\n", R)
	return true
}

//CollectStats This function will take the endpoint
func CollectStats(EndPoint string) (*Redis, error) {

	var R Redis

	//if this is comming for kuberntes add the pronumber yourself
	if strings.Contains(EndPoint, "svc") {
		EndPoint += ":6379"
	}

	//Check if the supplied EP is valid
	if len(strings.Split(EndPoint, ":")) != 2 {
		return nil, fmt.Errorf(COLLECTSTATS_INVALID_INPUT)
	}

	//Try to connect to the redis-servers
	C, err := redis.Dial("tcp", EndPoint)
	if err != nil {
		log.Infof("CollectStats() %s Error:%v", COLLECTSTATS_SERVER_NOT_REACHABLE, err)
		return nil, fmt.Errorf(COLLECTSTATS_SERVER_NOT_REACHABLE)
	}
	Res := C.Cmd("INFO", "REPLICATION")

	//log.Infof("CollectStats(%s)=%v", EndPoint, Res.String())
	R.ParseResponse(Res.String())

	R.EndPoint = EndPoint
	R.Client = C
	return &R, nil
}

//CollectStatsAll Contact all the redis containers and collect statistics required to perform a Slave Promotion
func CollectStatsAll(EndPoints []string) []*Redis {

	var Servers []*Redis

	var wg sync.WaitGroup
	var lck sync.Mutex

	for _, S := range EndPoints {
		log.Infof("Processing %v", S)
		wg.Add(1)
		go func(S string) {
			defer wg.Done()
			R, err := CollectStats(S)
			if err == nil {
				lck.Lock()
				Servers = append(Servers, R)
				lck.Unlock()
			} else {
				log.Warningf("Error collecting stats for %v Error=%v", S, err)
			}
		}(S)
	}
	wg.Wait()
	return Servers
}

//FindNxtMaster This function will return next suitable master if there is such a situation otherwise it simply returns nil, for instance if the supplied list of containrs already form a proper master-slave cluster then it will leave the setup intact.
func FindNxtMaster(Servers []*Redis) (*Redis, *Redis) {

	var Slaves []*Redis

	//Check if Master is already there
	var isMasterAvailable bool
	var availableMaster *Redis
	var availableMasterHits int

	//Loop through all the servers and find of if there is already a master
	for _, rs := range Servers {
		//TODO: There might be a situation where there are multiple mis-configured masters, should handle that later
		if strings.Contains(rs.Role, REDIS_ROLE_MASTER) {

			isMasterAvailable = true
			availableMaster = rs
			break
		}

	}

	for _, rs := range Servers {
		if isMasterAvailable {
			if rs.EndPoint != availableMaster.EndPoint {
				Slaves = append(Slaves, rs)
				log.Infof("RSMaster_EP=%s available MasterEP=%v", rs.MasterHost+":"+rs.MasterPort, availableMaster.EndPoint)
				if (rs.MasterHost+":"+rs.MasterPort == availableMaster.EndPoint) && rs.MasterLinkStatus {
					availableMasterHits++
				}
			}
		} else {
			Slaves = append(Slaves, rs)
		}
	}
	//If master is available check if its already pouparly configured
	if isMasterAvailable {

		if availableMaster.SyncBytes > 0 && availableMasterHits == len(Slaves) {

			//Looks like the master is active and configured properly
			log.Warningf("The redis master is already configured, dont do anything SyncBytes=%v availableMasterHits=%v len(Slaves)=%v", availableMaster.SyncBytes, availableMasterHits, len(Slaves))
			return availableMaster, nil
		}
		log.Warningf("A Redis master is found, but misconfigured, considering it as a slave")
		Slaves = append(Slaves, availableMaster)

	}

	if len(Slaves) == 0 {
		return availableMaster, nil
	}

	//Sort the slaves according to the parameters
	sort.Sort(RedisSlaves(Slaves))

	//return the selected slaves
	return nil, Slaves[0]
}

//PromoteASlave It will look at all the eligible redis-servers and promote the most eligible one as a new master
func PromoteASlave(NewMaster *Redis, Servers []*Redis) bool {

	result := true

	//Make the slave as the master first
	resp := NewMaster.Client.Cmd("SLAVEOF", "NO", "ONE").String()
	if !strings.Contains(resp, "OK") {
		log.Errorf("Unable to make the slave as master response=%v", resp)
		return false
	}

	hostPort := strings.Split(NewMaster.EndPoint, ":")
	NewMaster.MasterHost = hostPort[0]
	NewMaster.MasterPort = hostPort[1]

	for _, rs := range Servers {

		if rs.EndPoint == NewMaster.EndPoint {
			continue
		}
		resp = rs.Client.Cmd("SLAVEOF", NewMaster.MasterHost, NewMaster.MasterPort).String()
		if !strings.Contains(resp, "OK") {
			log.Errorf("Unable to make the slave point to new master response=%v", resp)
			return false
		}
		//Make the slaves replication timeout as small as possible
		resp = rs.Client.Cmd("config", "set", "repl-ping-slave-period", "1").String()
		if !strings.Contains(resp, "OK") {
			log.Errorf("Unable to make slave ping frequenc to 1 second=%v", resp)
			return false
		}

		resp = rs.Client.Cmd("config", "set", "repl-timeout", "5").String()
		if !strings.Contains(resp, "OK") {
			log.Errorf("Unable to make replication timout to 5 seconds = %v", resp)
			return false
		}
	}
	return result

}

//LookupSrv Given a Kubernetes service name lookup its endpoints
func LookupSrv(svcName string) ([]string, error) {

	var endpoints []string

	log.V(2).Infof("lookup(%s)", svcName)
	_, srvRecords, err := net.LookupSRV("", "", svcName)
	if err != nil {
		return endpoints, err
	}
	for _, srvRecord := range srvRecords {
		// The SRV records ends in a "." for the root domain
		ep := fmt.Sprintf("%v", srvRecord.Target[:len(srvRecord.Target)-1])
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

//PrintServers A Debug function that prints state of the redis servers along with supplied 'message'
func PrintServers(message string, Servers []*Redis) {
	var result string
	result = fmt.Sprintf("PrintServers()\n")
	result += fmt.Sprintf("******%s******\n", message)
	r, _ := json.MarshalIndent(Servers, "", "  ")
	result += string(r)
	result += fmt.Sprintf("*****************\n")
	log.V(2).Info(result)
}

func main() {

	svc := flag.String("service", "cache", "Provide the redis statefulset's service name")
	flag.Set("logtostderr", "true")
	flag.Parse()

	//Lookup for the provided service, if not available, retry a few times
	lookupRetry := 0
	var ServersEndPoint []string
	var err error
	for {
		ServersEndPoint, err = LookupSrv(*svc)
		if err != nil {
			if lookupRetry <= LOOKUP_RETIRES {
				log.Errorf("Unable to lookup for the service err:%v", err)
				os.Exit(1)
			}
			log.Infof("Service not ready retrying after 5 seconds err=%V", err)
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}

	log.Infof("Available endpoints are %v", ServersEndPoint)

	//Collect stats on all the redis-servers supplied
	Servers := CollectStatsAll(ServersEndPoint)

	if len(Servers) == 0 {
		log.Infof("The cluster is empty or all redis-servers are not reachable")
		os.Exit(0)
	}

	PrintServers("Supplied Servers", Servers)

	//Does it really need a master
	OldMaster, NewMaster := FindNxtMaster(Servers)
	log.Infof("OldMaster=%v NewMaster=%v", OldMaster, NewMaster)

	if NewMaster == nil && OldMaster != nil {

		log.Errorf("Redis Instance does'nt need a Slave Promotion")
		NewMaster = OldMaster

	} else if OldMaster == nil && NewMaster != nil {

		//Now we have a potential master
		if !PromoteASlave(NewMaster, Servers) {

			PrintServers("In-consistantly configured", Servers)
			log.Errorf("Error occured in Slave Promotion")
			os.Exit(1)
		}
		log.Infof("New Master is %v, All the slaves are re-configured to replicate from this", NewMaster.EndPoint)
		PrintServers("Processed Servers", Servers)

	} else {
		//Both are nil or both are valid
		log.Errorf("Inconsitant Redis Cluster")
		return
	}
	//At this point

	//write the master information to /config/master.txt
	f, err := os.Create("/config/master.txt")
	if err != nil {
		log.Errorf("Unable to open the config file err:%v", err)
		os.Exit(1)
	}
	defer f.Close()
	_, err = f.WriteString(NewMaster.MasterHost + " " + NewMaster.MasterPort)
	if err != nil {
		log.Errorf("Error writing to the config file err:%v", err)

		os.Exit(1)
	}
	log.Infof("Redis-Sentinal-micro Finished")

}
