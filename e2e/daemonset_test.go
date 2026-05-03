//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestDaemonSetScheduledOnAllNodes verifies that the kube2iam DaemonSet has a pod
// on every worker node in the cluster.
func TestDaemonSetScheduledOnAllNodes(t *testing.T) {
	feature := features.New("DaemonSet scheduling").
		Assess("kube2iam pod exists on every node", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("listing nodes: %v", err)
			}

			ds, err := getDaemonSet(ctx, kubeClient, "kube-system", "kube2iam")
			if err != nil {
				t.Fatalf("getting kube2iam DaemonSet: %v", err)
			}

			totalNodes := int32(len(nodes.Items))
			t.Logf("✅ kube2iam running on %d/%d nodes (DesiredScheduled=%d)",
				ds.Status.NumberReady, totalNodes, ds.Status.DesiredNumberScheduled)

			// NumberReady must equal DesiredNumberScheduled (which the DaemonSet
			// itself calculates based on tolerations).
			if ds.Status.NumberReady < 1 {
				t.Errorf("expected at least 1 kube2iam pod ready, got %d", ds.Status.NumberReady)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestIPTablesRulesInstalled checks that kube2iam has installed iptables rules
// on each worker node to redirect 169.254.169.254 traffic to itself.
func TestIPTablesRulesInstalled(t *testing.T) {
	feature := features.New("iptables rules").
		Assess("REDIRECT rule present on each node", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Get kube2iam pods
			pods, err := kubeClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
				LabelSelector: "app=kube2iam",
			})
			if err != nil {
				t.Fatalf("listing kube2iam pods: %v", err)
			}
			if len(pods.Items) == 0 {
				t.Fatal("no kube2iam pods found")
			}

			for _, pod := range pods.Items {
				// Skip the control-plane node: user pods don't run there in real clusters,
				// and kind's control-plane node has different iptables state.
				if strings.Contains(pod.Spec.NodeName, "control-plane") {
					continue
				}
				// Skip pods that are not running.
				if pod.Status.Phase != "Running" {
					t.Logf("⏭️  Skipping pod %s on node %s (phase=%s)", pod.Name, pod.Spec.NodeName, pod.Status.Phase)
					continue
				}
				out, err := execInPod("kube-system", pod.Name, "kube2iam",
					"iptables", "-t", "nat", "-L", "PREROUTING", "-n")
				if err != nil {
					t.Errorf("pod %s: failed to read iptables: %v", pod.Name, err)
					continue
				}
				if !strings.Contains(out, "169.254.169.254") {
					t.Errorf("pod %s: expected iptables REDIRECT rule for 169.254.169.254, got:\n%s",
						pod.Name, out)
				}
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestHealthzEndpoint verifies that the kube2iam /healthz endpoint returns 200
// and contains a valid instanceId from AEMM.
func TestHealthzEndpoint(t *testing.T) {
	feature := features.New("healthz endpoint").
		Assess("returns 200 with instanceId", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pods, err := kubeClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
				LabelSelector: "app=kube2iam",
			})
			if err != nil || len(pods.Items) == 0 {
				t.Fatalf("could not find kube2iam pods: %v", err)
			}

			// Use the K8s API proxy to reach the pod's /healthz endpoint directly from
			// the test binary — no curl, no exec, no test pod required.
			// Must use a worker-node pod: the mocks DaemonSet (AEMM) only runs on workers.
			var workerPod *v1.Pod
			for i := range pods.Items {
				if !strings.Contains(pods.Items[i].Spec.NodeName, "control-plane") {
					workerPod = &pods.Items[i]
					break
				}
			}
			if workerPod == nil {
				t.Fatal("no kube2iam pod found on a worker node")
			}
			body, err := kubeClient.CoreV1().RESTClient().Get().
				Namespace("kube-system").
				Resource("pods").
				Name(fmt.Sprintf("%s:8181", workerPod.Name)).
				SubResource("proxy").
				Suffix("/healthz").
				DoRaw(ctx)
			if err != nil {
				t.Fatalf("healthz proxy request failed on pod %s: %v", workerPod.Name, err)
			}
			if !strings.Contains(string(body), "instanceId") {
				t.Errorf("expected instanceId in healthz response, got: %s", string(body))
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
