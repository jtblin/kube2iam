//go:build integration

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

// Integration tests spin up the real server handler chain with fake k8s stores
// and mock AWS clients. No live cluster or AWS credentials required.
// Run with: go test -tags=integration ./server/...

func integrationIAMClient(baseARN string, creds *iam.Credentials, stsErr error) *iam.Client {
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
		Region: &mockRegionClient2{
			regions: []types.Region{
				{RegionName: aws.String("us-east-1"), Endpoint: aws.String("ec2.us-east-1.amazonaws.com")},
			},
		},
	}
}

type mockRegionClient2 struct {
	regions []types.Region
}

func (m *mockRegionClient2) DescribeRegions(_ context.Context, _ *ec2.DescribeRegionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	return &ec2.DescribeRegionsOutput{Regions: m.regions}, nil
}

type integMockIMDS struct {
	instanceID string
	err        error
}

func (m *integMockIMDS) GetMetadata(_ context.Context, params *imds.GetMetadataInput, _ ...func(*imds.Options)) (*imds.GetMetadataOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &imds.GetMetadataOutput{Content: noopReadCloser(m.instanceID)}, nil
}

func (m *integMockIMDS) GetIAMInfo(_ context.Context, _ *imds.GetIAMInfoInput, _ ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error) {
	return &imds.GetIAMInfoOutput{}, nil
}

type integStore struct {
	pods       map[string]*v1.Pod
	namespaces map[string]*v1.Namespace
}

func (s *integStore) ListPodIPs() []string {
	ips := make([]string, 0, len(s.pods))
	for ip := range s.pods {
		ips = append(ips, ip)
	}
	return ips
}
func (s *integStore) PodByIP(ip string) (*v1.Pod, error) {
	if pod, ok := s.pods[ip]; ok {
		return pod, nil
	}
	return nil, errors.New("pod not found for IP " + ip)
}
func (s *integStore) ListNamespaces() []string {
	names := make([]string, 0, len(s.namespaces))
	for name := range s.namespaces {
		names = append(names, name)
	}
	return names
}
func (s *integStore) NamespaceByName(name string) (*v1.Namespace, error) {
	if ns, ok := s.namespaces[name]; ok {
		return ns, nil
	}
	return nil, errors.New("namespace not found: " + name)
}

func newIntegServer(store *integStore, baseARN string, creds *iam.Credentials, stsErr error, nsRestriction bool) *Server {
	iamClient := integrationIAMClient(baseARN, creds, stsErr)
	roleMapper := mappings.NewRoleMapper(
		defaultIAMRoleKey,
		defaultIAMExternalID,
		"",
		nsRestriction,
		defaultNamespaceKey,
		iamClient,
		store,
		"glob",
	)
	s := NewServer()
	s.iam = iamClient
	s.roleMapper = roleMapper
	return s
}

