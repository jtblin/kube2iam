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
	podByName  map[string]*api.Pod
	podNameByIP map[string]string
	onAdd       func(string, string)
	onDelete    func(string, string)
}

// Return true if pod is not in a completed state, and its host ip matches ours
// (if provided).
func (s *store) canTrackPod(pod *api.Pod) bool {
	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
		return false
	} else if s.hostIP != "" {
		return pod.Status.HostIP == s.hostIP
	}
	return true
}

// Get returns the iam role based on IP address.
func (s *store) Get(IP string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get role via ip -> pod-name -> pod -> role-annotation
	if podName, ok := s.podNameByIP[IP]; ok {
		if pod, ok := s.podByName[podName]; ok {
			if role, ok := pod.Annotations[s.iamRoleKey]; ok {
				return role, nil
			}
		}
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

	role := pod.Annotations[s.iamRoleKey]
	podName, err := kcache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		log.Errorf("Couldn't get pod name for object %+v", pod)
		return
	}

	// Only assume roles and track by ip if the pod has an IP, and if we can
	// determine that the pod is on our host.
	if role != "" && pod.Status.PodIP != "" && s.canTrackPod(pod) {
		log.Infof("Tracking pod %s with ip %s and role %s", podName, pod.Status.PodIP, role)
		s.mutex.Lock()
		defer s.mutex.Unlock()

		s.podByName[podName] = pod
		s.podNameByIP[pod.Status.PodIP] = podName
		if s.onAdd != nil {
			s.onAdd(role, pod.Status.PodIP)
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

	// Status changed, this could indicate pod is not running anymore
	if oldPod.Status.Phase != newPod.Status.Phase {
		// Stop tracking pods that are not running, but have not been garbage collected
		if newPod.Status.Phase == api.PodSucceeded || newPod.Status.Phase == api.PodFailed {
			s.OnDelete(oldPod)
			return
		}
	}

	// Re-track pod if ip address changed
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


	podName, err := kcache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("Couldn't get pod name for object %+v", obj)
		return
	}

	log.Infof("Removing pod %s with ip %s in phase %s", podName, pod.Status.PodIP, pod.Status.Phase)
	role := pod.Annotations[s.iamRoleKey]

	// Remove pod
	s.mutex.Lock()
	defer s.mutex.Unlock()


	delete(s.podByName, podName)

	if ipPodName, ok := s.podNameByIP[pod.Status.PodIP]; ok {
		if ipPodName != podName {
			log.Warnf("Deleting pod %s for ip %s, but found pod with name %s", podName, pod.Status.PodIP, ipPodName)
		} else {
			delete(s.podNameByIP, podName)

			if s.onDelete != nil && role != "" {
				s.onDelete(role, pod.Status.PodIP)
			}
		}
	}
}

func newStore(key, defaultRole, hostIP string, onAdd, onDelete func(string, string)) *store {
	return &store{
		defaultRole: defaultRole,
		iamRoleKey:  key,
		hostIP:      hostIP,
		podByName:   make(map[string]*api.Pod),
		podNameByIP: make(map[string]string),
		onAdd:       onAdd,
		onDelete:    onDelete,
	}
}
