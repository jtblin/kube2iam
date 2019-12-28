package k8s

import (
	"fmt"
	"time"

	"github.com/jtblin/kube2iam"
	"github.com/jtblin/kube2iam/metrics"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/pkg/api/v1"
	selector "k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	podIPIndexName     = "byPodIP"
	namespaceIndexName = "byName"
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
func (k8s *Client) WatchForPods(podEventLogger cache.ResourceEventHandler, resyncPeriod time.Duration) cache.InformerSynced {
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
func (k8s *Client) WatchForNamespaces(nsEventLogger cache.ResourceEventHandler, resyncPeriod time.Duration) cache.InformerSynced {
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
func (k8s *Client) PodByIP(IP string, dealWithDupIP bool) (*v1.Pod, error) {
	pods, err := k8s.podIndexer.ByIndex(podIPIndexName, IP)
	if err != nil {
		return nil, err
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("pod with specificed IP not found")
	}

	if len(pods) == 1 {
		return pods[0].(*v1.Pod), nil
	}

	if !dealWithDupIP {
		podNames := make([]string, len(pods))
		for i, pod := range pods {
			podNames[i] = pod.(*v1.Pod).ObjectMeta.Name
		}
		return nil, fmt.Errorf("%d pods (%v) with the ip %s indexed", len(pods), podNames, IP)
	}
	pod, err := DealWithDuplicatedIP(pods, k8s)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// DealWithDuplicatedIP queries the k8s api server trying to make a decision based on NON cached data
// If the indexed pods all have HostNetwork = true the function return nil and the error message.
// If there are pods in the cache that do not have HostNetwork = true will query the k8s api server
// searching for the ONLY pod in Running state among all the cached ones, if it doesn't find it returns error.
func DealWithDuplicatedIP(pods []interface{}, k8s *Client) (*v1.Pod, error) {
	podNames := make([]string, len(pods))
	podNamespaces := make([]string, len(pods))
	error := fmt.Errorf("more than a pod with the same IP has been indexed, this can happen when pods have hostNetwork: true")
	for i, pod := range pods {
		if pod.(*v1.Pod).Spec.HostNetwork {
			continue
		}
		podNames[i] = pod.(*v1.Pod).ObjectMeta.Name
		podNamespaces[i] = pod.(*v1.Pod).ObjectMeta.Namespace
	}
	// if len(podNames) > 0 it means that I have more than a pod with the same IP in the cache with HostNetwork = false
	// in this case we will investigate querying the k8s API server
	// note: we could check also here if len(podNames) is 1 and just return but I can't find the case for that condition to happen here.
	if len(podNames) > 0 {
		livePods := make([]*v1.Pod, len(podNames))
		for i := 0; i < len(podNames); i++ {
			livePod, err := k8s.CoreV1().Pods(podNamespaces[i]).Get(podNames[i])
			metrics.K8sAPIDupReqCount.Inc()
			if err != nil {
				//pod could not exist, error is expected here
				continue
			}
			if "Running" == string(livePod.Status.Phase) {
				livePods[i] = livePod
			}
		}
		//At this point len(livePods) has to be exaclty 1, we can't have multiple running pods with the same IP that are running
		//with hostNetwork: false
		if len(livePods) == 1 {
			return livePods[0], nil
		}
		return nil, error
	}
	return nil, error
}

// NamespaceByName retrieves a namespace by it's given name.
// Returns an error if there are no namespaces available
func (k8s *Client) NamespaceByName(namespaceName string) (*v1.Namespace, error) {
	namespace, err := k8s.namespaceIndexer.ByIndex(namespaceIndexName, namespaceName)
	if err != nil {
		return nil, err
	}

	if len(namespace) == 0 {
		return nil, fmt.Errorf("namespace was not found")
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
