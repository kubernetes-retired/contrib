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

package gce

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/cloudprovider"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/golang/glog"
	"github.com/scalingdata/gcfg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/cloud/compute/metadata"
)

const (
	ProviderName = "gce"

	k8sNodeRouteTag = "k8s-node-route"

	// AffinityTypeNone - no session affinity.
	gceAffinityTypeNone = "NONE"
	// AffinityTypeClientIP - affinity based on Client IP.
	gceAffinityTypeClientIP = "CLIENT_IP"
	// AffinityTypeClientIPProto - affinity based on Client IP and port.
	gceAffinityTypeClientIPProto = "CLIENT_IP_PROTO"

	operationPollInterval        = 3 * time.Second
	operationPollTimeoutDuration = 30 * time.Minute
)

// GCECloud is an implementation of Interface, LoadBalancer and Instances for Google Compute Engine.
type GCECloud struct {
	service           *compute.Service
	containerService  *container.Service
	projectID         string
	zone              string
	networkURL        string
	useMetadataServer bool
}

type Config struct {
	Global struct {
		TokenURL    string `gcfg:"token-url"`
		TokenBody   string `gcfg:"token-body"`
		ProjectID   string `gcfg:"project-id"`
		NetworkName string `gcfg:"network-name"`
	}
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) { return newGCECloud(config) })
}

func getProjectAndZone() (string, string, error) {
	result, err := metadata.Get("instance/zone")
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(result, "/")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("unexpected response: %s", result)
	}
	zone := parts[3]
	projectID, err := metadata.ProjectID()
	if err != nil {
		return "", "", err
	}
	return projectID, zone, nil
}

func getInstanceIDViaMetadata() (string, error) {
	result, err := metadata.Get("instance/hostname")
	if err != nil {
		return "", err
	}
	parts := strings.Split(result, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("unexpected response: %s", result)
	}
	return parts[0], nil
}

func getCurrentExternalIDViaMetadata() (string, error) {
	externalID, err := metadata.Get("instance/id")
	if err != nil {
		return "", fmt.Errorf("couldn't get external ID: %v", err)
	}
	return externalID, nil
}

func getNetworkNameViaMetadata() (string, error) {
	result, err := metadata.Get("instance/network-interfaces/0/network")
	if err != nil {
		return "", err
	}
	parts := strings.Split(result, "/")
	if len(parts) != 4 {
		return "", fmt.Errorf("unexpected response: %s", result)
	}
	return parts[3], nil
}

func getNetworkNameViaAPICall(svc *compute.Service, projectID string) (string, error) {
	networkList, err := svc.Networks.List(projectID).Do()
	if err != nil {
		return "", err
	}

	if networkList == nil || len(networkList.Items) <= 0 {
		return "", fmt.Errorf("GCE Network List call returned no networks for project %q.", projectID)
	}

	return networkList.Items[0].Name, nil
}

// newGCECloud creates a new instance of GCECloud.
func newGCECloud(config io.Reader) (*GCECloud, error) {
	projectID, zone, err := getProjectAndZone()
	if err != nil {
		return nil, err
	}

	networkName, err := getNetworkNameViaMetadata()
	if err != nil {
		return nil, err
	}
	networkURL := gceNetworkURL(projectID, networkName)

	tokenSource := google.ComputeTokenSource("")
	if config != nil {
		var cfg Config
		if err := gcfg.ReadInto(&cfg, config); err != nil {
			glog.Errorf("Couldn't read config: %v", err)
			return nil, err
		}
		if cfg.Global.ProjectID != "" {
			projectID = cfg.Global.ProjectID
		}
		if cfg.Global.NetworkName != "" {
			if strings.Contains(cfg.Global.NetworkName, "/") {
				networkURL = cfg.Global.NetworkName
			} else {
				networkURL = gceNetworkURL(cfg.Global.ProjectID, cfg.Global.NetworkName)
			}
		}
		if cfg.Global.TokenURL != "" {
			tokenSource = newAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody)
		}
	}

	return CreateGCECloud(projectID, zone, networkURL, tokenSource, true /* useMetadataServer */)
}

// Creates a GCECloud object using the specified parameters.
// If no networkUrl is specified, loads networkName via rest call.
// If no tokenSource is specified, uses oauth2.DefaultTokenSource.
func CreateGCECloud(projectID, zone, networkURL string, tokenSource oauth2.TokenSource, useMetadataServer bool) (*GCECloud, error) {
	if tokenSource == nil {
		var err error
		tokenSource, err = google.DefaultTokenSource(
			oauth2.NoContext,
			compute.CloudPlatformScope,
			compute.ComputeScope)
		glog.Infof("Using DefaultTokenSource %#v", tokenSource)
		if err != nil {
			return nil, err
		}
	} else {
		glog.Infof("Using existing Token Source %#v", tokenSource)
	}

	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	svc, err := compute.New(client)
	if err != nil {
		return nil, err
	}

	containerSvc, err := container.New(client)
	if err != nil {
		return nil, err
	}

	if networkURL == "" {
		networkName, err := getNetworkNameViaAPICall(svc, projectID)
		if err != nil {
			return nil, err
		}
		networkURL = gceNetworkURL(projectID, networkName)
	}

	return &GCECloud{
		service:           svc,
		containerService:  containerSvc,
		projectID:         projectID,
		zone:              zone,
		networkURL:        networkURL,
		useMetadataServer: useMetadataServer,
	}, nil
}

func (gce *GCECloud) Clusters() (cloudprovider.Clusters, bool) {
	return gce, true
}

// ProviderName returns the cloud provider ID.
func (gce *GCECloud) ProviderName() string {
	return ProviderName
}

// Known-useless DNS search path.
var uselessDNSSearchRE = regexp.MustCompile(`^[0-9]+.google.internal.$`)

// ScrubDNS filters DNS settings for pods.
func (gce *GCECloud) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	// GCE has too many search paths by default. Filter the ones we know are useless.
	for _, s := range searches {
		if !uselessDNSSearchRE.MatchString(s) {
			srchOut = append(srchOut, s)
		}
	}
	return nameservers, srchOut
}

// LoadBalancer returns an implementation of LoadBalancer for Google Compute Engine.
func (gce *GCECloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return gce, true
}

// Instances returns an implementation of Instances for Google Compute Engine.
func (gce *GCECloud) Instances() (cloudprovider.Instances, bool) {
	return gce, true
}

// Zones returns an implementation of Zones for Google Compute Engine.
func (gce *GCECloud) Zones() (cloudprovider.Zones, bool) {
	return gce, true
}

// Routes returns an implementation of Routes for Google Compute Engine.
func (gce *GCECloud) Routes() (cloudprovider.Routes, bool) {
	return gce, true
}

func makeHostURL(projectID, zone, host string) string {
	host = canonicalizeInstanceName(host)
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
		projectID, zone, host)
}

