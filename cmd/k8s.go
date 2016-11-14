package cmd

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	selector "k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod   = 30 * time.Minute
	podIPIndexName = "byPodIP"
)

var errPodNotFound = errors.New("Pod with specified IP not found")

type k8s struct {
	*client.Client
	podStore      kcache.Indexer
	podController *kcache.Controller
}

func podIPIndexFunc(obj interface{}) ([]string, error) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return nil, fmt.Errorf("obj not pod: %+v", obj)
	}
	if pod.Status.PodIP != "" && controller.IsPodActive(pod) {
		return []string{pod.Status.PodIP}, nil
	}
	return nil, nil
}

// Returns a cache.ListWatch that gets all changes to pods.
func (k8s *k8s) createPodLW() *kcache.ListWatch {
	return kcache.NewListWatchFromClient(k8s, "pods", api.NamespaceAll, selector.Everything())
}

func (k8s *k8s) PodByIP(IP string) (*api.Pod, error) {
	pods, err := k8s.podStore.ByIndex(podIPIndexName, IP)
	if err != nil {
		return nil, err
	}

	if len(pods) == 0 {
		return nil, errPodNotFound
	}
	if len(pods) == 1 {
		return pods[0].(*api.Pod), nil
	}
	podNames := make([]string, len(pods))
	for i, pod := range pods {
		podNames[i] = pod.(*api.Pod).ObjectMeta.Name
	}
	return nil, fmt.Errorf("%d pods (%v) with the ip %s indexed", len(pods), podNames, IP)
}

func (k8s *k8s) Run() {
	k8s.podController.Run(wait.NeverStop)
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
	k8s := &k8s{c, nil, nil}
	k8s.podStore, k8s.podController = kcache.NewIndexerInformer(
		k8s.createPodLW(),
		&api.Pod{},
		resyncPeriod,
		kcache.ResourceEventHandlerFuncs{},
		kcache.Indexers{podIPIndexName: podIPIndexFunc},
	)
	return k8s, nil
}
