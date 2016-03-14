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

/*
Much love and respect to the redhat/origin team on much of this code!
*/

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	knet "k8s.io/kubernetes/pkg/util/net"
)

const (
	baseURL = "https://%s/mgmt/tm"
)

// f5Config holds configuration for the f5 plugin.
type f5Config struct {
	// Host specifies the hostname or IP address of the F5 BIG-IP host.
	host string

	// Username specifies the username with the plugin should authenticate
	// with the F5 BIG-IP host.
	username string

	// Password specifies the password with which the plugin should
	// authenticate with F5 BIG-IP.
	password string

	// Insecure specifies whether the F5 plugin should perform strict certificate
	// validation for connections to the F5 BIG-IP host.
	insecure bool

	// PartitionPath specifies the F5 partition path to use. This is used
	// to create an access control boundary for users and applications.
	partitionPath string

	// FullURL is the fully qualified path to the F5
	fullURL string
}

// f5Pool represents an F5 BIG-IP LTM pool.  It describes the payload for a POST
// request by which the F5 router creates a new pool.
type f5Pool struct {
	// Mode is the method of load balancing that F5 BIG-IP employs over members of
	// the pool.  The F5 router uses round-robin; other allowed values are
	// dynamic-ratio-member, dynamic-ratio-node, fastest-app-response,
	// fastest-node, least-connections-node, least-sessions, observed-member,
	// observed-node, ratio-member, ratio-node, ratio-session,
	// ratio-least-connections-member, ratio-least-connections-node, and
	// weighted-least-connections-member.
	Mode string `json:"loadBalancingMode"`

	// Monitor is the name of the monitor associated with the pool.  The F5 router
	// uses /Common/http.
	Monitor string `json:"monitor"`

	// Name is the name of the pool.  The F5 router uses names of the form
	// openshift_<namespace>_<servicename>.
	Name string `json:"name"`
}

// f5PoolMember represents an F5 BIG-IP LTM pool member.  The F5 router uses it
// within f5PoolMemberset to unmarshal the JSON response when requesting a pool
// from F5.  f5PoolMember also describes the payload for a POST request by which
// the F5 router adds a member to a pool.
type f5PoolMember struct {
	// Name is the name of the pool member.  The F5 router uses names of the form
	// ipaddr:port.
	Name string `json:"name"`
}

// f5PoolMemberset represents an F5 BIG-IP LTM pool.  The F5 router uses it to
// unmarshal the JSON response when requesting a pool from F5.
type f5PoolMemberset struct {
	// Members is an array of pool members, which are represented using
	// f5PoolMember objects.
	Members []f5PoolMember `json:"items"`
}

type f5VirtualServer struct {
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Destination string      `json:"destination"`
	Pool        string      `json:"pool"`
	Mask        string      `json:"mask"`
	IpProtocol  string      `json:"ipProtocol"`
	Profiles    []f5Profile `json:"profiles"`
}

type f5Profile struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

const (
	// Default F5 partition path to use for syncing route config.
	F5DefaultPartitionPath = "/Common"
)

// Error implements the error interface.
func (err F5Error) Error() string {
	var msg string

	if err.err != nil {
		msg = fmt.Sprintf("error: %v", err.err)
	} else if err.Message != nil {
		msg = fmt.Sprintf("HTTP code: %d; error from F5: %s",
			err.httpStatusCode, *err.Message)
	} else {
		msg = fmt.Sprintf("HTTP code: %d.", err.httpStatusCode)
	}

	return fmt.Sprintf("Encountered an error on %s request to URL %s: %s",
		err.verb, err.url, msg)
}

// F5Error represents an error resulting from a request to the F5 BIG-IP
// iControl REST interface.
type F5Error struct {
	// f5result holds the standard header (code and message) that is included in
	// responses from F5.
	f5Result

	// verb is the HTTP verb (GET, POST, PUT, PATCH, or DELETE) that was
	// used in the request that resulted in the error.
	verb string

	// url is the URL that was used in the request that resulted in the error.
	url string

	// httpStatusCode is the HTTP response status code (e.g., 200, 404, etc.).
	httpStatusCode int

	// err contains a descriptive error object for error cases other than HTTP
	// errors (i.e., non-2xx responses), such as socket errors or malformed JSON.
	err error
}

// f5Result represents an F5 BIG-IP LTM request response.  f5Result is used to
// unmarshal the JSON response when receiving responses from the F5 iControl
// REST API.  These responses generally are JSON blobs containing at least the
// fields described in this structure.
//
// f5Result may be embedded into other types for requests that return objects.
type f5Result struct {
	// Code should match the HTTP status code.
	Code int `json:"code"`

	// Message should contain a short description of the result of the requested
	// operation.
	Message *string `json:"message"`
}

