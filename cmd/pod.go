package cmd

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
)

type podHandler struct {
	storage *store
}

// OnAdd is called when a pod is added.
func (p *podHandler) OnAdd(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		log.Errorf("Expected Pod but OnAdd handler received %+v", obj)
		return
	}
	log.Debugf("Pod OnAdd %s - %s", pod.GetName(), pod.Status.PodIP)

	p.storage.AddNamespaceToIP(pod)

	if pod.Status.PodIP != "" {
		if role, ok := pod.Annotations[p.storage.iamRoleKey]; ok {
			log.Debugf("- Role %s", role)
			p.storage.AddRoleToIP(pod, role)
		}
	}
}

// OnUpdate is called when a pod is modified.
func (p *podHandler) OnUpdate(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*api.Pod)
	newPod, ok2 := newObj.(*api.Pod)
	if !ok1 || !ok2 {
		log.Errorf("Expected Pod but OnUpdate handler received %+v %+v", oldObj, newObj)
		return
	}
	log.Debugf("Pod OnUpdate %s - %s", newPod.GetName(), newPod.Status.PodIP)

	if oldPod.Status.PodIP != newPod.Status.PodIP {
		p.OnDelete(oldPod)
		p.OnAdd(newPod)
	}
}

// OnDelete is called when a pod is deleted.
func (p *podHandler) OnDelete(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		deletedObj, dok := obj.(kcache.DeletedFinalStateUnknown)
		if dok {
			pod, ok = deletedObj.Obj.(*api.Pod)
		}
	}

	if !ok {
		log.Errorf("Expected Pod but OnDelete handler received %+v", obj)
		return
	}

	log.Debugf("Pod OnDelete %s - %s", pod.GetName(), pod.Status.PodIP)

	if pod.Status.PodIP != "" {
		p.storage.DeleteIP(pod.Status.PodIP)
	}
}

func newPodHandler(s *store) *podHandler {
	return &podHandler{
		storage: s,
	}
}
