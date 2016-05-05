package cmd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

// Server encapsulates all of the parameters necessary for starting up
// the server. These can either be set via command line or directly.
type Server struct {
	APIServer       string
	APIToken        string
	AppPort         string
	BaseRoleARN     string
	IAMRoleKey      string
	MetadataAddress string
	Insecure        bool
	Verbose         bool
	Version         bool
	iam             *iam
	k8s             *k8s
	store           *store
}

type appHandler func(http.ResponseWriter, *http.Request)

// ServeHTTP implements the net/http server Handler interface.
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("Requesting %s", r.RequestURI)
	log.Debugf("RemoteAddr %s", parseRemoteAddr(r.RemoteAddr))
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

func (s *Server) securityCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	roleARN := s.iam.roleARN(s.store.Get(remoteIP))
	idx := strings.LastIndex(roleARN, "/")
	write(w, roleARN[idx+1:])
}

func (s *Server) roleHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	roleARN := s.iam.roleARN(s.store.Get(remoteIP))
	credentials, err := s.iam.assumeRole(roleARN)
	if err != nil {
		log.Errorf("Error assuming role %+v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	if err := json.NewEncoder(w).Encode(credentials); err != nil {
		log.Errorf("Error sending json %+v", err)
		http.Error(w, err.Error(), 500)
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
	s.store = newStore(s.IAMRoleKey)
	s.k8s.watchForPods(s.store)
	s.iam = newIAM(s.BaseRoleARN)
	r := mux.NewRouter()
	r.Handle("/{version}/meta-data/iam/security-credentials/", appHandler(s.securityCredentialsHandler))
	r.Handle("/{version}/meta-data/iam/security-credentials/{role}", appHandler(s.roleHandler))
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
		AppPort:         "8080",
		MetadataAddress: "169.254.169.254",
	}
}