// f5LTM represents an F5 BIG-IP instance.
type f5LTM struct {
	// f5Config contains the configuration parameters for an F5 BIG-IP instance.
	f5Config

	// poolMembers maps pool name to set of pool members, where the pool
	// name is a string and the set of members of a pool is represented by
	// a map with value type bool.  A pool member will be identified by
	// a string of the format "ipaddress:port".
	poolMembers map[string]map[string]bool
}

func newf5LTM(host, user, password, partition string, insecure bool) *f5LTM {
	var FullURL = fmt.Sprintf("https://%s/mgmt/tm", host)

	ctrl := f5LTM{
		f5Config: f5Config{
			host:          host,
			username:      user,
			password:      password,
			insecure:      insecure,
			partitionPath: partition,
			fullURL:       FullURL,
		},
		poolMembers: map[string]map[string]bool{},
	}

	return &ctrl
}

// Creates a Virtual Server and Pool with Members
func (f5 *f5LTM) CreateLB(httpSvc []service, httpsTermSvc []service, tcpSvc []service,
	nodes []string, serviceName string, namespace string, specPorts []api.ServicePort,
	virtualServerIp string) {

	for _, servicePort := range specPorts {
		// TODO: headless services?
		sName := namespace + "-" + serviceName + strconv.Itoa(servicePort.Port)
		if servicePort.Protocol == api.ProtocolUDP {
			glog.Infof("Ignoring %v: %+v", sName, servicePort)
			continue
		}

		glog.Info("About to setup service as type loadbalancer in f5: ", serviceName)

		// --Create F5 LoadBalancer--
		// 1. Create the pool
		// 2. Add servers as members to the pool with NodePorts
		// 3. Create virtual server using pool and IP passed in

		poolExists, err := f5.PoolExists(sName)
		if err != nil {
			fmt.Println("Error checking if pool exists! ", err)
		}

		if poolExists != true {
			// Create Pool
			err = f5.CreatePool(sName)
			if err != nil {
				glog.Info("Error creating pool! ", err)
			}

			// Add members to pool
			for _, node := range nodes {
				err = f5.AddPoolMember(sName, node+":"+strconv.Itoa(servicePort.NodePort))
				if err != nil {
					glog.Info("Error adding member to pool! ", err)
				}
			}

			// Create virtual server
			err = f5.CreateVirtualServer(sName, fmt.Sprintf("%s:%s", virtualServerIp, servicePort.Port), "255.255.255.255", sName)
			if err != nil {
				glog.Info("Error creating virtual server! ", err)
			}

		} else {
			glog.Info("Skipping service as a pool already exists.")
		}

		// fmt.Println("got a create for:", newSvc.Name)
		// lbc.ibc.createHost(newSvc.Name, "", nodes)
	}
}

// Deletes VirtualServer and Pool
func (f5 *f5LTM) DeleteLB(serviceName string, namespace string, specPorts []api.ServicePort) {
	for _, servicePort := range specPorts {
		// TODO: headless services?
		sName := namespace + "-" + serviceName + strconv.Itoa(servicePort.Port)
		err := f5.DeletePool(sName)
		if err != nil {
			glog.Error("Could not delete pool: ", sName)
		}

		err = f5.DeleteVirtualServer(sName)
		if err != nil {
			glog.Error("Could not delete virtual server: ", sName)
		}
	}
}

//
// Helper routines for REST calls.
//

// rest_request makes a REST request to the F5 BIG-IP host's F5 iControl REST
// API.
//
// One of three things can happen as a result of a request to F5 iControl REST:
//
// (1) The request succeeds and F5 returns an HTTP 200 response, possibly with
//     a JSON result payload, which should have the fields defined in the
//     result argument.  In this case, rest_request decodes the payload into
//     the result argument and returns nil.
//
// (2) The request fails and F5 returns an HTTP 4xx or 5xx response with a
//     response payload.  Usually, this payload is JSON containing a numeric
//     code (which should be the same as the HTTP response code) and a string
//     message.  However, in some cases, the F5 iControl REST API returns an
//     HTML response payload instead.  rest_request attempts to decode the
//     response payload as JSON but ignores decoding failures on the assumption
//     that a failure to decode means that the response was in HTML.  Finally,
//     rest_request returns an F5Error with the URL, HTTP verb, HTTP status
//     code, and (if the response was JSON) error information from the response
//     payload.
//
// (3) The REST call fails in some other way, such as a socket error or an
//     error decoding the result payload.  In this case, rest_request returns
//     an F5Error with the URL, HTTP verb, HTTP status code (if any), and error
//     value.
func (f5 *f5LTM) rest_request(verb string, url string, payload io.Reader,
	result interface{}) error {

	tr := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: f5.f5Config.insecure},
	})

	errorResult := F5Error{verb: verb, url: url}

	req, err := http.NewRequest(verb, url, payload)
	if err != nil {
		errorResult.err = fmt.Errorf("http.NewRequest failed: %v", err)
		return errorResult
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(f5.f5Config.username, f5.f5Config.password)

	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		errorResult.err = fmt.Errorf("client.Do failed: %v", err)
		return errorResult
	}

	defer resp.Body.Close()

	errorResult.httpStatusCode = resp.StatusCode

	decoder := json.NewDecoder(resp.Body)
	if resp.StatusCode >= 400 {
		// F5 sometimes returns an HTML response even though we ask for JSON.
		// If decoding fails, assume we got an HTML response and ignore both
		// the response and the error.
		decoder.Decode(&errorResult)
		return errorResult
	} else if result != nil {
		err = decoder.Decode(result)
		if err != nil {
			errorResult.err = fmt.Errorf("Decoder.Decode failed: %v", err)
			return errorResult
		}
	}

	return nil
}