// TestIntegFullRequestChain exercises the complete request path:
// pod annotation → role mapping → STS AssumeRole → JSON credentials response.
func TestIntegFullRequestChain(t *testing.T) {
	const (
		baseARN  = "arn:aws:iam::123456789012:role/"
		roleName = "my-service-role"
		podIP    = "10.10.0.1"
	)

	store := &integStore{
		pods: map[string]*v1.Pod{
			podIP: {
				ObjectMeta: metav1.ObjectMeta{
					Name:        "my-service",
					Namespace:   "production",
					Annotations: map[string]string{defaultIAMRoleKey: roleName},
				},
				Status: v1.PodStatus{PodIP: podIP, Phase: v1.PodRunning},
			},
		},
		namespaces: map[string]*v1.Namespace{
			"production": {ObjectMeta: metav1.ObjectMeta{Name: "production"}},
		},
	}

	expectedCreds := &iam.Credentials{
		AccessKeyID:     "AKIAINTEG0001",
		SecretAccessKey: "secretkey",
		Token:           "sessiontoken",
	}

	s := newIntegServer(store, baseARN, expectedCreds, nil, false)

	// First: GET security-credentials → returns role name
	credReq := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials", nil)
	credReq.RemoteAddr = podIP + ":12345"
	credRW := httptest.NewRecorder()
	s.securityCredentialsHandler(log.WithFields(log.Fields{}), credRW, credReq)

	if credRW.Code != http.StatusOK {
		t.Fatalf("security-credentials: expected 200, got %d: %s", credRW.Code, credRW.Body.String())
	}
	if credRW.Body.String() != roleName {
		t.Errorf("expected role name %q, got %q", roleName, credRW.Body.String())
	}

	// Second: GET role-specific credentials → returns STS credentials
	roleReq := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials/"+roleName, nil)
	roleReq.RemoteAddr = podIP + ":12345"
	roleReq = mux.SetURLVars(roleReq, map[string]string{"role": roleName})
	roleRW := httptest.NewRecorder()
	s.roleHandler(log.WithFields(log.Fields{}), roleRW, roleReq)

	if roleRW.Code != http.StatusOK {
		t.Fatalf("roleHandler: expected 200, got %d: %s", roleRW.Code, roleRW.Body.String())
	}
	var creds iam.Credentials
	if err := json.NewDecoder(roleRW.Body).Decode(&creds); err != nil {
		t.Fatalf("failed to decode credentials: %v", err)
	}
	if creds.AccessKeyID != expectedCreds.AccessKeyID {
		t.Errorf("expected AccessKeyID %q, got %q", expectedCreds.AccessKeyID, creds.AccessKeyID)
	}
	if creds.Code != "Success" {
		t.Errorf("expected Code 'Success', got %q", creds.Code)
	}
}

// TestIntegNamespaceRestrictionAllowed ensures that when namespace restriction
// is enabled and the pod's role is listed in the namespace annotation, access is granted.
func TestIntegNamespaceRestrictionAllowed(t *testing.T) {
	const (
		baseARN  = "arn:aws:iam::123456789012:role/"
		roleName = "allowed-role"
		podIP    = "10.10.0.2"
	)

	store := &integStore{
		pods: map[string]*v1.Pod{
			podIP: {
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-allowed",
					Namespace:   "restricted-ns",
					Annotations: map[string]string{defaultIAMRoleKey: roleName},
				},
				Status: v1.PodStatus{PodIP: podIP, Phase: v1.PodRunning},
			},
		},
		namespaces: map[string]*v1.Namespace{
			"restricted-ns": {
				ObjectMeta: metav1.ObjectMeta{
					Name:        "restricted-ns",
					Annotations: map[string]string{defaultNamespaceKey: `["allowed-role"]`},
				},
			},
		},
	}

	creds := &iam.Credentials{AccessKeyID: "AKIAINTEG0002", SecretAccessKey: "s", Token: "t"}
	s := newIntegServer(store, baseARN, creds, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials/"+roleName, nil)
	req.RemoteAddr = podIP + ":12345"
	req = mux.SetURLVars(req, map[string]string{"role": roleName})
	rw := httptest.NewRecorder()
	s.roleHandler(log.WithFields(log.Fields{}), rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed role, got %d: %s", rw.Code, rw.Body.String())
	}
}

// TestIntegNamespaceRestrictionDenied ensures that when namespace restriction
// is enabled and the role is NOT listed in the namespace annotation, access is denied.
func TestIntegNamespaceRestrictionDenied(t *testing.T) {
	const (
		baseARN  = "arn:aws:iam::123456789012:role/"
		roleName = "unauthorized-role"
		podIP    = "10.10.0.3"
	)

	store := &integStore{
		pods: map[string]*v1.Pod{
			podIP: {
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-denied",
					Namespace:   "restricted-ns",
					Annotations: map[string]string{defaultIAMRoleKey: roleName},
				},
				Status: v1.PodStatus{PodIP: podIP, Phase: v1.PodRunning},
			},
		},
		namespaces: map[string]*v1.Namespace{
			"restricted-ns": {
				ObjectMeta: metav1.ObjectMeta{
					Name:        "restricted-ns",
					Annotations: map[string]string{defaultNamespaceKey: `["some-other-role"]`},
				},
			},
		},
	}

	s := newIntegServer(store, baseARN, nil, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/latest/meta-data/iam/security-credentials/"+roleName, nil)
	req.RemoteAddr = podIP + ":12345"
	req = mux.SetURLVars(req, map[string]string{"role": roleName})
	rw := httptest.NewRecorder()
	s.roleHandler(log.WithFields(log.Fields{}), rw, req)

	if rw.Code != http.StatusNotFound {
		t.Errorf("expected 404 for denied role (namespace restriction), got %d: %s", rw.Code, rw.Body.String())
	}
}

