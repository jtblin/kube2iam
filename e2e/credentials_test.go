//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	metadataEndpoint = "http://169.254.169.254/latest/meta-data/iam/security-credentials/"
	testRole         = "arn:aws:iam::123456789012:role/test-role"
)

// dumpKube2iamLogs captures logs from the kube2iam pod on the same node as the given pod.
func dumpKube2iamLogs(ctx context.Context, client kubernetes.Interface, namespace, podName string) string {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("failed to get pod %s: %v", podName, err)
	}

	k2iPods, err := client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{LabelSelector: "app=kube2iam"})
	if err != nil {
		return fmt.Sprintf("failed to list kube2iam pods: %v", err)
	}

	for _, k2iPod := range k2iPods.Items {
		if k2iPod.Spec.NodeName == pod.Spec.NodeName {
			req := client.CoreV1().Pods("kube-system").GetLogs(k2iPod.Name, &v1.PodLogOptions{})
			logs, err := req.DoRaw(ctx)
			if err != nil {
				return fmt.Sprintf("failed to get logs for %s: %v", k2iPod.Name, err)
			}
			return fmt.Sprintf("--- kube2iam logs for node %s ---\n%s\n-------------------", k2iPod.Spec.NodeName, string(logs))
		}
	}
	return "no kube2iam pod found on node " + pod.Spec.NodeName
}

