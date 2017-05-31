package route53

import (
	"k8s.io/contrib/dns/pkg/providers"
	"k8s.io/kubernetes/pkg/api"
	"github.com/aws/aws-sdk-go/aws/session"
	awsr53 "github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/aws"
	"fmt"
	"strings"
	"github.com/golang/glog"
	"github.com/aws/aws-sdk-go/service/elb"
)

type Route53Provider struct {
	r53Client *awsr53.Route53
	elbClient *elb.ELB
}

var _ providers.DNSProvider = &Route53Provider{}

func NewRoute53Provider() (*Route53Provider, error) {
	awsConfig := &aws.Config{}
	awsSession := session.New()
	r53Client := awsr53.New(awsSession, awsConfig)
	elbClient := elb.New(awsSession, awsConfig)

	p := &Route53Provider{
		r53Client: r53Client,
		elbClient : elbClient,
	}
	return p, nil
}

func (p*Route53Provider) getELBInfo(dnsName string) (*elb.LoadBalancerDescription, error) {
	// TODO: Cache
	// TODO: Check if "looks like" an ELB?
	// TODO: Only do this if service type=LoadBalancer

	tokens := strings.Split(dnsName, ".")
	elbNameTokens := strings.Split(tokens[0], "-")

	if len(elbNameTokens) != 2 {
		glog.Infof("does not look like an ELB name: %q", dnsName)
		return nil, nil
	}

	elbName := elbNameTokens[0]

	glog.V(2).Infof("Querying ELB for load balancer information for %q", elbName)
	request := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			&elbName,
		},
	}
	response, err := p.elbClient.DescribeLoadBalancers(request)
	if err != nil {
		return nil, fmt.Errorf("error describing load balancer %q: %v", elbName, err)
	}

	if len(response.LoadBalancerDescriptions) == 0 {
		glog.V(4).Infof("ELB not found %q; assuming not an ELB", elbName)
		return nil, nil
	}
	if len(response.LoadBalancerDescriptions) > 1 {
		return nil, fmt.Errorf("found multiple ELBs with name %q", elbName)
	}
	return response.LoadBalancerDescriptions[0], nil
}

func (p *Route53Provider) EnsureNames(names map[string]*providers.DNSName, ingress []api.LoadBalancerIngress) error {
	glog.V(8).Infof("EnsureNames: names %v, ingress %v", names, ingress)

	hostedZones, err := p.listHostedZones()
	if err != nil {
		return err
	}

	namesByZone := make(map[string][]*providers.DNSName)
	for _, v := range names {
		name := v.Name
		firstDot := strings.IndexRune(name, '.')
		if firstDot == -1 {
			glog.Warningf("Ignoring name with no dot: %q", name)
			continue
		}

		zone := name[firstDot + 1:]
		if zone == "" {
			glog.Warningf("ignoring name with unexpected structure: %q", name)
		}

		// TODO: Move to "normalizeZone" function ?
		if !strings.HasSuffix(zone, ".") {
			zone = zone + "."
		}

		// Not sure if needed?
		zone = strings.ToLower(zone)

		names := namesByZone[zone]
		names = append(names, v)
		namesByZone[zone] = names
	}

	var errors []error
	for _, hostedZone := range hostedZones {
		zone := aws.StringValue(hostedZone.Name)
		if zone == "" {
			glog.Warningf("ignoring HostedZone with empty name: %v", hostedZone)
			continue
		}

		if !strings.HasSuffix(zone, ".") {
			zone = zone + "."
		}

		// Not sure if needed?
		zone = strings.ToLower(zone)

		names := namesByZone[zone]
		if len(names) == 0 {
			glog.V(8).Infof("no names for zone %q", zone)
			continue
		}

		// The route53 API is awkward...
		hostedZoneID := aws.StringValue(hostedZone.Id)
		hostedZoneID = strings.TrimPrefix(hostedZoneID, "/hostedzone/")

		// Clear it so that we can warn about unmanaged zones later
		namesByZone[zone] = nil

		var changes []*awsr53.Change

		for _, n := range names {
			// TODO: Query zone and be much more efficient

			// TODO: Only call this once per loop
			rrs, err := p.buildRRS(n.Name, ingress)
			if err != nil {
				// TODO: Invalidate cache if "zone not found" ?
				glog.Warningf("error building records for name %q: %v", n.Name, err)
				errors = append(errors, err)
			}

			if rrs != nil {
				change := &awsr53.Change{
					Action:            aws.String("UPSERT"),
					ResourceRecordSet: rrs,
				}
				changes = append(changes, change)
			}

			// TODO: Remove old records (and thus avoid making the call here if records already match...)
		}

		if len(changes) == 0 {
			glog.V(4).Infof("No changes for zone %q; skipping", zone)
			continue
		}

		batch := &awsr53.ChangeBatch{
			Changes: changes,
			// TODO: Comment with our pod id?
			//Comment *string `type:"string"`
		}

		changeRequest := &awsr53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(hostedZoneID),
			ChangeBatch: batch,
		}

		// TODO: cope with limits
		// A request cannot contain more than 100 Change elements.
		// A request cannot contain more than 1000 ResourceRecord elements.
		// The sum of the number of characters (including spaces) in all Value elements in a request cannot exceed 32,000 characters.

		glog.Infof("making Route53 request to change zone %q", zone)
		glog.V(4).Infof("change request is %v", changeRequest)
		response, err := p.r53Client.ChangeResourceRecordSets(changeRequest)
		if err != nil {
			// TODO: Invalidate cache if "zone not found" ?
			glog.Warningf("error applying change to hosted zone %q: %v", zone, err)
			errors = append(errors, err)
		}
		glog.V(4).Infof("change response is %v", response)
	}

	for zone, names := range namesByZone {
		if len(names) == 0 {
			continue
		}

		// TODO: Periodically resync list of zones (or maybe just when we see a request for an unmanaged zone?)
		glog.Warningf("Ignoring unmanaged zone: %q", zone)
	}

	if len(errors) != 0 {
		return fmt.Errorf("error applying changes to zones: %v", errors)
	}

	return nil
}

