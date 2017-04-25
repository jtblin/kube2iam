package cmd

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	selector "k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 30 * time.Minute
)

type k8s struct {
	*kubernetes.Clientset
}

// Returns a cache.ListWatch that gets all changes to pods.
func (k8s *k8s) createPodLW() *kcache.ListWatch {
	return kcache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "pods", v1.NamespaceAll, selector.Everything())
}

func (k8s *k8s) watchForPods(podManager kcache.ResourceEventHandler) kcache.Store {
	podStore, podController := kcache.NewInformer(
		k8s.createPodLW(),
		&v1.Pod{},
		resyncPeriod,
		podManager,
	)
	go podController.Run(wait.NeverStop)
	return podStore
}

// returns a listwatcher of namespaces
func (k8s *k8s) createNamespaceLW() *kcache.ListWatch {
	return kcache.NewListWatchFromClient(k8s.CoreV1().RESTClient(), "namespaces", v1.NamespaceAll, selector.Everything())
}

func (k8s *k8s) watchForNamespaces(nsManager kcache.ResourceEventHandler) kcache.Store {
	nsStore, nsController := kcache.NewInformer(
		k8s.createNamespaceLW(),
		&v1.Namespace{},
		resyncPeriod,
		nsManager,
	)
	go nsController.Run(wait.NeverStop)
	return nsStore
}

func newK8s(host, token string, insecure bool) (*k8s, error) {
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
	if err != nil {
		return nil, err
	}
	return &k8s{client}, nil
}