// TestAnnotatedPodGetsCreds verifies that a pod with a valid IAM role annotation
// can successfully retrieve credentials from the metadata endpoint.
func TestAnnotatedPodGetsCreds(t *testing.T) {
	feature := features.New("annotated_pod_credential_retrieval").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "annotated-pod",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": testRole,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "tester",
							Image:           testPodImage,
							ImagePullPolicy: v1.PullNever,
							Command:         []string{"sleep", "3600"},
						},
					},
				},
			}
			client, err := buildKubeClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.CoreV1().Pods(e2eNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err == nil {
				time.Sleep(2 * time.Second)
			}
			if _, err := client.CoreV1().Pods(e2eNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := waitForPodRunning(ctx, client, e2eNamespace, pod.Name, 2*time.Minute); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("GET /security-credentials returns role name", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			out, err := execInPod(e2eNamespace, "annotated-pod", "tester", "curl", "-si", metadataEndpoint)
			if err != nil {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "annotated-pod"))
				t.Fatalf("failed to get security credentials list: %v\nOutput: %s", err, out)
			}
			if !strings.Contains(out, "test-role") {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "annotated-pod"))
				t.Errorf("expected role name test-role in response, got: %s", out)
			}
			return ctx
		}).
		Assess("GET /security-credentials/<role> returns credentials JSON", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			url := metadataEndpoint + "test-role"
			out, err := execInPod(e2eNamespace, "annotated-pod", "tester", "curl", "-s", url)
			if err != nil {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "annotated-pod"))
				t.Fatalf("failed to get credentials: %v\nOutput: %s", err, out)
			}

			var creds map[string]interface{}
			err = json.Unmarshal([]byte(out), &creds)
			if err != nil {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "annotated-pod"))
				t.Fatalf("response is not valid JSON: %v\nBody: %s", err, out)
			}

			if _, ok := creds["AccessKeyId"]; !ok {
				t.Errorf("expected AccessKeyId in credentials, got: %s", out)
			}
			if creds["Code"] != "Success" {
				t.Errorf("expected Code=Success, got: %v", creds["Code"])
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, "annotated-pod", metav1.DeleteOptions{})
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestUnannotatedPodNoCredentials verifies that a pod WITHOUT an IAM role annotation
// cannot obtain credentials. kube2iam returns HTTP 200 with an empty body (no role
// name listed) rather than 404. This is because --base-role-arn is always set, and
// RoleARN("") resolves to just the base ARN prefix — which is non-empty, so the
// 404 code path in extractRoleARN is not reached. The security property still holds:
// an empty role list means no credentials can be fetched.
func TestUnannotatedPodNoCredentials(t *testing.T) {
	feature := features.New("unannotated_pod_denial").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unannotated-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "tester",
							Image:           testPodImage,
							ImagePullPolicy: v1.PullNever,
							Command:         []string{"sleep", "3600"},
						},
					},
				},
			}
			client, err := buildKubeClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.CoreV1().Pods(e2eNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err == nil {
				time.Sleep(2 * time.Second)
			}
			if _, err := client.CoreV1().Pods(e2eNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := waitForPodRunning(ctx, client, e2eNamespace, pod.Name, 2*time.Minute); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("GET /security-credentials returns no role for unannotated pod", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			out, err := execInPod(e2eNamespace, "unannotated-pod", "tester", "curl", "-si", metadataEndpoint)
			if err != nil {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "unannotated-pod"))
				t.Fatalf("unexpected error: %v\nOutput: %s", err, out)
			}
			// kube2iam returns 200 with empty body for unannotated pods (fallback path).
			// The security guarantee is that no role name is advertised, so no credentials
			// can be obtained.
			if !strings.Contains(out, "200 OK") {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "unannotated-pod"))
				t.Errorf("expected 200 for unannotated pod, got response:\n%s", out)
				return ctx
			}
			// Extract body (after blank line separating headers from body).
			parts := strings.SplitN(out, "\r\n\r\n", 2)
			body := ""
			if len(parts) == 2 {
				body = strings.TrimSpace(parts[1])
			}
			if body != "" {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "unannotated-pod"))
				t.Errorf("expected empty body for unannotated pod (no role should be listed), got: %s", body)
				return ctx
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, "unannotated-pod", metav1.DeleteOptions{})
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestWrongRoleRejected verifies that a pod requesting a role different from its annotation
// receives a 403 Forbidden response.
func TestWrongRoleRejected(t *testing.T) {
	feature := features.New("wrong_role_rejection").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-role-pod",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": testRole,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "tester",
							Image:           testPodImage,
							ImagePullPolicy: v1.PullNever,
							Command:         []string{"sleep", "3600"},
						},
					},
				},
			}
			client, err := buildKubeClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.CoreV1().Pods(e2eNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err == nil {
				time.Sleep(2 * time.Second)
			}
			if _, err := client.CoreV1().Pods(e2eNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := waitForPodRunning(ctx, client, e2eNamespace, pod.Name, 2*time.Minute); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("GET /security-credentials/different-role returns 403", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			url := metadataEndpoint + "different-role"
			out, err := execInPod(e2eNamespace, "wrong-role-pod", "tester", "curl", "-si", url)
			if err != nil && !strings.Contains(out, "403") {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "wrong-role-pod"))
				t.Fatalf("unexpected error: %v\nOutput: %s", err, out)
			}
			if !strings.Contains(out, "403 Forbidden") {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "wrong-role-pod"))
				t.Errorf("expected 403 for wrong role, got response: \n%s", out)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, "wrong-role-pod", metav1.DeleteOptions{})
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestIMDSv2Passthrough verifies that IMDSv2 token requests are correctly passed through
// or handled by kube2iam.
func TestIMDSv2Passthrough(t *testing.T) {
	feature := features.New("IMDSv2_passthrough").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "imds-v2-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "tester",
							Image:           testPodImage,
							ImagePullPolicy: v1.PullNever,
							Command:         []string{"sleep", "3600"},
						},
					},
				},
			}
			client, err := buildKubeClient(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.CoreV1().Pods(e2eNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err == nil {
				time.Sleep(2 * time.Second)
			}
			if _, err := client.CoreV1().Pods(e2eNamespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := waitForPodRunning(ctx, client, e2eNamespace, pod.Name, 2*time.Minute); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("PUT /latest/api/token returns IMDSv2 token", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			url := "http://169.254.169.254/latest/api/token"
			out, err := execInPod(e2eNamespace, "imds-v2-pod", "tester", "curl", "-X", "PUT", "-H", "X-aws-ec2-metadata-token-ttl-seconds: 21600", "-s", url)
			if err != nil {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "imds-v2-pod"))
				t.Fatalf("IMDSv2 token request failed: %v\nOutput: %s", err, out)
			}
			if len(out) < 32 {
				t.Log(dumpKube2iamLogs(ctx, kubeClient, e2eNamespace, "imds-v2-pod"))
				t.Errorf("expected IMDSv2 token, got: %s", out)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, "imds-v2-pod", metav1.DeleteOptions{})
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
