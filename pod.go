package kube2iam

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/jtblin/kube2iam/store"
)

// PodHandler represents a pod handler.
type PodHandler struct {
	mutex   sync.RWMutex
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
	logger := log.WithFields(p.podFields(pod))
	logger.Debug("Pod OnAdd")

	p.storage.AddNamespaceToIP(pod)

	if pod.Status.PodIP != "" {
		if role, ok := pod.GetAnnotations()[p.storage.IamRoleKey]; ok {
			logger.Info("Adding pod to store")
			p.storage.AddRoleToIP(pod, role)
		}
	}
}

func (p *PodHandler) shouldUpdate(oldPod, newPod *v1.Pod) bool {
	return oldPod.Status.PodIP != newPod.Status.PodIP ||
		annotationDiffers(oldPod.GetAnnotations(), newPod.GetAnnotations(), p.storage.IamRoleKey)
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

	if p.shouldUpdate(oldPod, newPod) {
		logger.Info("Updating pod due to added/updated annotation value or different pod IP")
		p.mutex.Lock()
		defer p.mutex.Unlock()
		p.OnDelete(oldPod)
		p.OnAdd(newPod)
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

	logger := log.WithFields(p.podFields(pod))
	logger.Debug("Pod OnDelete")

	if pod.Status.PodIP != "" {
		logger.Info("Removing pod from store")
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
