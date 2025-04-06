package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gorilla/mux"
	"github.com/jtblin/kube2iam/iam"
	"github.com/jtblin/kube2iam/mappings"
	"github.com/karlseguin/ccache"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---- Mock implementations --------------------------------------------------

// mockStore satisfies the mappings.store interface.
type mockStore struct {
	pod       *v1.Pod
	podErr    error
	namespace *v1.Namespace
	nsErr     error
	podIPs    []string
	nsNames   []string
}

func (m *mockStore) ListPodIPs() []string {
	if m.podIPs != nil {
		return m.podIPs
	}
	if m.pod != nil {
		return []string{m.pod.Status.PodIP}
	}
	return nil
}
func (m *mockStore) ListNamespaces() []string {
	if m.nsNames != nil {
		return m.nsNames
	}
	if m.namespace != nil {
		return []string{m.namespace.Name}
	}
	return nil
}
func (m *mockStore) PodByIP(_ string) (*v1.Pod, error)               { return m.pod, m.podErr }
func (m *mockStore) NamespaceByName(_ string) (*v1.Namespace, error) { return m.namespace, m.nsErr }

// mockSTSClient implements iam.STSClient.
type mockSTSClient struct {
	output *sts.AssumeRoleOutput
	err    error
}

func (m *mockSTSClient) AssumeRole(_ context.Context, _ *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	return m.output, m.err
}

// mockRegionClient implements iam.RegionClient.
type mockRegionClient struct{}

func (m *mockRegionClient) DescribeRegions(_ context.Context, _ *ec2.DescribeRegionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return &ec2.DescribeRegionsOutput{
		Regions: []types.Region{
			{RegionName: aws.String("us-east-1"), Endpoint: aws.String("ec2.us-east-1.amazonaws.com")},
		},
	}, nil
}

// mockIMDSClient implements iam.IMDSClient.
type mockIMDSClient struct {
	instanceID string
	err        error
}

func (m *mockIMDSClient) GetMetadata(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	body := m.instanceID
	return &imds.GetMetadataOutput{Content: noopReadCloser(body)}, nil
}

func (m *mockIMDSClient) GetIAMInfo(_ context.Context, _ *imds.GetIAMInfoInput, _ ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
	return &imds.GetIAMInfoOutput{}, nil
}

// noopReadCloser creates an io.ReadCloser from a string.
func noopReadCloser(s string) interface {
	Read([]byte) (int, error)
	Close() error
} {
	return nopCloser{strings.NewReader(s)}
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }

// ---- Helpers ----------------------------------------------------------------

func newLogger() *log.Entry {
	return log.WithFields(log.Fields{})
}

func newTestIAMClient(baseARN string, creds *iam.Credentials, stsErr error) *iam.Client {
	var stsOutput *sts.AssumeRoleOutput
	if creds != nil {
		exp := time.Now().Add(time.Hour)
		stsOutput = &sts.AssumeRoleOutput{
			Credentials: &ststypes.Credentials{
				AccessKeyId:     aws.String(creds.AccessKeyID),
				SecretAccessKey: aws.String(creds.SecretAccessKey),
				SessionToken:    aws.String(creds.Token),
				Expiration:      &exp,
			},
		}
	}
	return &iam.Client{
		BaseARN: baseARN,
		Cache:   ccache.New(ccache.Configure()),
		STS:     &mockSTSClient{output: stsOutput, err: stsErr},
		Region:  &mockRegionClient{},
	}
}

func newRoleMapper(pod *v1.Pod, podErr error, ns *v1.Namespace, nsErr error, baseARN, defaultRole string, nsRestriction bool) *mappings.RoleMapper {
	store := &mockStore{pod: pod, podErr: podErr, namespace: ns, nsErr: nsErr}
	iamClient := &iam.Client{BaseARN: baseARN}
	return mappings.NewRoleMapper(
		defaultIAMRoleKey,
		defaultIAMExternalID,
		defaultRole,
		nsRestriction,
		defaultNamespaceKey,
		iamClient,
		store,
		"glob",
	)
}

func buildServer(roleMapper *mappings.RoleMapper, iamClient *iam.Client) *Server {
	s := NewServer()
	s.roleMapper = roleMapper
	s.iam = iamClient
	return s
}

