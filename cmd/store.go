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
	pod := obj.(*api.Pod)
	log.Infof("See add of pod with ip %s: role %s", pod.Status.PodIP, pod.Annotations[s.iamRoleKey])

	s.doAdd(pod)
}

func (s *store) doAdd(pod *api.Pod) {
	if pod.Status.PodIP != "" {
		if role, ok := pod.Annotations[s.iamRoleKey]; ok {
			s.mutex.Lock()
			s.rolesByIP[pod.Status.PodIP] = role
			s.mutex.Unlock()
		}
	}
}

func (s *store) isMismatch(pod *api.Pod) bool {
	if pod.Status.PodIP == "" {
		return false
	}

	actualRole, ok := pod.Annotations[s.iamRoleKey]
	if !ok {
		return false
	}

	currentRole, _ := s.Get(pod.Status.PodIP)
	if currentRole == actualRole {
		return false
	}

	log.Errorf("Mismatch: ip %s mapped to %s instead of %s", pod.Status.PodIP, currentRole, actualRole)
	return true
}

// OnUpdate is called when a pod is modified.
func (s *store) OnUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*api.Pod)
	newPod := newObj.(*api.Pod)

	if oldPod.Status.PodIP != newPod.Status.PodIP ||
		oldPod.Annotations[s.iamRoleKey] != newPod.Annotations[s.iamRoleKey] ||
		s.isMismatch(oldPod) || s.isMismatch(newPod) {
		log.Infof("See update of old pod with ip %s: role %s", oldPod.Status.PodIP, oldPod.Annotations[s.iamRoleKey])
		log.Infof("See update of new pod with ip %s: role %s", newPod.Status.PodIP, newPod.Annotations[s.iamRoleKey])
		s.doDelete(oldPod)
		s.doAdd(newPod)
	}
}

// OnDelete is called when a pod is deleted.
func (s *store) OnDelete(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		pod = obj.(kcache.DeletedFinalStateUnknown).Obj.(*api.Pod)
	}

	log.Infof("See delete of pod with ip %s: role %s", pod.Status.PodIP, pod.Annotations[s.iamRoleKey])
	s.doDelete(pod)
}

func (s *store) doDelete(pod *api.Pod) {
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