func makeComparableHostPath(zone, host string) string {
	host = canonicalizeInstanceName(host)
	return fmt.Sprintf("/zones/%s/instances/%s", zone, host)
}

func hostURLToComparablePath(hostURL string) string {
	idx := strings.Index(hostURL, "/zones/")
	if idx < 0 {
		return ""
	}
	return hostURL[idx:]
}

func (gce *GCECloud) targetPoolURL(name, region string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/%s/targetPools/%s", gce.projectID, region, name)
}

func waitForOp(op *compute.Operation, getOperation func(operationName string) (*compute.Operation, error)) error {
	if op == nil {
		return fmt.Errorf("operation must not be nil")
	}

	if opIsDone(op) {
		return getErrorFromOp(op)
	}

	opName := op.Name
	return wait.Poll(operationPollInterval, operationPollTimeoutDuration, func() (bool, error) {
		pollOp, err := getOperation(opName)
		if err != nil {
			glog.Warningf("GCE poll operation failed: %v", err)
		}
		return opIsDone(pollOp), getErrorFromOp(pollOp)
	})
}

func opIsDone(op *compute.Operation) bool {
	return op != nil && op.Status == "DONE"
}

func getErrorFromOp(op *compute.Operation) error {
	if op != nil && op.Error != nil && len(op.Error.Errors) > 0 {
		err := &googleapi.Error{
			Code:    int(op.HttpErrorStatusCode),
			Message: op.Error.Errors[0].Message,
		}
		glog.Errorf("GCE operation failed: %v", err)
		return err
	}

	return nil
}

func (gce *GCECloud) waitForGlobalOp(op *compute.Operation) error {
	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return gce.service.GlobalOperations.Get(gce.projectID, operationName).Do()
	})
}

func (gce *GCECloud) waitForRegionOp(op *compute.Operation, region string) error {
	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return gce.service.RegionOperations.Get(gce.projectID, region, operationName).Do()
	})
}

func (gce *GCECloud) waitForZoneOp(op *compute.Operation) error {
	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return gce.service.ZoneOperations.Get(gce.projectID, gce.zone, operationName).Do()
	})
}

// GetLoadBalancer is an implementation of LoadBalancer.GetLoadBalancer
func (gce *GCECloud) GetLoadBalancer(name, region string) (*api.LoadBalancerStatus, bool, error) {
	fwd, err := gce.service.ForwardingRules.Get(gce.projectID, region, name).Do()
	if err == nil {
		status := &api.LoadBalancerStatus{}
		status.Ingress = []api.LoadBalancerIngress{{IP: fwd.IPAddress}}

		return status, true, nil
	}
	if isHTTPErrorCode(err, http.StatusNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

func isHTTPErrorCode(err error, code int) bool {
	apiErr, ok := err.(*googleapi.Error)
	return ok && apiErr.Code == code
}

// EnsureLoadBalancer is an implementation of LoadBalancer.EnsureLoadBalancer.
// Our load balancers in GCE consist of four separate GCE resources - a static
// IP address, a firewall rule, a target pool, and a forwarding rule. This
// function has to manage all of them.
// Due to an interesting series of design decisions, this handles both creating
// new load balancers and updating existing load balancers, recognizing when
// each is needed.
func (gce *GCECloud) EnsureLoadBalancer(name, region string, requestedIP net.IP, ports []*api.ServicePort, hosts []string, affinityType api.ServiceAffinity) (*api.LoadBalancerStatus, error) {
	glog.V(2).Infof("EnsureLoadBalancer(%v, %v, %v, %v, %v)", name, region, requestedIP, ports, hosts)

	if len(hosts) == 0 {
		return nil, fmt.Errorf("Cannot EnsureLoadBalancer() with no hosts")
	}

	// Check if the forwarding rule exists, and if so, what its IP is.
	fwdRuleExists, fwdRuleNeedsUpdate, fwdRuleIP, err := gce.forwardingRuleNeedsUpdate(name, region, requestedIP, ports)
	if err != nil {
		return nil, err
	}

	// Make sure we know which IP address will be used and have properly reserved
	// it as static before moving forward with the rest of our operations.
	//
	// We use static IP addresses when updating a load balancer to ensure that we
	// can replace the load balancer's other components without changing the
	// address its service is reachable on. We do it this way rather than always
	// keeping the static IP around even though this is more complicated because
	// it makes it less likely that we'll run into quota issues. Only 7 static
	// IP addresses are allowed per region by default.
	//
	// We could let an IP be allocated for us when the forwarding rule is created,
	// but we need the IP to set up the firewall rule, and we want to keep the
	// forwarding rule creation as the last thing that needs to be done in this
	// function in order to maintain the invariant that "if the forwarding rule
	// exists, the LB has been fully created".
	ipAddress := ""
	if requestedIP != nil {
		// If a specific IP address has been requested, we have to respect the
		// user's request and use that IP. If the forwarding rule was already using
		// a different IP, it will be harmlessly abandoned because it was only an
		// ephemeral IP (or it was a different static IP owned by the user, in which
		// case we shouldn't delete it anyway).
		if err := gce.projectOwnsStaticIP(name, region, requestedIP.String()); err != nil {
			return nil, err
		}
		ipAddress = requestedIP.String()
	} else {
		// This will either allocate a new static IP if the forwarding rule didn't
		// already have an IP, or it will promote the forwarding rule's IP from
		// ephemeral to static.
		ipAddress, err = gce.createOrPromoteStaticIP(name, region, fwdRuleIP)
		if err != nil {
			return nil, err
		}
	}

	// Deal with the firewall next. The reason we do this here rather than last
	// is because the forwarding rule is used as the indicator that the load
	// balancer is fully created - it's what getLoadBalancer checks for.
	firewallExists, firewallNeedsUpdate, err := gce.firewallNeedsUpdate(name, region, ipAddress, ports)
	if err != nil {
		return nil, err
	}

	if firewallNeedsUpdate {
		// Unlike forwarding rules and target pools, firewalls can be updated
		// without needing to be deleted and recreated.
		if firewallExists {
			if err := gce.updateFirewall(name, region, ipAddress, ports, hosts); err != nil {
				return nil, err
			}
		} else {
			if err := gce.createFirewall(name, region, ipAddress, ports, hosts); err != nil {
				return nil, err
			}
		}
	}

	tpExists, tpNeedsUpdate, err := gce.targetPoolNeedsUpdate(name, region, affinityType)
	if err != nil {
		return nil, err
	}

	// Now we get to some slightly more interesting logic.
	// First, neither target pools nor forwarding rules can be updated in place -
	// they have to be deleted and recreated.
	// Second, forwarding rules are layered on top of target pools in that you
	// can't delete a target pool that's currently in use by a forwarding rule.
	// Thus, we have to tear down the forwarding rule if either it or the target
	// pool needs to be updated.
	if fwdRuleExists && (fwdRuleNeedsUpdate || tpNeedsUpdate) {
		if err := gce.deleteForwardingRule(name, region); err != nil {
			return nil, fmt.Errorf("failed to delete existing forwarding rule %s for load balancer update: %v", name, err)
		}
	}
	if tpExists && tpNeedsUpdate {
		if err := gce.deleteTargetPool(name, region); err != nil {
			return nil, fmt.Errorf("failed to delete existing target pool %s for load balancer update: %v", name, err)
		}
	}

	// Once we've deleted the resources (if necessary), build them back up (or for
	// the first time if they're new).
	if tpNeedsUpdate {
		if err := gce.createTargetPool(name, region, hosts, affinityType); err != nil {
			return nil, fmt.Errorf("failed to create target pool %s: %v", name, err)
		}
	}
	if tpNeedsUpdate || fwdRuleNeedsUpdate {
		if err := gce.createForwardingRule(name, region, ipAddress, ports); err != nil {
			return nil, fmt.Errorf("failed to create forwarding rule %s: %v", name, err)
		}
	}

	// Now that we're done operating on everything, demote the static IP back to
	// ephemeral to avoid taking up the user's static IP quota.
	if err := gce.deleteStaticIP(name, region); err != nil {
		return nil, fmt.Errorf("failed to release static IP %s after finishing update of load balancer resources: %v", err)
	}

	status := &api.LoadBalancerStatus{}
	status.Ingress = []api.LoadBalancerIngress{{IP: ipAddress}}
	return status, nil
}

// Passing nil for requested IP is perfectly fine - it just means that no specific
// IP is being requested.
// Returns whether the forwarding rule exists, whether it needs to be updated,
// what its IP address is (if it exists), and any error we encountered.
func (gce *GCECloud) forwardingRuleNeedsUpdate(name, region string, requestedIP net.IP, ports []*api.ServicePort) (exists bool, needsUpdate bool, ipAddress string, err error) {
	fwd, err := gce.service.ForwardingRules.Get(gce.projectID, region, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return false, true, "", nil
		}
		return false, false, "", fmt.Errorf("error getting load balancer's forwarding rule: %v", err)
	}
	if requestedIP != nil && requestedIP.String() != fwd.IPAddress {
		return true, true, fwd.IPAddress, nil
	}
	portRange, err := loadBalancerPortRange(ports)
	if err != nil {
		return false, false, "", err
	}
	if portRange != fwd.PortRange {
		return true, true, fwd.IPAddress, nil
	}
	// The service controller verified all the protocols match on the ports, just check the first one
	if string(ports[0].Protocol) != fwd.IPProtocol {
		return true, true, fwd.IPAddress, nil
	}

	return true, false, fwd.IPAddress, nil
}

func loadBalancerPortRange(ports []*api.ServicePort) (string, error) {
	if len(ports) == 0 {
		return "", fmt.Errorf("no ports specified for GCE load balancer")
	}

	// The service controller verified all the protocols match on the ports, just check and use the first one
	if ports[0].Protocol != api.ProtocolTCP && ports[0].Protocol != api.ProtocolUDP {
		return "", fmt.Errorf("Invalid protocol %s, only TCP and UDP are supported", string(ports[0].Protocol))
	}

	minPort := 65536
	maxPort := 0
	for i := range ports {
		if ports[i].Port < minPort {
			minPort = ports[i].Port
		}
		if ports[i].Port > maxPort {
			maxPort = ports[i].Port
		}
	}
	return fmt.Sprintf("%d-%d", minPort, maxPort), nil
}

// Doesn't check whether the hosts have changed, since host updating is handled
// separately.
func (gce *GCECloud) targetPoolNeedsUpdate(name, region string, affinityType api.ServiceAffinity) (exists bool, needsUpdate bool, err error) {
	tp, err := gce.service.TargetPools.Get(gce.projectID, region, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return false, true, nil
		}
		return false, false, fmt.Errorf("error getting load balancer's target pool: %v", err)
	}
	if translateAffinityType(affinityType) != tp.SessionAffinity {
		return true, true, nil
	}
	return true, false, nil
}