// TestIntegRoleCredentialCaching ensures that multiple requests for the same role
// result in only a single STS call (cache hit on subsequent requests).
func TestIntegRoleCredentialCaching(t *testing.T) {
	const (
		baseARN  = "arn:aws:iam::123456789012:role/"
		roleName = "cached-role"
	)

	callCount := 0
	stsClient := &countingSTSClient{
		delegate: &mockSTSClient{
			output: &sts.AssumeRoleOutput{
				Credentials: &ststypes.Credentials{
					AccessKeyId:     aws.String("AKIAINTEG0003"),
					SecretAccessKey: aws.String("secret"),
					SessionToken:    aws.String("token"),
					Expiration:      aws.Time(time.Now().Add(time.Hour)),
				},
			},
		},
		count: &callCount,
	}

	iamClient := &iam.Client{
		BaseARN: baseARN,
		Cache:      ccache.New(ccache.Configure()),
		ErrorCache: ccache.New(ccache.Configure()),
		STS:     stsClient,
		Region: &mockRegionClient2{regions: []types.Region{
			{RegionName: aws.String("us-east-1"), Endpoint: aws.String("ec2.us-east-1.amazonaws.com")},
		}},
	}

	roleARN := baseARN + roleName

	// Make 5 AssumeRole calls for the same ARN
	for i := 0; i < 5; i++ {
		_, err := iamClient.AssumeRole(roleARN, "", "10.10.0.4", time.Hour, time.Minute)
		if err != nil {
			t.Fatalf("call %d: AssumeRole failed: %v", i+1, err)
		}
	}

	if callCount != 1 {
		t.Errorf("expected STS to be called exactly once (cache), but was called %d times", callCount)
	}
}

type countingSTSClient struct {
	delegate iam.STSClient
	count    *int
}

func (c *countingSTSClient) AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	*c.count++
	return c.delegate.AssumeRole(ctx, params, optFns...)
}

// TestIntegErrorCaching ensures that STS errors are cached.
func TestIntegErrorCaching(t *testing.T) {
	const (
		baseARN  = "arn:aws:iam::123456789012:role/"
		roleName = "failing-role"
	)

	callCount := 0
	stsClient := &countingSTSClient{
		delegate: &mockSTSClient{err: errors.New("ThrottlingException")},
		count:    &callCount,
	}

	iamClient := &iam.Client{
		BaseARN:    baseARN,
		Cache:      ccache.New(ccache.Configure()),
		ErrorCache: ccache.New(ccache.Configure()),
		STS:        stsClient,
		Region: &mockRegionClient2{regions: []types.Region{
			{RegionName: aws.String("us-east-1"), Endpoint: aws.String("ec2.us-east-1.amazonaws.com")},
		}},
	}

	roleARN := baseARN + roleName
	const numCalls = 5

	for i := 0; i < numCalls; i++ {
		_, err := iamClient.AssumeRole(roleARN, "", "10.10.0.5", time.Hour, time.Minute)
		if err == nil {
			t.Errorf("call %d: expected error, got nil", i+1)
		}
	}

	// Now that error caching is implemented, STS should only be called once.
	if callCount != 1 {
		t.Errorf("expected 1 STS call (with error caching), got %d", callCount)
	}
}

// TestIntegHealthcheck verifies the doHealthcheck sets instanceID from IMDS
// and clears the fail reason on success.
func TestIntegHealthcheck(t *testing.T) {
	s := NewServer()
	s.iam = &iam.Client{
		IMDS: &integMockIMDS{instanceID: "i-integ-0001"},
	}

	s.doHealthcheck()

	if s.HealthcheckFailReason != "" {
		t.Errorf("expected empty HealthcheckFailReason after success, got %q", s.HealthcheckFailReason)
	}
	if s.InstanceID != "i-integ-0001" {
		t.Errorf("expected InstanceID 'i-integ-0001', got %q", s.InstanceID)
	}
}