// rest_request_payload is a helper for F5 operations that take
// a payload.
func (f5 *f5LTM) rest_request_payload(verb string, url string,
	payload interface{}, result interface{}) error {
	jsonStr, err := json.Marshal(payload)
	if err != nil {
		return F5Error{verb: verb, url: url, err: err}
	}

	encodedPayload := bytes.NewBuffer(jsonStr)

	return f5.rest_request(verb, url, encodedPayload, result)
}

// get issues a GET request against the F5 iControl REST API.
func (f5 *f5LTM) get(url string, result interface{}) error {
	return f5.rest_request("GET", url, nil, result)
}

// post issues a POST request against the F5 iControl REST API.
func (f5 *f5LTM) post(url string, payload interface{}, result interface{}) error {
	return f5.rest_request_payload("POST", url, payload, result)
}

// patch issues a PATCH request against the F5 iControl REST API.
func (f5 *f5LTM) patch(url string, payload interface{}, result interface{}) error {
	return f5.rest_request_payload("PATCH", url, payload, result)
}

// delete issues a DELETE request against the F5 iControl REST API.
func (f5 *f5LTM) delete(url string, result interface{}) error {
	return f5.rest_request("DELETE", url, nil, result)
}

// checkPartitionPathExists checks if the partition path exists.
func (f5 *f5LTM) checkPartitionPathExists(pathName string) (bool, error) {
	glog.V(4).Infof("Checking if partition path %q exists...", pathName)

	// F5 iControl REST API expects / characters in the path to be
	// escaped as ~.
	uri := fmt.Sprintf("https://%s/mgmt/tm/sys/folder/%s",
		f5.f5Config.host, strings.Replace(pathName, "/", "~", -1))

	err := f5.get(uri, nil)
	if err != nil {
		if err.(F5Error).httpStatusCode != 404 {
			glog.Errorf("partition path %q error: %v", pathName, err)
			return false, err
		}

		//  404 is ok means that the path doesn't exist == !err.
		return false, nil
	}

	glog.V(4).Infof("Partition path %q exists.", pathName)
	return true, nil
}

// CreatePool creates a pool named poolname on F5 BIG-IP.
func (f5 *f5LTM) CreatePool(poolname string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool", f5.f5Config.host)

	// The http monitor is still used from the /Common partition.
	// From @Miciah: In the future, we should allow the administrator
	// to specify a different monitor to use.
	payload := f5Pool{
		Mode:    "round-robin",
		Monitor: "/Common/http",
		Name:    poolname,
	}

	err := f5.post(url, payload, nil)
	if err != nil {
		return err
	}

	// We don't really need to initialise f5.poolMembers[poolname] because
	// we always check whether it is initialised before using it, but
	// initialising it to an empty map here saves a REST call later the first
	// time f5.PoolHasMember is invoked with poolname.
	f5.poolMembers[poolname] = map[string]bool{}

	glog.V(4).Infof("Pool %s created.", poolname)

	return nil
}

// DeletePool deletes the specified pool from F5 BIG-IP, and deletes
// f5.poolMembers[poolname].
func (f5 *f5LTM) DeletePool(poolname string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s", f5.host, poolname)

	err := f5.delete(url, nil)
	if err != nil {
		return err
	}

	// Note: We *must* use delete here rather than merely assigning false because
	// len() includes false items, and we want len() to return an accurate count
	// of members.  Also, we probably save some memory by using delete.
	delete(f5.poolMembers, poolname)

	glog.V(4).Infof("Pool %s deleted.", poolname)

	return nil
}