// translate from what K8s supports to what the cloud provider supports for session affinity.
func translateAffinityType(affinityType api.ServiceAffinity) string {
	switch affinityType {
	case api.ServiceAffinityClientIP:
		return gceAffinityTypeClientIP
	case api.ServiceAffinityNone:
		return gceAffinityTypeNone
	default:
		glog.Errorf("Unexpected affinity type: %v", affinityType)
		return gceAffinityTypeNone
	}
}

func (gce *GCECloud) firewallNeedsUpdate(name, region, ipAddress string, ports []*api.ServicePort) (exists bool, needsUpdate bool, err error) {
	fw, err := gce.service.Firewalls.Get(gce.projectID, makeFirewallName(name)).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return false, true, nil
		}
		return false, false, fmt.Errorf("error getting load balancer's target pool: %v", err)
	}
	if fw.Description != makeFirewallDescription(ipAddress) {
		return true, true, nil
	}
	if len(fw.Allowed) != 1 || (fw.Allowed[0].IPProtocol != "tcp" && fw.Allowed[0].IPProtocol != "udp") {
		return true, true, nil
	}
	// Make sure the allowed ports match.
	allowedPorts := make([]string, len(ports))
	for ix := range ports {
		allowedPorts[ix] = strconv.Itoa(ports[ix].Port)
	}
	if !slicesEqual(allowedPorts, fw.Allowed[0].Ports) {
		return true, true, nil
	}
	// The service controller already verified that the protocol matches on all ports, no need to check.

	return true, false, nil
}

func makeFirewallName(name string) string {
	return fmt.Sprintf("k8s-fw-%s", name)
}

func makeFirewallDescription(ipAddress string) string {
	return fmt.Sprintf("KubernetesAutoGenerated_OnlyAllowTrafficForDestinationIP_%s", ipAddress)
}

func slicesEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func (gce *GCECloud) createForwardingRule(name, region, ipAddress string, ports []*api.ServicePort) error {
	portRange, err := loadBalancerPortRange(ports)
	if err != nil {
		return err
	}
	req := &compute.ForwardingRule{
		Name:      name,
		IPAddress: ipAddress, IPProtocol: string(ports[0].Protocol),
		PortRange: portRange,
		Target:    gce.targetPoolURL(name, region),
	}

	op, err := gce.service.ForwardingRules.Insert(gce.projectID, region, req).Do()
	if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
		return err
	}
	if op != nil {
		err = gce.waitForRegionOp(op, region)
		if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) createTargetPool(name, region string, hosts []string, affinityType api.ServiceAffinity) error {
	var instances []string
	for _, host := range hosts {
		instances = append(instances, makeHostURL(gce.projectID, gce.zone, host))
	}
	pool := &compute.TargetPool{
		Name:            name,
		Instances:       instances,
		SessionAffinity: translateAffinityType(affinityType),
	}
	op, err := gce.service.TargetPools.Insert(gce.projectID, region, pool).Do()
	if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
		return err
	}
	if op != nil {
		err = gce.waitForRegionOp(op, region)
		if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) createFirewall(name, region, ipAddress string, ports []*api.ServicePort, hosts []string) error {
	firewall, err := gce.firewallObject(name, region, ipAddress, ports, hosts)
	if err != nil {
		return err
	}
	op, err := gce.service.Firewalls.Insert(gce.projectID, firewall).Do()
	if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
		return err
	}
	if op != nil {
		err = gce.waitForGlobalOp(op)
		if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) updateFirewall(name, region, ipAddress string, ports []*api.ServicePort, hosts []string) error {
	firewall, err := gce.firewallObject(name, region, ipAddress, ports, hosts)
	if err != nil {
		return err
	}
	op, err := gce.service.Firewalls.Update(gce.projectID, makeFirewallName(name), firewall).Do()
	if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
		return err
	}
	if op != nil {
		err = gce.waitForGlobalOp(op)
		if err != nil {
			return err
		}
	}
	return nil
}