// setMuxVars injects gorilla/mux route variables into a request.
func setMuxVars(r *http.Request, vars map[string]string) *http.Request {
	return mux.SetURLVars(r, vars)
}

// ---- parseRemoteAddr --------------------------------------------------------

func TestParseRemoteAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid IPv4 with port", "10.0.0.1:8080", "10.0.0.1"},
		{"localhost with port", "127.0.0.1:12345", "127.0.0.1"},
		{"no port", "10.0.0.1", "10.0.0.1"},
		{"IPv6 without port", "fd00:ec2::254", "fd00:ec2::254"},
		{"IPv6 with port", "[fd00:ec2::254]:8181", "fd00:ec2::254"},
		{"single char before colon", "a:", ""},
		{"non-IP hostname", "myhost:1234", ""},
		{"empty string", "", ""},
		{"invalid address", "abcd", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRemoteAddr(tt.input)
			if got != tt.expected {
				t.Errorf("parseRemoteAddr(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---- healthHandler ----------------------------------------------------------

func TestHealthHandlerHealthy(t *testing.T) {
	s := NewServer()
	s.HealthcheckFailReason = ""
	s.InstanceID = "i-0abcdef1234567890"
	s.HostIP = "10.0.0.42"

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rw := httptest.NewRecorder()
	s.healthHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	var resp HealthResponse
	if err := json.NewDecoder(rw.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.InstanceID != "i-0abcdef1234567890" {
		t.Errorf("expected InstanceID 'i-0abcdef1234567890', got %q", resp.InstanceID)
	}
	if resp.HostIP != "10.0.0.42" {
		t.Errorf("expected HostIP '10.0.0.42', got %q", resp.HostIP)
	}
}

func TestHealthHandlerUnhealthy(t *testing.T) {
	s := NewServer()
	s.HealthcheckFailReason = "IMDS unreachable"

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rw := httptest.NewRecorder()
	s.healthHandler(newLogger(), rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rw.Code)
	}
	if !strings.Contains(rw.Body.String(), "IMDS unreachable") {
		t.Errorf("expected body to contain failure reason, got %q", rw.Body.String())
	}
}

// ---- securityCredentialsHandler ---------------------------------------------

func TestSecurityCredentialsHandlerFound(t *testing.T) {
	const baseARN = "arn:aws:iam::123456789012:role/"
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: "my-role"},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.1", Phase: v1.PodRunning},
	}

	roleMapper := newRoleMapper(pod, nil, nil, nil, baseARN, "", false)
	iamClient := &iam.Client{BaseARN: baseARN}
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rw := httptest.NewRecorder()
	s.securityCredentialsHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
	// With BaseARN prefix match, returns just the role name (prefix stripped)
	body := strings.TrimSpace(rw.Body.String())
	if body != "my-role" {
		t.Errorf("expected 'my-role', got %q", body)
	}
}

func TestSecurityCredentialsHandlerCrossAccountARN(t *testing.T) {
	const baseARN = "arn:aws:iam::111111111111:role/"
	const crossAccountARN = "arn:aws:iam::999999999999:role/cross-role"
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: crossAccountARN},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.2", Phase: v1.PodRunning},
	}

	roleMapper := newRoleMapper(pod, nil, nil, nil, baseARN, "", false)
	iamClient := &iam.Client{BaseARN: baseARN}
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials", nil)
	req.RemoteAddr = "10.0.0.2:9999"
	rw := httptest.NewRecorder()
	s.securityCredentialsHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
	body := strings.TrimSpace(rw.Body.String())
	if body != crossAccountARN {
		t.Errorf("expected full cross-account ARN, got %q", body)
	}
}

func TestSecurityCredentialsHandlerPodNotFound(t *testing.T) {
	roleMapper := newRoleMapper(nil, errors.New("pod not found"), nil, nil, "", "", false)
	s := buildServer(roleMapper, &iam.Client{})

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials", nil)
	req.RemoteAddr = "10.99.0.1:9999"
	rw := httptest.NewRecorder()
	s.securityCredentialsHandler(newLogger(), rw, req)

	if rw.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rw.Code)
	}
}

// ---- roleHandler ------------------------------------------------------------

