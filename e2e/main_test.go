//go:build e2e

// Package e2e contains end-to-end tests for kube2iam.
//
// Prerequisites:
//   - kind installed (https://kind.sigs.k8s.io)
//   - kubectl installed
//   - Docker running (kind uses Docker as the container runtime)
//   - kube2iam Docker image built as "kube2iam:e2e-test"
//
// Running locally:
//
//	make build-e2e-image     # build Docker image tagged kube2iam:e2e-test
//	go test -v -tags=e2e ./e2e/... -timeout=15m
//
// The test suite will:
//  1. Create a kind cluster (multi-node, from e2e/kind-config.yaml)
//  2. Load the kube2iam image into kind
//  3. Deploy AEMM (AWS EC2 Metadata Mock)
//  4. Deploy kube2iam as a DaemonSet
//  5. Run all E2E test scenarios
//  6. Destroy the cluster on teardown
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

const (
	clusterName    = "kube2iam-e2e"
	e2eNamespace   = "kube2iam-e2e"
	kube2iamImage  = "kube2iam:e2e-test"
	testPodImage   = "kube2iam-e2e-client:latest" // minimal Alpine+curl, built by make build-e2e-client
	kindConfigPath = "kind-config.yaml"

	// How long to wait for DaemonSet rollout / pod readiness.
	daemonSetReadyTimeout = 3 * time.Minute
	podReadyTimeout       = 2 * time.Minute
)

var (
	testenv    env.Environment
	kubeClient kubernetes.Interface
)

func TestMain(m *testing.M) {
	testenv = env.New()

	testenv.Setup(
		// 1. Create the kind cluster.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf(">>> E2E: Starting kind cluster creation (%s) using %s...\n", clusterName, kindConfigPath)
			ctx, err := envfuncs.CreateKindClusterWithConfig(clusterName, "kindest/node:v1.35.1", kindConfigPath)(ctx, cfg)
			if err != nil {
				fmt.Printf(">>> E2E: Kind cluster creation failed: %v\n", err)
				return ctx, err
			}
			fmt.Println(">>> E2E: Kind cluster created successfully")
			return ctx, nil
		},

		// 2. Load the kube2iam Docker image into kind (avoids registry push).
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf(">>> E2E: Loading image %s into kind cluster...\n", kube2iamImage)
			err := loadImageIntoKind(clusterName, kube2iamImage)
			if err != nil {
				fmt.Printf(">>> E2E: Image load failed: %v\n", err)
				return ctx, err
			}
			fmt.Println(">>> E2E: Image loaded successfully")
			return ctx, nil
		},

		// 2b. Load the E2E test client image (Alpine+curl) into kind.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf(">>> E2E: Loading test client image %s into kind cluster...\n", testPodImage)
			if err := loadImageIntoKind(clusterName, testPodImage); err != nil {
				fmt.Printf(">>> E2E: Test client image load failed: %v\n", err)
				return ctx, err
			}
			fmt.Println(">>> E2E: Test client image loaded successfully")
			return ctx, nil
		},

		// 3. Create the test namespace.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf(">>> E2E: Creating namespace %s...\n", e2eNamespace)
			ctx, err := envfuncs.CreateNamespace(e2eNamespace)(ctx, cfg)
			if err != nil {
				fmt.Printf(">>> E2E: Namespace creation failed: %v\n", err)
				return ctx, err
			}
			fmt.Println(">>> E2E: Namespace created")
			return ctx, nil
		},

		// 4. Deploy Unified Mocks (AEMM + STS/EC2 Sidecar).
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println(">>> E2E: Deploying kube2iam-mocks...")
			if err := kubectlApply("testdata/kube2iam-mocks.yaml"); err != nil {
				return ctx, fmt.Errorf("kube2iam-mocks deployment failed: %w", err)
			}

			client, err := buildKubeClient(cfg)
			if err != nil {
				return ctx, err
			}
			fmt.Println(">>> E2E: Waiting for kube2iam-mocks readiness...")
			if err := waitForDaemonSet(ctx, client, e2eNamespace, "kube2iam-mocks", 5*time.Minute); err != nil {
				dumpDebugInfo(ctx, client)
				return ctx, fmt.Errorf("kube2iam-mocks not ready: %w", err)
			}
			return ctx, nil
		},

		// 5. Deploy kube2iam DaemonSet.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println(">>> E2E: Deploying kube2iam DaemonSet...")
			err := kubectlApply("testdata/kube2iam-ds.yaml")
			if err != nil {
				fmt.Printf(">>> E2E: kube2iam deployment failed: %v\n", err)
				return ctx, err
			}
			fmt.Println(">>> E2E: kube2iam deployed")
			return ctx, nil
		},

		// 6. Wait for kube2iam DaemonSet to be fully ready on all nodes.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println(">>> E2E: Waiting for kube2iam DaemonSet rollout...")
			client, err := buildKubeClient(cfg)
			if err != nil {
				return ctx, fmt.Errorf("building kube client: %w", err)
			}
			kubeClient = client
			err = waitForDaemonSet(ctx, client, "kube-system", "kube2iam", daemonSetReadyTimeout)
			if err != nil {
				fmt.Printf(">>> E2E: DaemonSet rollout failed or timed out: %v\n", err)
				dumpDebugInfo(ctx, client)
				return ctx, err
			}
			fmt.Println(">>> E2E: kube2iam DaemonSet is ready on all nodes")
			return ctx, nil
		},

		// 7. Warmup: create a canary pod to verify the cluster can schedule pods
		// and the image is fully usable before actual tests start.
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println(">>> E2E: Running canary pod warmup...")
			canary := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "e2e-canary",
					Namespace: e2eNamespace,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:            "canary",
						Image:           testPodImage,
						ImagePullPolicy: v1.PullNever,
						Command:         []string{"sleep", "3600"},
					}},
					RestartPolicy: v1.RestartPolicyNever,
				},
			}
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, canary.Name, metav1.DeleteOptions{})
			if _, err := kubeClient.CoreV1().Pods(e2eNamespace).Create(ctx, canary, metav1.CreateOptions{}); err != nil {
				return ctx, fmt.Errorf("canary pod creation failed: %w", err)
			}
			if err := waitForPodRunning(ctx, kubeClient, e2eNamespace, canary.Name, 5*time.Minute); err != nil {
				dumpDebugInfo(ctx, kubeClient)
				return ctx, fmt.Errorf("canary pod never became ready: %w", err)
			}
			_ = kubeClient.CoreV1().Pods(e2eNamespace).Delete(ctx, canary.Name, metav1.DeleteOptions{})
			fmt.Println(">>> E2E: Canary pod ready — cluster is warm")
			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Printf(">>> E2E: Destroying kind cluster %s...\n", clusterName)
			return envfuncs.DestroyKindCluster(clusterName)(ctx, cfg)
		},
	)

	fmt.Println(">>> E2E: Starting test execution...")
	os.Exit(testenv.Run(m))
}