func (gce *GCECloud) firewallObject(name, region, ipAddress string, ports []*api.ServicePort, hosts []string) (*compute.Firewall, error) {
	allowedPorts := make([]string, len(ports))
	for ix := range ports {
		allowedPorts[ix] = strconv.Itoa(ports[ix].Port)
	}
	hostTags, err := gce.computeHostTags(hosts)
	if err != nil {
		return nil, err
	}
	firewall := &compute.Firewall{
		Name:         makeFirewallName(name),
		Description:  makeFirewallDescription(ipAddress),
		Network:      gce.networkURL,
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   hostTags,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: strings.ToLower(string(ports[0].Protocol)),
				Ports:      allowedPorts,
			},
		},
	}
	return firewall, nil
}

// We grab all tags from all instances being added to the pool.
// * The longest tag that is a prefix of the instance name is used
// * If any instance has a prefix tag, all instances must
// * If no instances have a prefix tag, no tags are used
func (gce *GCECloud) computeHostTags(hosts []string) ([]string, error) {
	listCall := gce.service.Instances.List(gce.projectID, gce.zone)

	// Add the filter for hosts
	listCall = listCall.Filter("name eq (" + strings.Join(hosts, "|") + ")")

	// Add the fields we want
	listCall = listCall.Fields("items(name,tags)")

	res, err := listCall.Do()
	if err != nil {
		return nil, err
	}

	tags := sets.NewString()
	for _, instance := range res.Items {
		longest_tag := ""
		for _, tag := range instance.Tags.Items {
			if strings.HasPrefix(instance.Name, tag) && len(tag) > len(longest_tag) {
				longest_tag = tag
			}
		}
		if len(longest_tag) > 0 {
			tags.Insert(longest_tag)
		} else if len(tags) > 0 {
			return nil, fmt.Errorf("Some, but not all, instances have prefix tags (%s is missing)", instance.Name)
		}
	}

	if len(tags) == 0 {
		glog.V(2).Info("No instances had tags, creating rule without target tags")
	}

	return tags.List(), nil
}

func (gce *GCECloud) projectOwnsStaticIP(name, region string, ipAddress string) error {
	addresses, err := gce.service.Addresses.List(gce.projectID, region).Do()
	if err != nil {
		return fmt.Errorf("failed to list gce IP addresses: %v", err)
	}
	for _, addr := range addresses.Items {
		if addr.Address == ipAddress {
			// This project does own the address, so return success.
			return nil
		}
	}
	return fmt.Errorf("this gce project doesn't own the IP address: %s", ipAddress)
}

func (gce *GCECloud) createOrPromoteStaticIP(name, region, existingIP string) (ipAddress string, err error) {
	// If the address doesn't exist, this will create it.
	// If the existingIP exists but is ephemeral, this will promote it to static.
	// If the address already exists, this will harmlessly return a StatusConflict
	// and we'll grab the IP before returning.
	addressObj := &compute.Address{Name: name}
	if existingIP != "" {
		addressObj.Address = existingIP
	}
	op, err := gce.service.Addresses.Insert(gce.projectID, region, addressObj).Do()
	if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
		return "", fmt.Errorf("error creating gce static IP address: %v", err)
	}
	if op != nil {
		err := gce.waitForRegionOp(op, region)
		if err != nil && !isHTTPErrorCode(err, http.StatusConflict) {
			return "", fmt.Errorf("error waiting for gce static IP address to be created: %v", err)
		}
	}

	// We have to get the address to know which IP was allocated for us.
	address, err := gce.service.Addresses.Get(gce.projectID, region, name).Do()
	if err != nil {
		return "", fmt.Errorf("error re-getting gce static IP address: %v", err)
	}
	return address.Address, nil
}

// UpdateLoadBalancer is an implementation of LoadBalancer.UpdateLoadBalancer.
func (gce *GCECloud) UpdateLoadBalancer(name, region string, hosts []string) error {
	pool, err := gce.service.TargetPools.Get(gce.projectID, region, name).Do()
	if err != nil {
		return err
	}
	existing := sets.NewString()
	for _, instance := range pool.Instances {
		existing.Insert(hostURLToComparablePath(instance))
	}

	var toAdd []*compute.InstanceReference
	var toRemove []*compute.InstanceReference
	for _, host := range hosts {
		link := makeComparableHostPath(gce.zone, host)
		if !existing.Has(link) {
			toAdd = append(toAdd, &compute.InstanceReference{Instance: link})
		}
		existing.Delete(link)
	}
	for link := range existing {
		toRemove = append(toRemove, &compute.InstanceReference{Instance: link})
	}

	if len(toAdd) > 0 {
		add := &compute.TargetPoolsAddInstanceRequest{Instances: toAdd}
		op, err := gce.service.TargetPools.AddInstance(gce.projectID, region, name, add).Do()
		if err != nil {
			return err
		}
		if err := gce.waitForRegionOp(op, region); err != nil {
			return err
		}
	}

	if len(toRemove) > 0 {
		rm := &compute.TargetPoolsRemoveInstanceRequest{Instances: toRemove}
		op, err := gce.service.TargetPools.RemoveInstance(gce.projectID, region, name, rm).Do()
		if err != nil {
			return err
		}
		if err := gce.waitForRegionOp(op, region); err != nil {
			return err
		}
	}

	// Try to verify that the correct number of nodes are now in the target pool.
	// We've been bitten by a bug here before (#11327) where all nodes were
	// accidentally removed and want to make similar problems easier to notice.
	updatedPool, err := gce.service.TargetPools.Get(gce.projectID, region, name).Do()
	if err != nil {
		return err
	}
	if len(updatedPool.Instances) != len(hosts) {
		glog.Errorf("Unexpected number of instances (%d) in target pool %s after updating (expected %d). Instances in updated pool: %s",
			len(updatedPool.Instances), name, len(hosts), strings.Join(updatedPool.Instances, ","))
		return fmt.Errorf("Unexpected number of instances (%d) in target pool %s after update (expected %d)", len(updatedPool.Instances), name, len(hosts))
	}
	return nil
}