func TestRoleHandlerMatch(t *testing.T) {
	const baseARN = "arn:aws:iam::123456789012:role/"
	const roleName = "my-role"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: roleName},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.10", Phase: v1.PodRunning},
	}

	roleMapper := newRoleMapper(pod, nil, nil, nil, baseARN, "", false)
	iamClient := newTestIAMClient(baseARN, &iam.Credentials{
		AccessKeyID:     "AKIATEST",
		SecretAccessKey: "secret",
		Token:           "token",
	}, nil)
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/latest/meta-data/iam/security-credentials/%s", roleName), nil)
	req.RemoteAddr = "10.0.0.10:9999"
	req = setMuxVars(req, map[string]string{"role": roleName})
	rw := httptest.NewRecorder()
	s.roleHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
	var creds iam.Credentials
	if err := json.NewDecoder(rw.Body).Decode(&creds); err != nil {
		t.Fatalf("failed to decode credentials: %v", err)
	}
	if creds.AccessKeyID != "AKIATEST" {
		t.Errorf("expected AccessKeyID 'AKIATEST', got %q", creds.AccessKeyID)
	}
}

func TestRoleHandlerMismatch(t *testing.T) {
	const baseARN = "arn:aws:iam::123456789012:role/"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: "annotated-role"},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.11", Phase: v1.PodRunning},
	}

	roleMapper := newRoleMapper(pod, nil, nil, nil, baseARN, "", false)
	iamClient := &iam.Client{BaseARN: baseARN}
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials/different-role", nil)
	req.RemoteAddr = "10.0.0.11:9999"
	req = setMuxVars(req, map[string]string{"role": "different-role"})
	rw := httptest.NewRecorder()
	s.roleHandler(newLogger(), rw, req)

	if rw.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rw.Code, rw.Body.String())
	}
}

func TestRoleHandlerMappingError(t *testing.T) {
	roleMapper := newRoleMapper(nil, errors.New("pod not found"), nil, nil, "", "", false)
	s := buildServer(roleMapper, &iam.Client{})

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials/some-role", nil)
	req.RemoteAddr = "10.99.99.99:9999"
	req = setMuxVars(req, map[string]string{"role": "some-role"})
	rw := httptest.NewRecorder()
	s.roleHandler(newLogger(), rw, req)

	if rw.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rw.Code)
	}
}

func TestRoleHandlerSTSError(t *testing.T) {
	const baseARN = "arn:aws:iam::123456789012:role/"
	const roleName = "failing-role"

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: roleName},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.12", Phase: v1.PodRunning},
	}

	roleMapper := newRoleMapper(pod, nil, nil, nil, baseARN, "", false)
	iamClient := newTestIAMClient(baseARN, nil, errors.New("AccessDenied"))
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/latest/meta-data/iam/security-credentials/%s", roleName), nil)
	req.RemoteAddr = "10.0.0.12:9999"
	req = setMuxVars(req, map[string]string{"role": roleName})
	rw := httptest.NewRecorder()
	s.roleHandler(newLogger(), rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rw.Code, rw.Body.String())
	}
}

// ---- reverseProxyHandler ----------------------------------------------------

