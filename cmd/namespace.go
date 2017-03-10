package cmd

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
)

type namespaceHandler struct {
	storage *store
}

// OnAdd called with a namespace is added to k8s
func (h *namespaceHandler) OnAdd(obj interface{}) {
	ns, ok := obj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnAdd handler received %+v", obj)
		return
	}

	log.Debugf("Namespace OnAdd %s", ns.GetName())

	roles := h.getRoleAnnotation(ns)
	for _, role := range roles {
		log.Debugf("- Role %s", role)
		h.storage.AddRoleToNamespace(ns.GetName(), role)
	}

}

// OnUpdate called with a namespace is updated inside k8s
func (h *namespaceHandler) OnUpdate(oldObj, newObj interface{}) {
	//ons, ok := oldObj.(*api.Namespace)
	nns, ok := newObj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnUpdate handler received %+v", newObj)
		return
	}
	log.Debugf("Namespace OnUpdate %s", nns.GetName())

	roles := h.getRoleAnnotation(nns)
	nsname := nns.GetName()
	h.storage.DeleteNamespace(nsname)
	for _, role := range roles {
		log.Debugf("- Role %s", role)
		h.storage.AddRoleToNamespace(nsname, role)
	}
}

// OnDelete called with a namespace is removed from k8s
func (h *namespaceHandler) OnDelete(obj interface{}) {
	ns, ok := obj.(*api.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnDelete handler received %+v", obj)
		return
	}
	log.Debugf("Namespace OnDelete %s", ns.GetName())
	h.storage.DeleteNamespace(ns.GetName())
}

// getRoleAnnotations reads the "iam.amazonaws.com/allowed-roles" annotation off a namespace
// and splits them as a JSON list (["role1", "role2", "role3"])
func (h *namespaceHandler) getRoleAnnotation(ns *api.Namespace) []string {
	rolesString := ns.Annotations[h.storage.namespaceKey]
	if rolesString != "" {
		var decoded []string
		if err := json.Unmarshal([]byte(rolesString), &decoded); err != nil {
			log.Errorf("Unable to decode roles on namespace %s ( role annotation is '%s' ) with error: %s", ns.Name, rolesString, err)
		}
		return decoded
	}
	return nil
}

func newNamespaceHandler(s *store) *namespaceHandler {
	return &namespaceHandler{
		storage: s,
	}
}
