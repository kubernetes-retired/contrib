package haproxy

import (
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"strings"

	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
)

const (
	// Global
	maxconn      = 4096
	maxpipes     = 2048
	spreadChecks = 5
	debug        = false

	// Defaults
	mode            = "http"
	balance         = "roundrobin"
	maxconnUpstream = 2048
	tcpLog          = true
	httpLog         = false
	abortOnClose    = true
	httpServerClose = true
	forwardFor      = true
	retries         = 3
	redispatch      = true
	timeoutConnect  = "5s"
	timeoutClient   = "30s"
	timeoutServer   = "30s"
	timeoutCheck    = "5s"
	dontLogNull     = true

	// Stats
	statsEnable = false
	statsAuth   = "admin:youshouldntbeusingth1$.passwrd"

	serverCheckInter = 10000
)

// HAProxy balancer instance
type HAProxy struct {
	Global    Global
	Defaults  Defaults
	Frontends map[int]FrontEnd
	Backends  map[string]Backend
	CertsDir  string
}

// FrontEnd listening and request parsing
type FrontEnd struct {
	Name           string
	Bind           Bind
	ACLs           map[string]ACL
	DefaultBackend UseBackend
	UseBackends    []UseBackend
	Opts           []string
}

// UseBackendsByPrio returns ordered use_backend by priority
func (fe FrontEnd) UseBackendsByPrio() []UseBackend {
	sort.Sort(UseBackends(fe.UseBackends))
	return fe.UseBackends
}

// Bind ip port and certificates
type Bind struct {
	IP   string
	Port int
	// TODO part away from balancer.Certs and create a struct that also holds the filename
	Certs []balancer.Certificate
}

// IsTLS retuns true if some certificates need to be configured for the binded port
func (b Bind) IsTLS() bool {
	return len(b.Certs) != 0
}

// func (b *Bind) CertFiles() []string {
// 	certs := []string{}
// 	for _, c := range b.Certs{
// 		certs =
// 	}
// }

// ACL from request
type ACL struct {
	Name    string
	Content string
}

// NewHostNameACL returns a host check ACL
func NewHostNameACL(h string) *ACL {
	return &ACL{
		Content: fmt.Sprintf("hdr(host) -i %s", h),
		Name:    convertToValidName("ishost_" + h),
	}
}

// NewPathACL returns a path check ACL
func NewPathACL(p string) *ACL {
	return &ACL{
		Content: fmt.Sprintf("path_beg %s", p),
		Name:    convertToValidName("ispath_" + p),
	}
}

// Backend servers group
type Backend struct {
	Servers map[string]Server
}

// TODO delete!
// Name accourding to backend contents
func (b *Backend) Name() string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%#v", b.Servers))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Server for backend
type Server struct {
	Address    string
	Port       int
	CheckInter int
	Opts       []string
}

// Name according to Server contents
func (s *Server) Name() string {
	return convertToValidName(fmt.Sprintf("%s_port_%d", s.Address, s.Port))
}

// UseBackend associates backends and ACLs
type UseBackend struct {
	Priority int
	Backend  string
	ACLs     []ACL
}

// UseBackends holds a sortable collection of UseBackend
type UseBackends []UseBackend

// Len of use_backends
func (u UseBackends) Len() int {
	return len(u)
}

// Swap use backends
func (u UseBackends) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

// Less compares use_backend priotities
func (u UseBackends) Less(i, j int) bool {
	return u[i].Priority < u[j].Priority
}

// Certs certificates
type Certs struct {
	Name    string
	Content string
}

// Global HAProxy section
type Global struct {
	// maximum concurrent connections
	Maxconn int
	// maximum number of pipes for kernel splicing
	Maxpipes int
	// Randomness for health checks intervals
	SpreadChecks int
	// Activate verbose logging
	Debug bool

	// placeholder for other global parameters
	Others map[string]string
}

// Defaults HAProxy section
type Defaults struct {
	// running mode for HAProxy
	Mode string
	// default load balancing strategy
	Balance string
	// maximum number of connections for an upstream server
	Maxconn int
	// enables rich TCP log
	TCPLog bool
	// enables rich HTTP log
	HTTPLog bool
	// early drop of aborted requests
	AbortOnClose bool
	// enables connection close, allowing also keep-alive
	HTTPServerClose bool
	// adds X-Forwarded-For header
	ForwardFor bool
	// number of retries to servers
	Retries int
	// break affinity when the upstream server is down
	Redispatch bool
	// timeout for connections to an upstream server
	TimeoutConnect string
	// timeout for client inactivity
	TimeoutClient string
	// timeout for server inactivity
	TimeoutServer string
	// avoid logging empty data connections
	DontLogNull bool
	// timeout for health checks
	TimeoutCheck string

	// placeholder for other global parameters
	Others map[string]string
}

// NewDefaultBalancer returns an HAProxy balancer
func NewDefaultBalancer() *HAProxy {

	ha := &HAProxy{
		Global: Global{
			Maxconn:      maxconn,
			Maxpipes:     maxpipes,
			SpreadChecks: spreadChecks,
			Debug:        debug,
		},
		Defaults: Defaults{
			Mode:            mode,
			Balance:         balance,
			Maxconn:         maxconnUpstream,
			TCPLog:          tcpLog,
			HTTPLog:         httpLog,
			AbortOnClose:    abortOnClose,
			HTTPServerClose: httpServerClose,
			ForwardFor:      forwardFor,
			Retries:         retries,
			Redispatch:      redispatch,
			TimeoutConnect:  timeoutConnect,
			TimeoutClient:   timeoutClient,
			TimeoutServer:   timeoutServer,
			DontLogNull:     dontLogNull,
			TimeoutCheck:    timeoutCheck,
		},
	}

	ha.Backends = make(map[string]Backend)
	ha.Frontends = make(map[int]FrontEnd)

	return ha
}

// NewDefaultBackendServer returns a backend server
func NewDefaultBackendServer() *Server {
	return &Server{
		CheckInter: serverCheckInter,
	}
}

// convertToValidName replace non valid characters in config names
func convertToValidName(s string) string {
	s = strings.TrimSpace(s)
	r := strings.NewReplacer("/", "_", ".", "_")
	return r.Replace(s)
}
