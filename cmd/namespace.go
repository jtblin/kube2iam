package cmd

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
)

type namespacehandler struct {
	storage *store
}

// OnAdd called with a namespace is added to k8s
func (h *namespacehandler) OnAdd(obj interface{}) {
	ns, ok := obj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnAdd handler received %+v", obj)
		return
	}

	log.Debugf("Namespace OnAdd %s", ns.GetName())

	roles := h.getRoleAnnotation(ns)
	if roles != nil {
		for i := range roles {
			log.Debugf("- Role %s", roles[i])
			h.storage.AddRoleToNamespace(ns.GetName(), roles[i])
		}
	}
}

// OnUpdate called with a namespace is updated inside k8s
func (h *namespacehandler) OnUpdate(oldObj, newObj interface{}) {
	//ons, ok := oldObj.(*api.Namespace)
	nns, ok := newObj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnUpdate handler received %+v", newObj)
		return
	}
	log.Debugf("Namespace OnUpdate %s", nns.GetName())

	roles := h.getRoleAnnotation(nns)
	if roles != nil {
		nsname := nns.GetName()
		h.storage.DeleteNamespace(nsname)
		for i := range roles {
			log.Debugf("- Role %s", roles[i])
			h.storage.AddRoleToNamespace(nsname, roles[i])
		}
	}
}

// OnDelete called with a namespace is removed from k8s
func (h *namespacehandler) OnDelete(obj interface{}) {
	ns, ok := obj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnDelete handler received %+v", obj)
		return
	}
	log.Debugf("Namespace OnDelete %s", ns.GetName())
	h.storage.DeleteNamespace(ns.GetName())
}

// getRoleAnnotations reads the "kube2iam/roles" annotation off a namespace
// and splits them on comma
func (h *namespacehandler) getRoleAnnotation(ns *api.Namespace) []string {
	rolesstring := ns.Annotations[h.storage.namespaceKey]
	if rolesstring != "" {
		var decoded []string
		if err := json.Unmarshal([]byte(rolesstring), &decoded); err != nil {
			log.Errorf("Unable to decode roles on namespace %s ( role annotation is '%s' )with error: %s", ns.Name, rolesstring, err)
		}
		return decoded
	}
	return nil
}

func newNamespacehandler(s *store) *namespacehandler {
	return &namespacehandler{
		storage: s,
	}
}