// EnsureLoadBalancerDeleted is an implementation of LoadBalancer.EnsureLoadBalancerDeleted.
func (gce *GCECloud) EnsureLoadBalancerDeleted(name, region string) error {
	err := utilerrors.AggregateGoroutines(
		func() error { return gce.deleteFirewall(name, region) },
		// Even though we don't hold on to static IPs for load balancers, it's
		// possible that EnsureLoadBalancer left one around in a failed
		// creation/update attempt, so make sure we clean it up here just in case.
		func() error { return gce.deleteStaticIP(name, region) },
		func() error {
			// The forwarding rule must be deleted before either the target pool can,
			// unfortunately, so we have to do these two serially.
			if err := gce.deleteForwardingRule(name, region); err != nil {
				return err
			}
			if err := gce.deleteTargetPool(name, region); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		return utilerrors.Flatten(err)
	}
	return nil
}

func (gce *GCECloud) deleteForwardingRule(name, region string) error {
	op, err := gce.service.ForwardingRules.Delete(gce.projectID, region, name).Do()
	if err != nil && isHTTPErrorCode(err, http.StatusNotFound) {
		glog.Infof("Forwarding rule %s already deleted. Continuing to delete other resources.", name)
	} else if err != nil {
		glog.Warningf("Failed to delete forwarding rule %s: got error %s.", name, err.Error())
		return err
	} else {
		if err := gce.waitForRegionOp(op, region); err != nil {
			glog.Warningf("Failed waiting for forwarding rule %s to be deleted: got error %s.", name, err.Error())
			return err
		}
	}
	return nil
}

func (gce *GCECloud) deleteTargetPool(name, region string) error {
	op, err := gce.service.TargetPools.Delete(gce.projectID, region, name).Do()
	if err != nil && isHTTPErrorCode(err, http.StatusNotFound) {
		glog.Infof("Target pool %s already deleted. Continuing to delete other resources.", name)
	} else if err != nil {
		glog.Warningf("Failed to delete target pool %s, got error %s.", name, err.Error())
		return err
	} else {
		if err := gce.waitForRegionOp(op, region); err != nil {
			glog.Warningf("Failed waiting for target pool %s to be deleted: got error %s.", name, err.Error())
			return err
		}
	}
	return nil
}

func (gce *GCECloud) deleteFirewall(name, region string) error {
	fwName := makeFirewallName(name)
	op, err := gce.service.Firewalls.Delete(gce.projectID, fwName).Do()
	if err != nil && isHTTPErrorCode(err, http.StatusNotFound) {
		glog.Infof("Firewall %s already deleted. Continuing to delete other resources.", name)
	} else if err != nil {
		glog.Warningf("Failed to delete firewall %s, got error %v", fwName, err)
		return err
	} else {
		if err := gce.waitForGlobalOp(op); err != nil {
			glog.Warningf("Failed waiting for Firewall %s to be deleted.  Got error: %v", fwName, err)
			return err
		}
	}
	return nil
}

func (gce *GCECloud) deleteStaticIP(name, region string) error {
	op, err := gce.service.Addresses.Delete(gce.projectID, region, name).Do()
	if err != nil && isHTTPErrorCode(err, http.StatusNotFound) {
		glog.Infof("Static IP address %s already deleted. Continuing to delete other resources.", name)
	} else if err != nil {
		glog.Warningf("Failed to delete static IP address %s, got error %v", name, err)
		return err
	} else {
		if err := gce.waitForRegionOp(op, region); err != nil {
			glog.Warningf("Failed waiting for address %s to be deleted, got error: %v", name, err)
			return err
		}
	}
	return nil
}

// UrlMap management

// GetUrlMap returns the UrlMap by name.
func (gce *GCECloud) GetUrlMap(name string) (*compute.UrlMap, error) {
	return gce.service.UrlMaps.Get(gce.projectID, name).Do()
}

// CreateUrlMap creates an url map, using the given backend service as the default service.
func (gce *GCECloud) CreateUrlMap(backend *compute.BackendService, name string) (*compute.UrlMap, error) {
	urlMap := &compute.UrlMap{
		Name:           name,
		DefaultService: backend.SelfLink,
	}
	op, err := gce.service.UrlMaps.Insert(gce.projectID, urlMap).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForGlobalOp(op); err != nil {
		return nil, err
	}
	return gce.GetUrlMap(name)
}

// UpdateUrlMap applies the given UrlMap as an update, and returns the new UrlMap.
func (gce *GCECloud) UpdateUrlMap(urlMap *compute.UrlMap) (*compute.UrlMap, error) {
	op, err := gce.service.UrlMaps.Update(gce.projectID, urlMap.Name, urlMap).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForGlobalOp(op); err != nil {
		return nil, err
	}
	return gce.service.UrlMaps.Get(gce.projectID, urlMap.Name).Do()
}

// DeleteUrlMap deletes a url map by name.
func (gce *GCECloud) DeleteUrlMap(name string) error {
	op, err := gce.service.UrlMaps.Delete(gce.projectID, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForGlobalOp(op)
}

// ListUrlMaps lists all UrlMaps in the project.
func (gce *GCECloud) ListUrlMaps() (*compute.UrlMapList, error) {
	return gce.service.UrlMaps.List(gce.projectID).Do()
}

// TargetHttpProxy management

// GetTargetHttpProxy returns the UrlMap by name.
func (gce *GCECloud) GetTargetHttpProxy(name string) (*compute.TargetHttpProxy, error) {
	return gce.service.TargetHttpProxies.Get(gce.projectID, name).Do()
}

// CreateTargetHttpProxy creates and returns a TargetHttpProxy with the given UrlMap.
func (gce *GCECloud) CreateTargetHttpProxy(urlMap *compute.UrlMap, name string) (*compute.TargetHttpProxy, error) {
	proxy := &compute.TargetHttpProxy{
		Name:   name,
		UrlMap: urlMap.SelfLink,
	}
	op, err := gce.service.TargetHttpProxies.Insert(gce.projectID, proxy).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForGlobalOp(op); err != nil {
		return nil, err
	}
	return gce.GetTargetHttpProxy(name)
}

// SetUrlMapForTargetHttpProxy sets the given UrlMap for the given TargetHttpProxy.
func (gce *GCECloud) SetUrlMapForTargetHttpProxy(proxy *compute.TargetHttpProxy, urlMap *compute.UrlMap) error {
	op, err := gce.service.TargetHttpProxies.SetUrlMap(gce.projectID, proxy.Name, &compute.UrlMapReference{urlMap.SelfLink}).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// DeleteTargetHttpProxy deletes the TargetHttpProxy by name.
func (gce *GCECloud) DeleteTargetHttpProxy(name string) error {
	op, err := gce.service.TargetHttpProxies.Delete(gce.projectID, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForGlobalOp(op)
}

// ListTargetHttpProxies lists all TargetHttpProxies in the project.
func (gce *GCECloud) ListTargetHttpProxies() (*compute.TargetHttpProxyList, error) {
	return gce.service.TargetHttpProxies.List(gce.projectID).Do()
}

// GlobalForwardingRule management

// CreateGlobalForwardingRule creates and returns a GlobalForwardingRule that points to the given TargetHttpProxy.
func (gce *GCECloud) CreateGlobalForwardingRule(proxy *compute.TargetHttpProxy, name string, portRange string) (*compute.ForwardingRule, error) {
	rule := &compute.ForwardingRule{
		Name:       name,
		Target:     proxy.SelfLink,
		PortRange:  portRange,
		IPProtocol: "TCP",
	}
	op, err := gce.service.GlobalForwardingRules.Insert(gce.projectID, rule).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForGlobalOp(op); err != nil {
		return nil, err
	}
	return gce.GetGlobalForwardingRule(name)
}

// SetProxyForGlobalForwardingRule links the given TargetHttpProxy with the given GlobalForwardingRule.
func (gce *GCECloud) SetProxyForGlobalForwardingRule(fw *compute.ForwardingRule, proxy *compute.TargetHttpProxy) error {
	op, err := gce.service.GlobalForwardingRules.SetTarget(gce.projectID, fw.Name, &compute.TargetReference{proxy.SelfLink}).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// DeleteGlobalForwardingRule deletes the GlobalForwardingRule by name.
func (gce *GCECloud) DeleteGlobalForwardingRule(name string) error {
	op, err := gce.service.GlobalForwardingRules.Delete(gce.projectID, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForGlobalOp(op)
}

// GetGlobalForwardingRule returns the GlobalForwardingRule by name.
func (gce *GCECloud) GetGlobalForwardingRule(name string) (*compute.ForwardingRule, error) {
	return gce.service.GlobalForwardingRules.Get(gce.projectID, name).Do()
}

// ListGlobalForwardingRules lists all GlobalForwardingRules in the project.
func (gce *GCECloud) ListGlobalForwardingRules() (*compute.ForwardingRuleList, error) {
	return gce.service.GlobalForwardingRules.List(gce.projectID).Do()
}

// BackendService Management

// GetBackendService retrieves a backend by name.
func (gce *GCECloud) GetBackendService(name string) (*compute.BackendService, error) {
	return gce.service.BackendServices.Get(gce.projectID, name).Do()
}

// UpdateBackendService applies the given BackendService as an update to an existing service.
func (gce *GCECloud) UpdateBackendService(bg *compute.BackendService) error {
	op, err := gce.service.BackendServices.Update(gce.projectID, bg.Name, bg).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// DeleteBackendService deletes the given BackendService by name.
func (gce *GCECloud) DeleteBackendService(name string) error {
	op, err := gce.service.BackendServices.Delete(gce.projectID, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForGlobalOp(op)
}

// CreateBackendService creates the given BackendService.
func (gce *GCECloud) CreateBackendService(bg *compute.BackendService) error {
	op, err := gce.service.BackendServices.Insert(gce.projectID, bg).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// ListBackendServices lists all backend services in the project.
func (gce *GCECloud) ListBackendServices() (*compute.BackendServiceList, error) {
	return gce.service.BackendServices.List(gce.projectID).Do()
}

// GetHealth returns the health of the BackendService identified by the given
// name, in the given instanceGroup. The instanceGroupLink is the fully
// qualified self link of an instance group.
func (gce *GCECloud) GetHealth(name string, instanceGroupLink string) (*compute.BackendServiceGroupHealth, error) {
	groupRef := &compute.ResourceGroupReference{instanceGroupLink}
	return gce.service.BackendServices.GetHealth(gce.projectID, name, groupRef).Do()
}

// Health Checks

// GetHttpHealthCheck returns the given HttpHealthCheck by name.
func (gce *GCECloud) GetHttpHealthCheck(name string) (*compute.HttpHealthCheck, error) {
	return gce.service.HttpHealthChecks.Get(gce.projectID, name).Do()
}

// UpdateHttpHealthCheck applies the given HttpHealthCheck as an update.
func (gce *GCECloud) UpdateHttpHealthCheck(hc *compute.HttpHealthCheck) error {
	op, err := gce.service.HttpHealthChecks.Update(gce.projectID, hc.Name, hc).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// DeleteHttpHealthCheck deletes the given HttpHealthCheck by name.
func (gce *GCECloud) DeleteHttpHealthCheck(name string) error {
	op, err := gce.service.HttpHealthChecks.Delete(gce.projectID, name).Do()
	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForGlobalOp(op)
}

// CreateHttpHealthCheck creates the given HttpHealthCheck.
func (gce *GCECloud) CreateHttpHealthCheck(hc *compute.HttpHealthCheck) error {
	op, err := gce.service.HttpHealthChecks.Insert(gce.projectID, hc).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(op)
}

// ListHttpHealthCheck lists all HttpHealthChecks in the project.
func (gce *GCECloud) ListHttpHealthChecks() (*compute.HttpHealthCheckList, error) {
	return gce.service.HttpHealthChecks.List(gce.projectID).Do()
}

// InstanceGroup Management

// CreateInstanceGroup creates an instance group with the given instances. It is the callers responsibility to add named ports.
func (gce *GCECloud) CreateInstanceGroup(name string) (*compute.InstanceGroup, error) {
	op, err := gce.service.InstanceGroups.Insert(
		gce.projectID, gce.zone, &compute.InstanceGroup{Name: name}).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForZoneOp(op); err != nil {
		return nil, err
	}
	return gce.GetInstanceGroup(name)
}

// DeleteInstanceGroup deletes an instance group.
func (gce *GCECloud) DeleteInstanceGroup(name string) error {
	op, err := gce.service.InstanceGroups.Delete(
		gce.projectID, gce.zone, name).Do()
	if err != nil {
		return err
	}
	return gce.waitForZoneOp(op)
}

// ListInstanceGroups lists all InstanceGroups in the project and zone.
func (gce *GCECloud) ListInstanceGroups() (*compute.InstanceGroupList, error) {
	return gce.service.InstanceGroups.List(gce.projectID, gce.zone).Do()
}

// ListInstancesInInstanceGroup lists all the instances in a given istance group and state.
func (gce *GCECloud) ListInstancesInInstanceGroup(name string, state string) (*compute.InstanceGroupsListInstances, error) {
	return gce.service.InstanceGroups.ListInstances(
		gce.projectID, gce.zone, name,
		&compute.InstanceGroupsListInstancesRequest{InstanceState: state}).Do()
}

// AddInstancesToInstanceGroup adds the given instances to the given instance group.
func (gce *GCECloud) AddInstancesToInstanceGroup(name string, instanceNames []string) error {
	if len(instanceNames) == 0 {
		return nil
	}
	// Adding the same instance twice will result in a 4xx error
	instances := []*compute.InstanceReference{}
	for _, ins := range instanceNames {
		instances = append(instances, &compute.InstanceReference{makeHostURL(gce.projectID, gce.zone, ins)})
	}
	op, err := gce.service.InstanceGroups.AddInstances(
		gce.projectID, gce.zone, name,
		&compute.InstanceGroupsAddInstancesRequest{
			Instances: instances,
		}).Do()

	if err != nil {
		return err
	}
	return gce.waitForZoneOp(op)
}

// RemoveInstancesFromInstanceGroup removes the given instances from the instance group.
func (gce *GCECloud) RemoveInstancesFromInstanceGroup(name string, instanceNames []string) error {
	if len(instanceNames) == 0 {
		return nil
	}
	instances := []*compute.InstanceReference{}
	for _, ins := range instanceNames {
		instanceLink := makeHostURL(gce.projectID, gce.zone, ins)
		instances = append(instances, &compute.InstanceReference{instanceLink})
	}
	op, err := gce.service.InstanceGroups.RemoveInstances(
		gce.projectID, gce.zone, name,
		&compute.InstanceGroupsRemoveInstancesRequest{
			Instances: instances,
		}).Do()

	if err != nil {
		if isHTTPErrorCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return gce.waitForZoneOp(op)
}

// AddPortToInstanceGroup adds a port to the given instance group.
func (gce *GCECloud) AddPortToInstanceGroup(ig *compute.InstanceGroup, port int64) (*compute.NamedPort, error) {
	for _, np := range ig.NamedPorts {
		if np.Port == port {
			glog.Infof("Instance group %v already has named port %+v", ig.Name, np)
			return np, nil
		}
	}
	glog.Infof("Adding port %v to instance group %v with %d ports", port, ig.Name, len(ig.NamedPorts))
	namedPort := compute.NamedPort{fmt.Sprintf("port%v", port), port}
	ig.NamedPorts = append(ig.NamedPorts, &namedPort)
	op, err := gce.service.InstanceGroups.SetNamedPorts(
		gce.projectID, gce.zone, ig.Name,
		&compute.InstanceGroupsSetNamedPortsRequest{
			NamedPorts: ig.NamedPorts}).Do()
	if err != nil {
		return nil, err
	}
	if err = gce.waitForZoneOp(op); err != nil {
		return nil, err
	}
	return &namedPort, nil
}

// GetInstanceGroup returns an instance group by name.
func (gce *GCECloud) GetInstanceGroup(name string) (*compute.InstanceGroup, error) {
	return gce.service.InstanceGroups.Get(gce.projectID, gce.zone, name).Do()
}

// Take a GCE instance 'hostname' and break it down to something that can be fed
// to the GCE API client library.  Basically this means reducing 'kubernetes-
// minion-2.c.my-proj.internal' to 'kubernetes-minion-2' if necessary.
func canonicalizeInstanceName(name string) string {
	ix := strings.Index(name, ".")
	if ix != -1 {
		name = name[:ix]
	}
	return name
}

// Return the instances matching the relevant name.
func (gce *GCECloud) getInstanceByName(name string) (*compute.Instance, error) {
	name = canonicalizeInstanceName(name)
	res, err := gce.service.Instances.Get(gce.projectID, gce.zone, name).Do()
	if err != nil {
		glog.Errorf("Failed to retrieve TargetInstance resource for instance: %s", name)
		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == http.StatusNotFound {
			return nil, cloudprovider.InstanceNotFound
		}
		return nil, err
	}
	return res, nil
}

// Implementation of Instances.CurrentNodeName
func (gce *GCECloud) CurrentNodeName(hostname string) (string, error) {
	return hostname, nil
}

func (gce *GCECloud) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
		project, err := gce.service.Projects.Get(gce.projectID).Do()
		if err != nil {
			glog.Errorf("Could not get project: %v", err)
			return false, nil
		}
		keyString := fmt.Sprintf("%s:%s %s@%s", user, strings.TrimSpace(string(keyData)), user, user)
		found := false
		for _, item := range project.CommonInstanceMetadata.Items {
			if item.Key == "sshKeys" {
				if strings.Contains(item.Value, keyString) {
					// We've already added the key
					glog.Info("SSHKey already in project metadata")
					return true, nil
				}
				item.Value = item.Value + "\n" + keyString
				found = true
				break
			}
		}
		if !found {
			// This is super unlikely, so log.
			glog.Infof("Failed to find sshKeys metadata, creating a new item")
			project.CommonInstanceMetadata.Items = append(project.CommonInstanceMetadata.Items,
				&compute.MetadataItems{
					Key:   "sshKeys",
					Value: keyString,
				})
		}
		op, err := gce.service.Projects.SetCommonInstanceMetadata(gce.projectID, project.CommonInstanceMetadata).Do()
		if err != nil {
			glog.Errorf("Could not Set Metadata: %v", err)
			return false, nil
		}
		if err := gce.waitForGlobalOp(op); err != nil {
			glog.Errorf("Could not Set Metadata: %v", err)
			return false, nil
		}
		glog.Infof("Successfully added sshKey to project metadata")
		return true, nil
	})
}

// NodeAddresses is an implementation of Instances.NodeAddresses.
func (gce *GCECloud) NodeAddresses(_ string) ([]api.NodeAddress, error) {
	internalIP, err := metadata.Get("instance/network-interfaces/0/ip")
	if err != nil {
		return nil, fmt.Errorf("couldn't get internal IP: %v", err)
	}
	externalIP, err := metadata.Get("instance/network-interfaces/0/access-configs/0/external-ip")
	if err != nil {
		return nil, fmt.Errorf("couldn't get external IP: %v", err)
	}
	return []api.NodeAddress{
		{Type: api.NodeInternalIP, Address: internalIP},
		{Type: api.NodeExternalIP, Address: externalIP},
	}, nil
}

// isCurrentInstance uses metadata server to check if specified instanceID matches current machine's instanceID
func (gce *GCECloud) isCurrentInstance(instanceID string) bool {
	currentInstanceID, err := getInstanceIDViaMetadata()
	if err != nil {
		// Log and swallow error
		glog.Errorf("Failed to fetch instanceID via Metadata: %v", err)
		return false
	}

	return currentInstanceID == canonicalizeInstanceName(instanceID)
}

// ExternalID returns the cloud provider ID of the specified instance (deprecated).
func (gce *GCECloud) ExternalID(instance string) (string, error) {
	if gce.useMetadataServer {
		// Use metadata, if possible, to fetch ID. See issue #12000
		if gce.isCurrentInstance(instance) {
			externalInstanceID, err := getCurrentExternalIDViaMetadata()
			if err == nil {
				return externalInstanceID, nil
			}
		}
	}

	// Fallback to GCE API call if metadata server fails to retrieve ID
	inst, err := gce.getInstanceByName(instance)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(inst.Id, 10), nil
}

// InstanceID returns the cloud provider ID of the specified instance.
func (gce *GCECloud) InstanceID(instance string) (string, error) {
	return gce.projectID + "/" + gce.zone + "/" + canonicalizeInstanceName(instance), nil
}

// List is an implementation of Instances.List.
func (gce *GCECloud) List(filter string) ([]string, error) {
	listCall := gce.service.Instances.List(gce.projectID, gce.zone)
	if len(filter) > 0 {
		listCall = listCall.Filter("name eq " + filter)
	}
	res, err := listCall.Do()
	if err != nil {
		return nil, err
	}
	var instances []string
	for _, instance := range res.Items {
		instances = append(instances, instance.Name)
	}
	return instances, nil
}

func getMetadataValue(metadata *compute.Metadata, key string) (string, bool) {
	for _, item := range metadata.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return "", false
}

func truncateClusterName(clusterName string) string {
	if len(clusterName) > 26 {
		return clusterName[:26]
	}
	return clusterName
}

func (gce *GCECloud) ListRoutes(clusterName string) ([]*cloudprovider.Route, error) {
	listCall := gce.service.Routes.List(gce.projectID)

	prefix := truncateClusterName(clusterName)
	listCall = listCall.Filter("name eq " + prefix + "-.*")

	res, err := listCall.Do()
	if err != nil {
		return nil, err
	}
	var routes []*cloudprovider.Route
	for _, r := range res.Items {
		if r.Network != gce.networkURL {
			continue
		}
		// Not managed if route description != "k8s-node-route"
		if r.Description != k8sNodeRouteTag {
			continue
		}
		// Not managed if route name doesn't start with <clusterName>
		if !strings.HasPrefix(r.Name, prefix) {
			continue
		}

		target := path.Base(r.NextHopInstance)
		routes = append(routes, &cloudprovider.Route{Name: r.Name, TargetInstance: target, DestinationCIDR: r.DestRange})
	}
	return routes, nil
}

func gceNetworkURL(project, network string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", project, network)
}

func (gce *GCECloud) CreateRoute(clusterName string, nameHint string, route *cloudprovider.Route) error {
	routeName := truncateClusterName(clusterName) + "-" + nameHint

	instanceName := canonicalizeInstanceName(route.TargetInstance)
	insertOp, err := gce.service.Routes.Insert(gce.projectID, &compute.Route{
		Name:            routeName,
		DestRange:       route.DestinationCIDR,
		NextHopInstance: fmt.Sprintf("zones/%s/instances/%s", gce.zone, instanceName),
		Network:         gce.networkURL,
		Priority:        1000,
		Description:     k8sNodeRouteTag,
	}).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(insertOp)
}

func (gce *GCECloud) DeleteRoute(clusterName string, route *cloudprovider.Route) error {
	deleteOp, err := gce.service.Routes.Delete(gce.projectID, route.Name).Do()
	if err != nil {
		return err
	}
	return gce.waitForGlobalOp(deleteOp)
}

func (gce *GCECloud) GetZone() (cloudprovider.Zone, error) {
	region, err := getGceRegion(gce.zone)
	if err != nil {
		return cloudprovider.Zone{}, err
	}
	return cloudprovider.Zone{
		FailureDomain: gce.zone,
		Region:        region,
	}, nil
}

func (gce *GCECloud) CreateDisk(name string, sizeGb int64) error {
	diskToCreate := &compute.Disk{
		Name:   name,
		SizeGb: sizeGb,
	}
	createOp, err := gce.service.Disks.Insert(gce.projectID, gce.zone, diskToCreate).Do()
	if err != nil {
		return err
	}

	return gce.waitForZoneOp(createOp)
}

func (gce *GCECloud) DeleteDisk(diskToDelete string) error {
	deleteOp, err := gce.service.Disks.Delete(gce.projectID, gce.zone, diskToDelete).Do()
	if err != nil {
		return err
	}

	return gce.waitForZoneOp(deleteOp)
}

func (gce *GCECloud) AttachDisk(diskName, instanceID string, readOnly bool) error {
	disk, err := gce.getDisk(diskName)
	if err != nil {
		return err
	}
	readWrite := "READ_WRITE"
	if readOnly {
		readWrite = "READ_ONLY"
	}
	attachedDisk := gce.convertDiskToAttachedDisk(disk, readWrite)

	attachOp, err := gce.service.Instances.AttachDisk(gce.projectID, gce.zone, instanceID, attachedDisk).Do()
	if err != nil {
		return err
	}

	return gce.waitForZoneOp(attachOp)
}

func (gce *GCECloud) DetachDisk(devicePath, instanceID string) error {
	detachOp, err := gce.service.Instances.DetachDisk(gce.projectID, gce.zone, instanceID, devicePath).Do()
	if err != nil {
		return err
	}

	return gce.waitForZoneOp(detachOp)
}

func (gce *GCECloud) DiskIsAttached(diskName, instanceID string) (bool, error) {
	instance, err := gce.service.Instances.Get(gce.projectID, gce.zone, instanceID).Do()
	if err != nil {
		return false, err
	}

	for _, disk := range instance.Disks {
		if disk.DeviceName == diskName {
			// Disk is still attached to node
			return true, nil
		}
	}

	return false, nil
}

func (gce *GCECloud) getDisk(diskName string) (*compute.Disk, error) {
	return gce.service.Disks.Get(gce.projectID, gce.zone, diskName).Do()
}

// getGceRegion returns region of the gce zone. Zone names
// are of the form: ${region-name}-${ix}.
// For example "us-central1-b" has a region of "us-central1".
// So we look for the last '-' and trim to just before that.
func getGceRegion(zone string) (string, error) {
	ix := strings.LastIndex(zone, "-")
	if ix == -1 {
		return "", fmt.Errorf("unexpected zone: %s", zone)
	}
	return zone[:ix], nil
}

// Converts a Disk resource to an AttachedDisk resource.
func (gce *GCECloud) convertDiskToAttachedDisk(disk *compute.Disk, readWrite string) *compute.AttachedDisk {
	return &compute.AttachedDisk{
		DeviceName: disk.Name,
		Kind:       disk.Kind,
		Mode:       readWrite,
		Source:     "https://" + path.Join("www.googleapis.com/compute/v1/projects/", gce.projectID, "zones", gce.zone, "disks", disk.Name),
		Type:       "PERSISTENT",
	}
}

func (gce *GCECloud) ListClusters() ([]string, error) {
	list, err := gce.containerService.Projects.Zones.Clusters.List(gce.projectID, gce.zone).Do()
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, cluster := range list.Clusters {
		result = append(result, cluster.Name)
	}
	return result, nil
}

func (gce *GCECloud) Master(clusterName string) (string, error) {
	return "k8s-" + clusterName + "-master.internal", nil
}