func (p*Route53Provider) listHostedZones() ([]*awsr53.HostedZone, error) {
	// TODO: Cache (with periodic resync for new zones)
	var hostedZones []*awsr53.HostedZone
	request := &awsr53.ListHostedZonesInput{}
	glog.V(2).Infof("querying Route53 for hosted zones")
	err := p.r53Client.ListHostedZonesPages(request, func(p *awsr53.ListHostedZonesOutput, lastPage bool) (/*shouldContinue*/
	bool) {
		hostedZones = append(hostedZones, p.HostedZones...)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error listing hosted zones: %v", err)
	}

	glog.V(8).Infof("Hosted zones: %v", hostedZones)

	return hostedZones, nil
}

func (p*Route53Provider) buildRRS(name string, ingress []api.LoadBalancerIngress) (*awsr53.ResourceRecordSet, error) {
	// TODO: Most of this construction logic does not depend on the name
	var elbs []*elb.LoadBalancerDescription
	var hostnames []string
	var ips []string

	for _, target := range ingress {
		if target.Hostname != "" {
			elbInfo, err := p.getELBInfo(target.Hostname)
			if err != nil {
				return nil, fmt.Errorf("error querying ELB %q: %v", target.Hostname, err)
			}
			if elbInfo == nil {
				hostnames = append(hostnames, target.Hostname)
			} else {
				elbs = append(elbs, elbInfo)
			}
		} else if target.IP != "" {

		} else {
			glog.Warningf("Ignoring LoadBalancerIngress with neither Hostname nor IP")
		}
	}

	var rrs *awsr53.ResourceRecordSet

	if len(elbs) == 0 && len(ips) == 0 && len(hostnames) == 0 {
		glog.Warningf("no ingress elbs nor ips; ignoring")
		return nil, nil
	} else if len(elbs) == 1 && len(ips) == 0 && len(hostnames) == 0 {
		// TODO: Cope with cross-account ELBs?
		aliasTarget := &awsr53.AliasTarget{
			DNSName:              elbs[0].DNSName,
			EvaluateTargetHealth: aws.Bool(false),
			HostedZoneId:         elbs[0].CanonicalHostedZoneNameID,
		}

		rrs = &awsr53.ResourceRecordSet{
			AliasTarget: aliasTarget,
			Name:        aws.String(name),
			Type:        aws.String("A"),
		}
	} else if len(elbs) == 0 && len(hostnames) == 0 {
		var records []*awsr53.ResourceRecord
		for _, ip := range ips {
			// I think we can join these into a single string, but this feels clearer
			record := &awsr53.ResourceRecord{
				Value: aws.String(ip),
			}
			records = append(records, record)
		}

		rrs = &awsr53.ResourceRecordSet{
			Name:        aws.String(name),
			Type:        aws.String("A"),
			ResourceRecords: records,
		}
	} else if len(ips) == 0 {
		// CNAMEs
		var records []*awsr53.ResourceRecord
		for _, hostname := range hostnames {
			// I think we can join these into a single string, but this feels clearer
			record := &awsr53.ResourceRecord{
				Value: aws.String(hostname),
			}
			records = append(records, record)
		}

		if len(elbs) != 0 {
			// TODO: Is this right?  Should we warn here (less efficient, we're losing health-checks etc)
			glog.Warningf("Using CNAMEs for multiple ELBs")
			var records []*awsr53.ResourceRecord
			for _, elb := range elbs {
				record := &awsr53.ResourceRecord{
					Value: elb.DNSName,
				}
				records = append(records, record)
			}
		}

		rrs = &awsr53.ResourceRecordSet{
			Name:        aws.String(name),
			Type:        aws.String("CNAME"),
			ResourceRecords: records,
		}
	} else {
		return nil, fmt.Errorf("cannot mix IP and hostnames in DNS")
	}

	return rrs, nil
}