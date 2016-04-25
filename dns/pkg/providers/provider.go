package providers

import (
	"k8s.io/kubernetes/pkg/api"
	"fmt"
)

type DNSProvider interface {
	EnsureNames(names map[string]*DNSName, ingress []api.LoadBalancerIngress) error
}

type DNSName struct {
	Name string
}

func (n *DNSName) String() string {
	return fmt.Sprintf("DNSName{Name:%q}", n.Name)
}