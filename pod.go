package kube2iam

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/jtblin/kube2iam/store"
)

// PodHandler represents a pod handler.
type PodHandler struct {
	storage *store.Store
}

// OnAdd is called when a pod is added.
func (p *PodHandler) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Errorf("Expected Pod but OnAdd handler received %+v", obj)
		return
	}
	log.Debugf("Pod OnAdd %s - %s", pod.GetName(), pod.Status.PodIP)

	p.storage.AddNamespaceToIP(pod)

	if pod.Status.PodIP != "" {
		if role, ok := pod.Annotations[p.storage.IamRoleKey]; ok {
			log.Debugf("- Role %s", role)
			p.storage.AddRoleToIP(pod, role)
		}
	}
}

// OnUpdate is called when a pod is modified.
func (p *PodHandler) OnUpdate(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*v1.Pod)
	newPod, ok2 := newObj.(*v1.Pod)
	if !ok1 || !ok2 {
		log.Errorf("Expected Pod but OnUpdate handler received %+v %+v", oldObj, newObj)
		return
	}
	log.Debugf("Pod OnUpdate %s - %s", newPod.GetName(), newPod.Status.PodIP)

	if oldPod.Status.PodIP != newPod.Status.PodIP {
		p.OnDelete(oldPod)
		p.OnAdd(newPod)
		return
	}

	if annotationDiffers(oldPod.Annotations, newPod.Annotations, p.storage.IamRoleKey) {
		log.Debugf("Updating pod %s due to added/updated annotation value", newPod.GetName())
		p.OnDelete(oldPod)
		p.OnAdd(newPod)
		return
	}
}

// OnDelete is called when a pod is deleted.
func (p *PodHandler) OnDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		deletedObj, dok := obj.(cache.DeletedFinalStateUnknown)
		if dok {
			pod, ok = deletedObj.Obj.(*v1.Pod)
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

func annotationDiffers(oldAnnotations, newAnnotations map[string]string, annotationName string) bool {
	oldValue, oldPresent := oldAnnotations[annotationName]
	newValue, newPresent := newAnnotations[annotationName]
	if oldPresent != newPresent || oldValue != newValue {
		return true
	}
	return false
}

// NewPodHandler returns a new pod handler.
func NewPodHandler(s *store.Store) *PodHandler {
	return &PodHandler{
		storage: s,
	}
}
