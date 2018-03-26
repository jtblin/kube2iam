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

	"github.com/cenk/backoff"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"

	"github.com/jtblin/kube2iam"
	"github.com/jtblin/kube2iam/iam"
	"github.com/jtblin/kube2iam/k8s"
	"github.com/jtblin/kube2iam/mappings"
)

const (
	defaultAppPort           = "8181"
	defaultCacheSyncAttempts = 10
	defaultIAMRoleKey        = "iam.amazonaws.com/role"
	defaultLogLevel          = "info"
	defaultLogFormat         = "text"
	defaultMaxElapsedTime    = 2 * time.Second
	defaultMaxInterval       = 1 * time.Second
	defaultMetadataAddress   = "169.254.169.254"
	defaultNamespaceKey      = "iam.amazonaws.com/allowed-roles"
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
	LogLevel                string
	LogFormat               string
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
	roleMapper              *mappings.RoleMapper
	BackoffMaxElapsedTime   time.Duration
	BackoffMaxInterval      time.Duration
}

type appHandler func(*log.Entry, http.ResponseWriter, *http.Request)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

// ServeHTTP implements the net/http server Handler interface
// and recovers from panics.
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.WithFields(log.Fields{
		"req.method": r.Method,
		"req.path":   r.URL.Path,
		"req.remote": parseRemoteAddr(r.RemoteAddr),
	})
	start := time.Now()
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
			logger.WithField("res.status", http.StatusInternalServerError).
				Errorf("PANIC error processing request: %+v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()
	rw := newResponseWriter(w)
	fn(logger, rw, r)
	if r.URL.Path != "/healthz" {
		latency := time.Since(start)
		logger.WithFields(log.Fields{"res.duration": latency.Nanoseconds(), "res.status": rw.statusCode}).
			Infof("%s %s (%d) took %d ns", r.Method, r.URL.Path, rw.statusCode, latency.Nanoseconds())
	}
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

func (s *Server) getRoleMapping(IP string) (*mappings.RoleMappingResult, error) {
	var roleMapping *mappings.RoleMappingResult
	var err error
	operation := func() error {
		roleMapping, err = s.roleMapper.GetRoleMapping(IP)
		return err
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxInterval = s.BackoffMaxInterval
	expBackoff.MaxElapsedTime = s.BackoffMaxElapsedTime

	err = backoff.Retry(operation, expBackoff)
	if err != nil {
		return nil, err
	}

	return roleMapping, nil
}

// HealthResponse represents a response for the health check.
type HealthResponse struct {
	HostIP     string `json:"hostIP"`
	InstanceID string `json:"instanceId"`
}

func (s *Server) healthHandler(logger *log.Entry, w http.ResponseWriter, r *http.Request) {
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

func (s *Server) debugStoreHandler(logger *log.Entry, w http.ResponseWriter, r *http.Request) {
	o, err := json.Marshal(s.roleMapper.DumpDebugInfo())
	if err != nil {
		log.Errorf("Error converting debug map to json: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	write(logger, w, string(o))
}

func (s *Server) securityCredentialsHandler(logger *log.Entry, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "EC2ws")
	remoteIP := parseRemoteAddr(r.RemoteAddr)
	roleMapping, err := s.getRoleMapping(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// If a base ARN has been supplied and this is not cross-account then
	// return a simple role-name, otherwise return the full ARN
	if s.iam.BaseARN != "" && strings.HasPrefix(roleMapping.Role, s.iam.BaseARN) {
		write(logger, w, strings.TrimPrefix(roleMapping.Role, s.iam.BaseARN))
		return
	}
	write(logger, w, roleMapping.Role)
}

func (s *Server) roleHandler(logger *log.Entry, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "EC2ws")
	remoteIP := parseRemoteAddr(r.RemoteAddr)

	roleMapping, err := s.getRoleMapping(remoteIP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	roleLogger := logger.WithFields(log.Fields{
		"pod.iam.role": roleMapping.Role,
		"ns.name":      roleMapping.Namespace,
	})

	wantedRole := mux.Vars(r)["role"]
	wantedRoleARN := s.iam.RoleARN(wantedRole)

	if wantedRoleARN != roleMapping.Role {
		roleLogger.WithField("params.iam.role", wantedRole).
			Error("Invalid role: does not match annotated role")
		http.Error(w, fmt.Sprintf("Invalid role %s", wantedRole), http.StatusForbidden)
		return
	}

	credentials, err := s.iam.AssumeRole(wantedRoleARN, remoteIP)
	if err != nil {
		roleLogger.Errorf("Error assuming role %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(credentials); err != nil {
		roleLogger.Errorf("Error sending json %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) reverseProxyHandler(logger *log.Entry, w http.ResponseWriter, r *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: s.MetadataAddress})
	proxy.ServeHTTP(w, r)
	logger.WithField("metadata.url", s.MetadataAddress).Debug("Proxy ec2 metadata request")
}

func write(logger *log.Entry, w http.ResponseWriter, s string) {
	if _, err := w.Write([]byte(s)); err != nil {
		logger.Errorf("Error writing response: %+v", err)
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
	s.roleMapper = mappings.NewRoleMapper(s.IAMRoleKey, s.DefaultIAMRole, s.NamespaceRestriction, s.NamespaceKey, s.iam, s.k8s)
	podSynched := s.k8s.WatchForPods(kube2iam.NewPodHandler(s.IAMRoleKey))
	namespaceSynched := s.k8s.WatchForNamespaces(kube2iam.NewNamespaceHandler(s.NamespaceKey))

	synced := false
	for i := 0; i < defaultCacheSyncAttempts && !synced; i++ {
		synced = cache.WaitForCacheSync(nil, podSynched, namespaceSynched)
	}

	if !synced {
		log.Fatalf("Attempted to wait for caches to be synced for %d however it is not done.  Giving up.", defaultCacheSyncAttempts)
	} else {
		log.Debugln("Caches have been synced.  Proceeding with server.")
	}

	r := mux.NewRouter()

	if s.Debug {
		// This is a potential security risk if enabled in some clusters, hence the flag
		r.Handle("/debug/store", appHandler(s.debugStoreHandler))
	}
	r.Handle("/{version}/meta-data/iam/security-credentials", appHandler(s.securityCredentialsHandler))
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
		LogLevel:              defaultLogLevel,
		LogFormat:             defaultLogFormat,
		MetadataAddress:       defaultMetadataAddress,
		NamespaceKey:          defaultNamespaceKey,
	}
}