func TestReverseProxyHandlerIMDSv2TokenRoute(t *testing.T) {
	// The PUT /latest/api/token route should clear RemoteAddr before proxying.
	// We spin up a test backend to capture what the proxy forwards.
	var capturedXFF string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedXFF = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	s := NewServer()
	s.MetadataAddress = strings.TrimPrefix(backend.URL, "http://")

	req := httptest.NewRequest(http.MethodPut, "/latest/api/token", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rw := httptest.NewRecorder()
	s.reverseProxyHandler(newLogger(), rw, req)

	// When RemoteAddr is cleared, the proxy adds no X-Forwarded-For.
	if capturedXFF != "" {
		t.Errorf("expected no X-Forwarded-For for IMDSv2 token PUT, got %q", capturedXFF)
	}
}

func TestReverseProxyHandlerIMDSv2GETWithToken(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	s := NewServer()
	s.MetadataAddress = strings.TrimPrefix(backend.URL, "http://")

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/instance-id", nil)
	req.Header.Set("X-aws-ec2-metadata-token", "some-token-value")
	req.RemoteAddr = "10.0.0.1:9999"
	rw := httptest.NewRecorder()
	s.reverseProxyHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
}

func TestReverseProxyHandlerStandardRequest(t *testing.T) {
	var capturedXFF string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedXFF = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	s := NewServer()
	s.MetadataAddress = strings.TrimPrefix(backend.URL, "http://")

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/ami-id", nil)
	req.RemoteAddr = "10.0.0.5:9999"
	rw := httptest.NewRecorder()
	s.reverseProxyHandler(newLogger(), rw, req)

	// Standard GET without token header: RemoteAddr is NOT cleared, so X-Forwarded-For is set.
	if capturedXFF == "" {
		t.Error("expected X-Forwarded-For to be set for standard request (RemoteAddr not cleared)")
	}
}

// ---- debugStoreHandler ------------------------------------------------------

func TestDebugStoreHandler(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "debug-pod",
			Namespace:   "default",
			Annotations: map[string]string{defaultIAMRoleKey: "debug-role"},
		},
		Status: v1.PodStatus{PodIP: "10.0.0.20", Phase: v1.PodRunning},
	}
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "default",
			Annotations: map[string]string{defaultNamespaceKey: `["debug-role"]`},
		},
	}

	store := &mockStore{pod: pod, namespace: ns}
	iamClient := &iam.Client{BaseARN: "arn:aws:iam::123456789012:role/"}
	roleMapper := mappings.NewRoleMapper(
		defaultIAMRoleKey, defaultIAMExternalID, "", false,
		defaultNamespaceKey, iamClient, store, "glob",
	)
	s := buildServer(roleMapper, iamClient)

	req := httptest.NewRequest(http.MethodGet, "/debug/store", nil)
	rw := httptest.NewRecorder()
	s.debugStoreHandler(newLogger(), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}
	var result map[string]interface{}
	if err := json.NewDecoder(rw.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode debug response: %v", err)
	}
	for _, key := range []string{"rolesByIP", "namespaceByIP", "rolesByNamespace"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in debug response", key)
		}
	}
}

// ---- doHealthcheck ----------------------------------------------------------

func TestDoHealthcheckSuccess(t *testing.T) {
	s := NewServer()
	s.iam = &iam.Client{
		IMDS: &mockIMDSClient{instanceID: "i-0test123"},
	}

	s.doHealthcheck()

	if s.HealthcheckFailReason != "" {
		t.Errorf("expected empty HealthcheckFailReason, got %q", s.HealthcheckFailReason)
	}
	if s.InstanceID != "i-0test123" {
		t.Errorf("expected InstanceID 'i-0test123', got %q", s.InstanceID)
	}
}

func TestDoHealthcheckFailure(t *testing.T) {
	s := NewServer()
	s.iam = &iam.Client{
		IMDS: &mockIMDSClient{err: errors.New("IMDS timeout")},
	}

	s.doHealthcheck()

	if s.HealthcheckFailReason == "" {
		t.Error("expected HealthcheckFailReason to be set on IMDS failure")
	}
}

// ---- NewServer defaults -----------------------------------------------------

func TestNewServerDefaults(t *testing.T) {
	s := NewServer()
	if s.AppPort != defaultAppPort {
		t.Errorf("AppPort: expected %q, got %q", defaultAppPort, s.AppPort)
	}
	if s.MetadataAddress != defaultMetadataAddress {
		t.Errorf("MetadataAddress: expected %q, got %q", defaultMetadataAddress, s.MetadataAddress)
	}
	if s.IAMRoleKey != defaultIAMRoleKey {
		t.Errorf("IAMRoleKey: expected %q, got %q", defaultIAMRoleKey, s.IAMRoleKey)
	}
	if s.NamespaceRestrictionFormat != defaultNamespaceRestrictionFormat {
		t.Errorf("NamespaceRestrictionFormat: expected %q, got %q", defaultNamespaceRestrictionFormat, s.NamespaceRestrictionFormat)
	}
	if s.HealthcheckFailReason != "Healthcheck not yet performed" {
		t.Errorf("HealthcheckFailReason should indicate not-yet-run, got %q", s.HealthcheckFailReason)
	}
}
