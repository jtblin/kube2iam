package cmd

import (
	"fmt"
	"sync"

	"k8s.io/kubernetes/pkg/api"
)

// store implements the k8s framework ResourceEventHandler interface.
type store struct {
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
	return "", fmt.Errorf("Unable to find role for IP %s", IP)
}

// OnAdd is called when a pod is added.
func (s *store) OnAdd(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		if pod.Status.PodIP != "" {
			if role, ok := pod.Annotations[s.iamRoleKey]; ok {
				s.mutex.Lock()
				s.rolesByIP[pod.Status.PodIP] = role
				s.mutex.Unlock()
			}
		}
	}
}

// OnUpdate is called when a pod is modified.
func (s *store) OnUpdate(oldObj, newObj interface{}) {
	oldPod, okOld := oldObj.(*api.Pod)
	newPod, okNew := newObj.(*api.Pod)

	// Validate that the objects are good
	if okOld && okNew {
		if oldPod.Status.PodIP != newPod.Status.PodIP {
			s.OnDelete(oldPod)
			s.OnAdd(newPod)
		}
	} else if okNew {
		s.OnAdd(newPod)
	} else if okOld {
		s.OnDelete(oldPod)
	}
}

// OnDelete is called when a pod is deleted.
func (s *store) OnDelete(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		if pod.Status.PodIP != "" {
			s.mutex.Lock()
			delete(s.rolesByIP, pod.Status.PodIP)
			s.mutex.Unlock()
		}
	}
}

func newStore(key string) *store {
	return &store{
		iamRoleKey:  key,
		rolesByIP:   make(map[string]string),
	}
}
