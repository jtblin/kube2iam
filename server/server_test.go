package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"k8s.io/client-go/pkg/api/unversioned"

	v1 "k8s.io/client-go/pkg/api/v1"
)

func withServer(t *testing.T, metadataProtection bool, request *http.Request, callback func(rr *httptest.ResponseRecorder)) {
	kubernetesService := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/namespaces" {
			responseJSON, err := json.Marshal(v1.NamespaceList{
				TypeMeta: unversioned.TypeMeta{
					Kind:       "NamespaceList",
					APIVersion: "v1",
				},
				Items: []v1.Namespace{},
			})
			if err != nil {
				t.Errorf("Failed to marshal json: %v", err)
			}

			_, err = rw.Write(responseJSON)
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		} else if r.URL.Path == "/api/v1/pods" {
			responseJSON, err := json.Marshal(v1.PodList{
				TypeMeta: unversioned.TypeMeta{
					Kind:       "PodList",
					APIVersion: "v1",
				},
				Items: []v1.Pod{
					{
						Status: v1.PodStatus{
							PodIP: "10.0.0.1",
						},
						ObjectMeta: v1.ObjectMeta{
							Annotations: map[string]string{
								"iam.amazonaws.com/role": "my-role",
							},
						},
					},
				},
			})
			if err != nil {
				t.Errorf("Failed to marshal json: %v", err)
			}

			_, err = rw.Write(responseJSON)
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}
	}))
	defer kubernetesService.Close()

	metadataService := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest/meta-data/instance-id" {
			rw.Write([]byte("instanceid"))
		}
	}))
	defer metadataService.Close()

	server := NewServer()
	metadataURL, err := url.Parse(metadataService.URL)
	if err != nil {
		t.Errorf("Failed to parse metadata url: %v", err)
	}
	server.MetadataAddress = metadataURL.Host
	server.MetadataProtection = metadataProtection

	err = server.setup(kubernetesService.URL, "-", "", true)
	if err != nil {
		t.Errorf("Error starting server: %v", err)
	}

	rr := httptest.NewRecorder()
	request.RemoteAddr = "10.0.0.1:80"
	server.server.Handler.ServeHTTP(rr, request)
	callback(rr)
}

func TestMetadataProtection(t *testing.T) {
	t.Run("allows requests with any user-agent when metadata protection is disabled", func(tt *testing.T) {
		request, err := http.NewRequest("GET", "http://127.0.0.1/latest/meta-data/iam/security-credentials", nil)
		if err != nil {
			tt.Errorf("Failed to make request: %v", err)
		}

		withServer(tt, false, request, func(rr *httptest.ResponseRecorder) {
			if rr.Code != 200 {
				tt.Errorf("Expected 200, got %d", rr.Code)
			}

			body, err := ioutil.ReadAll(rr.Body)
			if err != nil {
				tt.Errorf("Error reading body: %v", err)
			}
			if string(body) != "my-role" {
				tt.Errorf("Got unexpected role")
			}
		})
	})

	t.Run("blocks requests with wrong user-agent when metadata protection is enabled", func(tt *testing.T) {
		request, err := http.NewRequest("GET", "http://127.0.0.1/latest/meta-data/iam/security-credentials", nil)
		if err != nil {
			tt.Errorf("Failed to make request: %v", err)
		}

		withServer(tt, true, request, func(rr *httptest.ResponseRecorder) {
			if rr.Code != 403 {
				tt.Errorf("Expected 403, got %d", rr.Code)
			}
		})
	})

	t.Run("allows requests with the correct user-agent when metadata protection is enabled", func(tt *testing.T) {
		request, err := http.NewRequest("GET", "http://127.0.0.1/latest/meta-data/iam/security-credentials", nil)
		request.Header.Set("User-Agent", "aws-cli/1.0")
		if err != nil {
			tt.Errorf("Failed to make request: %v", err)
		}

		withServer(tt, true, request, func(rr *httptest.ResponseRecorder) {
			if rr.Code != 200 {
				tt.Errorf("Expected 200, got %d", rr.Code)
			}

			body, err := ioutil.ReadAll(rr.Body)
			if err != nil {
				tt.Errorf("Error reading body: %v", err)
			}
			if string(body) != "my-role" {
				tt.Errorf("Got unexpected role")
			}
		})
	})

	t.Run("allows healthchecks with wrong user-agent when metadata protection is enabled", func(tt *testing.T) {
		request, err := http.NewRequest("GET", "http://127.0.0.1/healthz", nil)
		if err != nil {
			tt.Errorf("Failed to make request: %v", err)
		}

		withServer(tt, true, request, func(rr *httptest.ResponseRecorder) {
			if rr.Code != 200 {
				tt.Errorf("Expected 200, got %d", rr.Code)
			}

			_, err := ioutil.ReadAll(rr.Body)
			if err != nil {
				tt.Errorf("Error reading body: %v", err)
			}
		})
	})

}