// GetPoolMembers returns f5.poolMembers[poolname], first initializing it from
// F5 if it is zero.
func (f5 *f5LTM) GetPoolMembers(poolname string) (map[string]bool, error) {
	members, ok := f5.poolMembers[poolname]
	if ok {
		return members, nil
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members",
		f5.host, poolname)

	res := f5PoolMemberset{}

	err := f5.get(url, &res)
	if err != nil {
		return nil, err
	}

	// Note that we do not initialise f5.poolMembers[poolname] unless we know that
	// the pool exists (i.e., the above GET request for the pool succeeds).
	f5.poolMembers[poolname] = map[string]bool{}

	for _, member := range res.Members {
		f5.poolMembers[poolname][member.Name] = true
	}

	return f5.poolMembers[poolname], nil
}

// PoolExists checks whether the specified pool exists.  Internally, PoolExists
// uses f5.poolMembers[poolname], as a side effect initialising it if it is
// zero.
func (f5 *f5LTM) PoolExists(poolname string) (bool, error) {
	_, err := f5.GetPoolMembers(poolname)
	if err == nil {
		return true, nil
	}

	if err.(F5Error).httpStatusCode == 404 {
		return false, nil
	}

	return false, err
}

// PoolHasMember checks whether the given member is in the specified pool on F5
// BIG-IP.  Internally, PoolHasMember uses f5.poolMembers[poolname], causing it
// to be initialised first if it is zero.
func (f5 *f5LTM) PoolHasMember(poolname, member string) (bool, error) {
	members, err := f5.GetPoolMembers(poolname)
	if err != nil {
		return false, err
	}

	return members[member], nil
}

// AddPoolMember adds the given member to the specified pool on F5 BIG-IP, and
// updates f5.poolMembers[poolname].
func (f5 *f5LTM) AddPoolMember(poolname, member string) error {
	hasMember, err := f5.PoolHasMember(poolname, member)
	if err != nil {
		return err
	}
	if hasMember {
		glog.V(4).Infof("Pool %s already has member %s.\n", poolname, member)
		return nil
	}

	glog.V(4).Infof("Adding pool member %s to pool %s.", member, poolname)

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members",
		f5.host, poolname)

	payload := f5PoolMember{
		Name: member,
	}

	err = f5.post(url, payload, nil)
	if err != nil {
		return err
	}

	members, err := f5.GetPoolMembers(poolname)
	if err != nil {
		return err
	}

	members[member] = true

	glog.V(4).Infof("Added pool member %s to pool %s.",
		member, poolname)

	return nil
}

// DeletePoolMember deletes the given member from the specified pool on F5
// BIG-IP, and updates f5.poolMembers[poolname].
func (f5 *f5LTM) DeletePoolMember(poolname, member string) error {
	// The invocation of f5.PoolHasMember has the side effect that it will
	// initialise f5.poolMembers[poolname], which is used below, if necessary.
	hasMember, err := f5.PoolHasMember(poolname, member)
	if err != nil {
		return err
	}
	if !hasMember {
		glog.V(4).Infof("Pool %s does not have member %s.\n", poolname, member)
		return nil
	}

	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/pool/%s/members/%s",
		f5.host, poolname, member)

	err = f5.delete(url, nil)
	if err != nil {
		return err
	}

	delete(f5.poolMembers[poolname], member)

	glog.V(4).Infof("Pool member %s deleted from pool %s.", member, poolname)

	return nil
}

// CreateVirtualServer creates a virtual server on F5 BIG-IP.
func (f5 *f5LTM) CreateVirtualServer(serverName, destination, mask, poolname string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual", f5.f5Config.host)

	payload := f5VirtualServer{
		Kind:        "tm:ltm:virtual:virtualstate",
		Name:        serverName,
		Description: fmt.Sprintf("Kubernetes vServer for service: %s", serverName),
		Mask:        mask,
		Pool:        poolname,
		Destination: destination,
		// SourceAddressTranslation: "{'type': \"automap\"}",
		IpProtocol: "tcp",
		Profiles: []f5Profile{
			{Kind: "ltm:virtual:profile", Name: "http"},
			{Kind: "ltm:virtual:profile", Name: "tcp"},
		},
	}

	err := f5.post(url, payload, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Virtual Server %s created.", serverName)

	return nil
}

// DeleteVirtualServer deletes a virtual server on F5 BIG-IP.
func (f5 *f5LTM) DeleteVirtualServer(serverName string) error {
	url := fmt.Sprintf("https://%s/mgmt/tm/ltm/virtual/%s", f5.f5Config.host, serverName)

	err := f5.delete(url, nil)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Virtual Server %s deleted.", serverName)

	return nil
}
