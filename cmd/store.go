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
	hostIP      string
	mutex       sync.RWMutex
	rolesByIP   map[string]string
	onAdd       func(string, string)
	onDelete    func(string, string)
}

func (s *store) canTrackPod(pod *api.Pod) bool {
	if s.hostIP != "" {
		return pod.Status.HostIP == s.hostIP
	}
	return true
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
		log.Errorf("Expected Pod but OnAdd handler received %+v", obj)
		return
	}

	if pod.Status.PodIP != "" && s.canTrackPod(pod) {
		if role, ok := pod.Annotations[s.iamRoleKey]; ok {
			s.mutex.Lock()
			s.rolesByIP[pod.Status.PodIP] = role
			if s.onAdd != nil {
				s.onAdd(role, pod.Status.PodIP)
			}
			s.mutex.Unlock()
		}
	}
}

// OnUpdate is called when a pod is modified.
func (s *store) OnUpdate(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*api.Pod)
	newPod, ok2 := newObj.(*api.Pod)
	if !ok1 || !ok2 {
		log.Errorf("Expected Pod but OnUpdate handler received %+v %+v", oldObj, newObj)
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
		deletedObj, dok := obj.(kcache.DeletedFinalStateUnknown)
		if dok {
			pod, ok = deletedObj.Obj.(*api.Pod)
		}
	}

	if !ok {
		log.Errorf("Expected Pod but OnDelete handler received %+v", obj)
		return
	}

	if pod.Status.PodIP != "" && s.canTrackPod(pod) {
		s.mutex.Lock()
		role := s.rolesByIP[pod.Status.PodIP]
		delete(s.rolesByIP, pod.Status.PodIP)
		if s.onDelete != nil && role != "" {
			s.onDelete(role, pod.Status.PodIP)
		}
		s.mutex.Unlock()
	}
}

func newStore(key, defaultRole, hostIP string, onAdd, onDelete func(string, string)) *store {
	return &store{
		defaultRole: defaultRole,
		iamRoleKey:  key,
		hostIP:      hostIP,
		rolesByIP:   make(map[string]string),
		onAdd:       onAdd,
		onDelete:    onDelete,
	}
}
