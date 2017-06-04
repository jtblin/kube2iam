package k8s

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	selector "k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 30 * time.Minute
)

// Client represents a kubernetes client.
type Client struct {
	*kubernetes.Clientset
}

// Returns a cache.ListWatch that gets all changes to pods.
func (k8s *Client) createPodLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "pods", v1.NamespaceAll, selector.Everything())
}

// WatchForPods watches for pod changes.
func (k8s *Client) WatchForPods(podManager cache.ResourceEventHandler) cache.Store {
	podStore, podController := cache.NewInformer(
		k8s.createPodLW(),
		&v1.Pod{},
		resyncPeriod,
		podManager,
	)
	go podController.Run(wait.NeverStop)
	return podStore
}

// returns a cache.ListWatch of namespaces.
func (k8s *Client) createNamespaceLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "namespaces", v1.NamespaceAll, selector.Everything())
}

// WatchForNamespaces watches for namespaces changes.
func (k8s *Client) WatchForNamespaces(nsManager cache.ResourceEventHandler) cache.Store {
	nsStore, nsController := cache.NewInformer(
		k8s.createNamespaceLW(),
		&v1.Namespace{},
		resyncPeriod,
		nsManager,
	)
	go nsController.Run(wait.NeverStop)
	return nsStore
}

// NewClient returns a new kubernetes client.
func NewClient(host, token string, insecure bool) (*Client, error) {
	var config *rest.Config
	var err error
	if host != "" && token != "" {
		config = &rest.Config{
			Host:        host,
			BearerToken: token,
			Insecure:    insecure,
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{client}, nil
}
