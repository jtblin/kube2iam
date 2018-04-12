package k8s

import (
	"fmt"
	"time"

	"github.com/jtblin/kube2iam"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	selector "k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	podIPIndexName     = "byPodIP"
	namespaceIndexName = "byName"
	// Resync period for the kube controller loop.
	resyncPeriod = 30 * time.Minute
)

// Client represents a kubernetes client.
type Client struct {
	*kubernetes.Clientset
	namespaceController *cache.Controller
	namespaceIndexer    cache.Indexer
	podController       *cache.Controller
	podIndexer          cache.Indexer
	nodeName            string
}

// Returns a cache.ListWatch that gets all changes to pods.
func (k8s *Client) createPodLW() *cache.ListWatch {
	fieldSelector := selector.Everything()
	if k8s.nodeName != "" {
		fieldSelector = selector.OneTermEqualSelector("spec.nodeName", k8s.nodeName)
	}
	return cache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "pods", v1.NamespaceAll, fieldSelector)
}

// WatchForPods watches for pod changes.
func (k8s *Client) WatchForPods(podEventLogger cache.ResourceEventHandler) cache.InformerSynced {
	k8s.podIndexer, k8s.podController = cache.NewIndexerInformer(
		k8s.createPodLW(),
		&v1.Pod{},
		resyncPeriod,
		podEventLogger,
		cache.Indexers{podIPIndexName: kube2iam.PodIPIndexFunc},
	)
	go k8s.podController.Run(wait.NeverStop)
	return k8s.podController.HasSynced
}

// returns a cache.ListWatch of namespaces.
func (k8s *Client) createNamespaceLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "namespaces", v1.NamespaceAll, selector.Everything())
}

// WatchForNamespaces watches for namespaces changes.
func (k8s *Client) WatchForNamespaces(nsEventLogger cache.ResourceEventHandler) cache.InformerSynced {
	k8s.namespaceIndexer, k8s.namespaceController = cache.NewIndexerInformer(
		k8s.createNamespaceLW(),
		&v1.Namespace{},
		resyncPeriod,
		nsEventLogger,
		cache.Indexers{namespaceIndexName: kube2iam.NamespaceIndexFunc},
	)
	go k8s.namespaceController.Run(wait.NeverStop)
	return k8s.namespaceController.HasSynced
}

// ListPodIPs returns the underlying set of pods being managed/indexed
func (k8s *Client) ListPodIPs() []string {
	// Decided to simply dump this and leave it up to consumer
	// as k8s package currently doesn't need to be concerned about what's
	// a signficant annotation to process, that is left up to store/server
	return k8s.podIndexer.ListIndexFuncValues(podIPIndexName)
}

// ListNamespaces returns the underlying set of namespaces being managed/indexed
func (k8s *Client) ListNamespaces() []string {
	return k8s.namespaceIndexer.ListIndexFuncValues(namespaceIndexName)
}

// PodByIP provides the representation of the pod itself being cached keyed off of it's IP
// Returns an error if there are multiple pods attempting to be keyed off of the same IP
// (Which happens when they of type `hostNetwork: true`)
func (k8s *Client) PodByIP(IP string) (*v1.Pod, error) {
	pods, err := k8s.podIndexer.ByIndex(podIPIndexName, IP)
	if err != nil {
		return nil, err
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("Pod with specificed IP not found")
	}

	if len(pods) == 1 {
		return pods[0].(*v1.Pod), nil
	}

	//This happens with `hostNetwork: true` pods
	podNames := make([]string, len(pods))
	for i, pod := range pods {
		podNames[i] = pod.(*v1.Pod).ObjectMeta.Name
	}
	return nil, fmt.Errorf("%d pods (%v) with the ip %s indexed", len(pods), podNames, IP)
}

// NamespaceByName retrieves a namespace by it's given name.
// Returns an error if there are no namespaces available
func (k8s *Client) NamespaceByName(namespaceName string) (*v1.Namespace, error) {
	namespace, err := k8s.namespaceIndexer.ByIndex(namespaceIndexName, namespaceName)
	if err != nil {
		return nil, err
	}

	if len(namespace) == 0 {
		return nil, fmt.Errorf("Namespace was not found")
	}

	return namespace[0].(*v1.Namespace), nil
}

// NewClient returns a new kubernetes client.
func NewClient(host, token, nodeName string, insecure bool) (*Client, error) {
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
	return &Client{Clientset: client, nodeName: nodeName}, nil
}
