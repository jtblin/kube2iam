package cmd

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	selector "k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 30 * time.Minute
)

type k8s struct {
	*client.Client
}

// Returns a cache.ListWatch that gets all changes to pods.
func (k8s *k8s) createPodLW() *kcache.ListWatch {
	return kcache.NewListWatchFromClient(k8s, "pods", api.NamespaceAll, selector.Everything())
}

func (k8s *k8s) watchForPods(podManager kcache.ResourceEventHandler) kcache.Store {
	podStore, podController := kcache.NewInformer(
		k8s.createPodLW(),
		&api.Pod{},
		resyncPeriod,
		kcache.ResourceEventHandlerFuncs{
			AddFunc:    podManager.OnAdd,
			DeleteFunc: podManager.OnDelete,
			UpdateFunc: podManager.OnUpdate,
		},
	)
	go podController.Run(wait.NeverStop)
	return podStore
}

func newK8s(host, token string, insecure bool) (*k8s, error) {
	var c *client.Client
	var err error
	if host != "" && token != "" {
		config := restclient.Config{
			Host:        host,
			BearerToken: token,
			Insecure:    insecure,
		}
		c, err = client.New(&config)
	} else {
		c, err = client.NewInCluster()
	}
	if err != nil {
		return nil, err
	}
	return &k8s{c}, nil
}
