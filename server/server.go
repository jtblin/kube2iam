package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cenk/backoff"
	"github.com/gorilla/mux"

	"github.com/jtblin/kube2iam"
	"github.com/jtblin/kube2iam/iam"
	"github.com/jtblin/kube2iam/k8s"
	"github.com/jtblin/kube2iam/store"
)

const (
	defaultAppPort         = "8181"
	defaultIAMRoleKey      = "iam.amazonaws.com/role"
	defaultMaxElapsedTime  = 2 * time.Second
	defaultMaxInterval     = 1 * time.Second
	defaultMetadataAddress = "169.254.169.254"
	defaultNamespaceKey    = "iam.amazonaws.com/allowed-roles"
)

// Server encapsulates all of the parameters necessary for starting up
// the server. These can either be set via command line or directly.
type Server struct {
	APIServer               string
	APIToken                string
	AppPort                 string
	BaseRoleARN             string
	DefaultIAMRole          string
	IAMRoleKey              string
	MetadataAddress         string
	HostInterface           string
	HostIP                  string
	NamespaceKey            string
	AddIPTablesRule         bool
	AutoDiscoverBaseArn     bool
	AutoDiscoverDefaultRole bool
	Debug                   bool
	Insecure                bool
	NamespaceRestriction    bool
	Verbose                 bool
	Version                 bool
	iam                     *iam.Client
	k8s                     *k8s.Client
	store                   *store.Store
	BackoffMaxElapsedTime   time.Duration
	BackoffMaxInterval      time.Duration
}

type appHandler func(http.ResponseWriter, *http.Request)

// ServeHTTP implements the net/http server Handler interface
// and recovers from panics.
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Requesting %s", r.RequestURI)
	log.Debugf("RemoteAddr %s", parseRemoteAddr(r.RemoteAddr))
	defer func() {
		var err error
		if rec := recover(); rec != nil {
			switch t := rec.(type) {
			case string:
				err = errors.New(t)
			case error:
				err = t
			default:
				err = errors.New("Unknown error")
			}
			log.Errorf("PANIC error processing request for %s: %+v", r.RequestURI, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()
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

// HealthResponse represents a response for the health check.
type HealthResponse struct {
	HostIP     string `json:"hostIP"`
	InstanceID string `json:"instanceId"`
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get(fmt.Sprintf("http://%s/latest/meta-data/instance-id", s.MetadataAddress))
	if err != nil {
		log.Errorf("Error getting instance id %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error getting instance id, got status: %+s", resp.Status)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	instanceID, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response body %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	health := &HealthResponse{InstanceID: string(instanceID), HostIP: s.HostIP}
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Errorf("Error sending json %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) debugStoreHandler(w http.ResponseWriter, r *http.Request) {
	output := make(map[string]interface{})

	output["rolesByIP"] = s.store.DumpRolesByIP()
	output["rolesByNamespace"] = s.store.DumpRolesByNamespace()
	output["namespaceByIP"] = s.store.DumpNamespaceByIP()

	o, err := json.Marshal(output)
	if err != nil {
		log.Errorf("Error converting debug map to json: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	write(w, string(o))
}

func (s *Server) securityCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "EC2ws")
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	role, err := s.getRole(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	roleARN := s.iam.RoleARN(role)
	// If a base ARN has been supplied and this is not cross-account then
	// return a simple role-name, otherwise return the full ARN
	if s.iam.BaseARN != "" && strings.HasPrefix(roleARN, s.iam.BaseARN) {
		idx := strings.LastIndex(roleARN, "/")
		write(w, roleARN[idx+1:])
		return
	}
	write(w, roleARN)
}

func (s *Server) roleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "EC2ws")
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	podRole, err := s.getRole(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	podRoleARN := s.iam.RoleARN(podRole)

	isRestricted, namespace := s.store.CheckNamespaceRestriction(podRoleARN, remoteIP)
	if !isRestricted {
		http.Error(w, fmt.Sprintf("Role requested %s not valid for namespace of pod at %s with namespace %s", podRole, remoteIP, namespace), http.StatusNotFound)
		return
	}
	allowedRole := podRole
	allowedRoleARN := podRoleARN

	wantedRole := mux.Vars(r)["role"]
	wantedRoleARN := s.iam.RoleARN(wantedRole)
	log.Debugf("Pod with RemoteAddr %s is annotated with role '%s' ('%s'), wants role '%s' ('%s')",
		remoteIP, allowedRole, allowedRoleARN, wantedRole, wantedRoleARN)
	if wantedRoleARN != allowedRoleARN {
		log.Errorf("Invalid role '%s' ('%s') for RemoteAddr %s: does not match annotated role '%s' ('%s')",
			wantedRole, wantedRoleARN, remoteIP, allowedRole, allowedRoleARN)
		http.Error(w, fmt.Sprintf("Invalid role %s", wantedRole), http.StatusForbidden)
		return
	}

	credentials, err := s.iam.AssumeRole(wantedRoleARN, remoteIP)
	if err != nil {
		log.Errorf("Error assuming role %+v for pod at %s with namespace %s", err, remoteIP, namespace)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(credentials); err != nil {
		log.Errorf("Error sending json %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) reverseProxyHandler(w http.ResponseWriter, r *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: s.MetadataAddress})
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
	k, err := k8s.NewClient(host, token, insecure)
	if err != nil {
		return err
	}
	s.k8s = k
	s.iam = iam.NewClient(s.BaseRoleARN)
	model := store.NewStore(s.IAMRoleKey, s.DefaultIAMRole, s.NamespaceRestriction, s.NamespaceKey, s.iam)
	s.store = model
	s.k8s.WatchForPods(kube2iam.NewPodHandler(model))
	s.k8s.WatchForNamespaces(kube2iam.NewNamespaceHandler(model))
	r := mux.NewRouter()
	if s.Debug {
		// This is a potential security risk if enabled in some clusters, hence the flag
		r.Handle("/debug/store", appHandler(s.debugStoreHandler))
	}
	r.Handle("/{version}/meta-data/iam/security-credentials/", appHandler(s.securityCredentialsHandler))
	r.Handle("/{version}/meta-data/iam/security-credentials/{role:.*}", appHandler(s.roleHandler))
	r.Handle("/healthz", appHandler(s.healthHandler))
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
		AppPort:               defaultAppPort,
		BackoffMaxElapsedTime: defaultMaxElapsedTime,
		IAMRoleKey:            defaultIAMRoleKey,
		BackoffMaxInterval:    defaultMaxInterval,
		MetadataAddress:       defaultMetadataAddress,
		NamespaceKey:          defaultNamespaceKey,
	}
}
