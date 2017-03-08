package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cenk/backoff"
	"github.com/gorilla/mux"
)

// Server encapsulates all of the parameters necessary for starting up
// the server. These can either be set via command line or directly.
type Server struct {
	APIServer             string
	APIToken              string
	AppPort               string
	BaseRoleARN           string
	DefaultIAMRole        string
	IAMRoleKey            string
	MetadataAddress       string
	HostInterface         string
	HostIP                string
	BackoffMaxInterval    time.Duration
	BackoffMaxElapsedTime time.Duration
	AddIPTablesRule       bool
	Debug                 bool
	Insecure              bool
	Verbose               bool
	Version               bool
	NamespaceRestriction  bool
	NamespaceKey          string
	iam                   *iam
	k8s                   *k8s
	store                 *store
}

type appHandler func(http.ResponseWriter, *http.Request)

// ServeHTTP implements the net/http server Handler interface.
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Requesting %s", r.RequestURI)
	log.Debugf("RemoteAddr %s", parseRemoteAddr(r.RemoteAddr))
	w.Header().Set("Server", "EC2ws")
	fn(w, r)
}

func parseRemoteAddr(addr string) string {
	n := strings.IndexByte(addr, ':')
	if n <= 1 {
		return ""
	}
	hostname := addr[0:n]
	if net.ParseIP(hostname) == nil {
		return ""
	}
	return hostname
}

func (s *Server) getRole(IP string) (string, error) {
	var role string
	var err error
	operation := func() error {
		role, err = s.store.Get(IP)
		return err
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxInterval = s.BackoffMaxInterval
	expBackoff.MaxElapsedTime = s.BackoffMaxElapsedTime

	err = backoff.Retry(operation, expBackoff)
	if err != nil {
		return "", err
	}

	return role, nil
}

func (s *Server) debugStoreHandler(w http.ResponseWriter, r *http.Request) {
	output := make(map[string]interface{})

	output["rolesByIP"] = s.store.DumpRolesByIP()
	output["rolesByNamespace"] = s.store.DumpRolesByNamespace()
	output["namespaceByIP"] = s.store.DumpNamespaceByIP()

	o, err := json.Marshal(output)
	if err != nil {
		log.Errorf("Error converting debug map to json: %+v", err)
	}

	if _, err := w.Write(o); err != nil {
		log.Errorf("Error writing response: %+v", err)
	}
}

func (s *Server) securityCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	role, err := s.getRole(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	roleARN := s.iam.roleARN(role)
	// If a base ARN has been supplied and this is not cross-account then
	// return a simple role-name, otherwise return the full ARN
	if s.iam.baseARN != "" && strings.HasPrefix(roleARN, s.iam.baseARN) {
		idx := strings.LastIndex(roleARN, "/")
		write(w, roleARN[idx+1:])
		return
	}
	write(w, roleARN)
}

func (s *Server) roleHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	podRole, err := s.getRole(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	podRoleARN := s.iam.roleARN(podRole)

	if !s.store.CheckNamespaceRestriction(podRoleARN, remoteIP) {
		http.Error(w, fmt.Sprintf("Role requested %s not valid for namespace of pod at %s", podRole, remoteIP), http.StatusNotFound)
		return
	}
	allowedRole := podRole
	allowedRoleARN := podRoleARN

	wantedRole := mux.Vars(r)["role"]
	wantedRoleARN := s.iam.roleARN(wantedRole)
	log.Debugf("Pod with RemoteAddr %s is annotated with role '%s' ('%s'), wants role '%s' ('%s')",
		remoteIP, allowedRole, allowedRoleARN, wantedRole, wantedRoleARN)
	if wantedRoleARN != allowedRoleARN {
		log.Errorf("Invalid role '%s' ('%s') for RemoteAddr %s: does not match annotated role '%s' ('%s')",
			wantedRole, wantedRoleARN, remoteIP, allowedRole, allowedRoleARN)
		http.Error(w, fmt.Sprintf("Invalid role %s", wantedRole), http.StatusForbidden)
		return
	}

	credentials, err := s.iam.assumeRole(wantedRoleARN, remoteIP)
	if err != nil {
		log.Errorf("Error assuming role %+v for pod at %s", err, remoteIP)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(credentials); err != nil {
		log.Errorf("Error sending json %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) reverseProxyHandler(w http.ResponseWriter, r *http.Request) {
	director := func(req *http.Request) {
		req = r
		req.URL.Scheme = "http"
		req.URL.Host = s.MetadataAddress
	}
	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(w, r)
	log.Debugf("Proxied %s", r.RequestURI)
}

func write(w http.ResponseWriter, s string) {
	if _, err := w.Write([]byte(s)); err != nil {
		log.Errorf("Error writing response: %+v", err)
	}
}

// Run runs the specified Server.
func (s *Server) Run(host, token string, insecure bool) error {
	k8s, err := newK8s(host, token, insecure)
	if err != nil {
		return err
	}
	s.k8s = k8s
	s.iam = newIAM(s.BaseRoleARN)
	model := newStore(s.IAMRoleKey, s.DefaultIAMRole, s.NamespaceRestriction, s.NamespaceKey, s.iam)
	s.store = model
	s.k8s.watchForPods(newPodhandler(model))
	s.k8s.watchForNamespaces(newNamespacehandler(model))
	r := mux.NewRouter()
	if s.Debug {
		// This is a potential security risk if enabled in some clusters, hence the flag
		r.Handle("/debug/store", appHandler(s.debugStoreHandler))
	}
	r.Handle("/{version}/meta-data/iam/security-credentials/", appHandler(s.securityCredentialsHandler))
	r.Handle("/{version}/meta-data/iam/security-credentials/{role:.*}", appHandler(s.roleHandler))
	r.Handle("/{path:.*}", appHandler(s.reverseProxyHandler))

	log.Infof("Listening on port %s", s.AppPort)
	if err := http.ListenAndServe(":"+s.AppPort, r); err != nil {
		log.Fatalf("Error creating http server: %+v", err)
	}
	return nil
}

// NewServer will create a new Server with default values.
func NewServer() *Server {
	return &Server{
		AppPort:         "8181",
		IAMRoleKey:      "iam.amazonaws.com/role",
		MetadataAddress: "169.254.169.254",
		NamespaceKey:    "kube2iam/allowed-roles",
	}
}