// ---- Helpers ----------------------------------------------------------------

func loadImageIntoKind(cluster, image string) error {
	cmd := exec.Command("kind", "load", "docker-image", image, "--name", cluster)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func kubectlApply(path string) error {
	cmd := exec.Command("kubectl", "apply", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildKubeClient(cfg *envconf.Config) (kubernetes.Interface, error) {
	kubeconfig := cfg.KubeconfigFile()
	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}

// waitForDaemonSet polls until the DaemonSet has NumberReady == DesiredNumberScheduled.
func waitForDaemonSet(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf(">>> E2E: Waiting for DaemonSet %s/%s to exist...\n", namespace, name)
			return false, nil // not found yet, keep polling
		}
		fmt.Printf(">>> E2E: DaemonSet %s/%s status: %d/%d ready\n", namespace, name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		ready := ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0
		return ready, nil
	})
}

// waitForDeployment polls until the deployment has all replicas ready.
func waitForDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return deploy.Status.ReadyReplicas >= *deploy.Spec.Replicas, nil
	})
}

// waitForPodRunning polls until the named pod is Running and Ready.
func waitForPodRunning(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pod.Status.Phase != v1.PodRunning {
			return false, nil
		}
		for _, c := range pod.Status.ContainerStatuses {
			if !c.Ready {
				return false, nil
			}
		}
		return true, nil
	})
}

// waitForDaemonSetReady returns the DaemonSet once it reaches desired state.
func getDaemonSet(ctx context.Context, client kubernetes.Interface, namespace, name string) (*appsv1.DaemonSet, error) {
	return client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// execInPod runs a command inside a pod and returns stdout.
func execInPod(namespace, podName, container string, cmd ...string) (string, error) {
	args := append([]string{
		"exec", "-n", namespace, podName, "-c", container, "--",
	}, cmd...)
	out, err := exec.Command("kubectl", args...).CombinedOutput()
	return string(out), err
}
func dumpDebugInfo(ctx context.Context, client kubernetes.Interface) {
	namespaces := []string{"kube-system", e2eNamespace}
	for _, ns := range namespaces {
		fmt.Printf(">>> E2E DEBUG: Dumping %s pod status...\n", ns)
		pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("Failed to list pods in %s: %v\n", ns, err)
			continue
		}
		for _, p := range pods.Items {
			fmt.Printf("Pod: %s | Phase: %s | Reason: %s | Message: %s\n", p.Name, p.Status.Phase, p.Status.Reason, p.Status.Message)
			var allContainers []v1.ContainerStatus
			allContainers = append(allContainers, p.Status.InitContainerStatuses...)
			allContainers = append(allContainers, p.Status.ContainerStatuses...)

			for _, cs := range allContainers {
				fmt.Printf("  Container: %s | Ready: %v | State: %+v\n", cs.Name, cs.Ready, cs.State)
				fmt.Printf("--- Logs for container %s in pod %s ---\n", cs.Name, p.Name)
				req := client.CoreV1().Pods(ns).GetLogs(p.Name, &v1.PodLogOptions{Container: cs.Name})
				logs, err := req.DoRaw(ctx)
				if err != nil {
					fmt.Printf("Failed to get logs: %v\n", err)
				} else {
					fmt.Println(string(logs))
				}
			}
			fmt.Println("-----------------------")
		}

		fmt.Printf(">>> E2E DEBUG: Dumping %s events...\n", ns)
		events, err := client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, e := range events.Items {
				fmt.Printf("Event: %s | Reason: %s | Message: %s\n", e.LastTimestamp, e.Reason, e.Message)
			}
		}
	}
}
