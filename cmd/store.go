package cmd

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
	kcache "k8s.io/kubernetes/pkg/client/cache"
)

// store implements the k8s framework ResourceEventHandler interface.
type store struct {
	defaultRole string
	iamRoleKey  string
	mutex       sync.RWMutex
	rolesByIP   map[string]string
}

// Get returns the iam role based on IP address.
func (s *store) Get(IP string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if role, ok := s.rolesByIP[IP]; ok {
		return role, nil
	}
	if s.defaultRole != "" {
		log.Warnf("Using fallback role for IP %s", IP)
		return s.defaultRole, nil
	}
	return "", fmt.Errorf("Unable to find role for IP %s", IP)
}

// OnAdd is called when a pod is added.
func (s *store) OnAdd(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		log.Errorf("Bad object in OnAdd %+v", obj)
		return
	}

	if pod.Status.PodIP != "" {
		if role, ok := pod.Annotations[s.iamRoleKey]; ok {
			s.mutex.Lock()
			s.rolesByIP[pod.Status.PodIP] = role
			s.mutex.Unlock()
		}
	}
}

// OnUpdate is called when a pod is modified.
func (s *store) OnUpdate(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*api.Pod)
	newPod, ok2 := newObj.(*api.Pod)
	if !ok1 || !ok2 {
		log.Errorf("Bad call to OnUpdate %+v %+v", oldObj, newObj)
		return
	}

	if oldPod.Status.PodIP != newPod.Status.PodIP {
		s.OnDelete(oldPod)
		s.OnAdd(newPod)
	}
}

// OnDelete is called when a pod is deleted.
func (s *store) OnDelete(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		pod, ok = obj.(kcache.DeletedFinalStateUnknown).Obj.(*api.Pod)
	}

	if !ok {
		log.Errorf("Bad call to OnUpdate %+v %+v", oldObj, newObj)
		return
	}

	if pod.Status.PodIP != "" {
		s.mutex.Lock()
		delete(s.rolesByIP, pod.Status.PodIP)
		s.mutex.Unlock()
	}
}

func newStore(key string, defaultRole string) *store {
	return &store{
		defaultRole: defaultRole,
		iamRoleKey:  key,
		rolesByIP:   make(map[string]string),
	}
}
