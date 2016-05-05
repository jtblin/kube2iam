package cmd

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
)

const (
	iamRoleKey = "iam/role"
)

// store implements the k8s framework ResourceEventHandler interface
type store struct {
	iamRoleKey string
	mutex      sync.RWMutex
	rolesByIP  map[string]string
}

// Get returns the iam role based on IP address
func (s *store) Get(IP string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if role, ok := s.rolesByIP[IP]; ok {
		return role
	}
	return "default" // FIXME: hardcoding
}

// OnAdd is called when a pod is added.
func (s *store) OnAdd(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		if role, ok := pod.Annotations[s.iamRoleKey]; ok {
			log.Debugf("Adding IAM role %s for IP %s", role, pod.Status.PodIP)
			s.mutex.Lock()
			s.rolesByIP[pod.Status.PodIP] = role
			s.mutex.Unlock()
		}
	}
}

// OnUpdate is called when a pod is modified.
func (s *store) OnUpdate(oldObj, newObj interface{}) {
	s.OnDelete(oldObj)
	s.OnAdd(newObj)
}

// OnDelete is called when a pod is deleted.
func (s *store) OnDelete(obj interface{}) {
	if pod, ok := obj.(*api.Pod); ok {
		s.mutex.Lock()
		delete(s.rolesByIP, pod.Status.PodIP)
		s.mutex.Unlock()
	}
}

func newStore(key string) *store {
	if key == "" {
		key = iamRoleKey
	}
	return &store{
		iamRoleKey: key,
		rolesByIP:  make(map[string]string),
	}
}
