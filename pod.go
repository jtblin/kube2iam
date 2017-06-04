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

func (p *PodHandler) podFields(pod *v1.Pod) log.Fields {
	return log.Fields{
		"pod.name":      pod.GetName(),
		"pod.namespace": pod.GetNamespace(),
		"pod.status.ip": pod.Status.PodIP,
		"pod.iam.role":  pod.GetAnnotations()[p.storage.IamRoleKey],
	}
}

// OnAdd is called when a pod is added.
func (p *PodHandler) OnAdd(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Errorf("Expected Pod but OnAdd handler received %+v", obj)
		return
	}
	log.WithFields(p.podFields(pod)).Debug("Pod OnAdd")

	p.storage.AddNamespaceToIP(pod)

	if pod.Status.PodIP != "" {
		if role, ok := pod.GetAnnotations()[p.storage.IamRoleKey]; ok {
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
	logger := log.WithFields(p.podFields(newPod))
	logger.Debug("Pod OnUpdate")

	if oldPod.Status.PodIP != newPod.Status.PodIP {
		p.OnDelete(oldPod)
		p.OnAdd(newPod)
		return
	}

	if annotationDiffers(oldPod.Annotations, newPod.Annotations, p.storage.IamRoleKey) {
		logger.Debug("Updating pod due to added/updated annotation value")
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

	log.WithFields(p.podFields(pod)).Debug("Pod OnDelete")

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
