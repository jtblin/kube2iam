package kube2iam

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/jtblin/kube2iam/store"
)

// NamespaceHandler represents a namespace handler.
type NamespaceHandler struct {
	storage *store.Store
}

// OnAdd called with a namespace is added to k8s
func (h *NamespaceHandler) OnAdd(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
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
func (h *NamespaceHandler) OnUpdate(oldObj, newObj interface{}) {
	//ons, ok := oldObj.(*v1.Namespace)
	nns, ok := newObj.(*v1.Namespace)
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
func (h *NamespaceHandler) OnDelete(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnDelete handler received %+v", obj)
		return
	}
	log.Debugf("Namespace OnDelete %s", ns.GetName())
	h.storage.DeleteNamespace(ns.GetName())
}

// getRoleAnnotations reads the "iam.amazonaws.com/allowed-roles" annotation off a namespace
// and splits them as a JSON list (["role1", "role2", "role3"])
func (h *NamespaceHandler) getRoleAnnotation(ns *v1.Namespace) []string {
	rolesString := ns.Annotations[h.storage.NamespaceKey]
	if rolesString != "" {
		var decoded []string
		if err := json.Unmarshal([]byte(rolesString), &decoded); err != nil {
			log.Errorf("Unable to decode roles on namespace %s ( role annotation is '%s' ) with error: %s", ns.Name, rolesString, err)
		}
		return decoded
	}
	return nil
}

// NewNamespaceHandler returns a new namespace handler.
func NewNamespaceHandler(s *store.Store) *NamespaceHandler {
	return &NamespaceHandler{
		storage: s,
	}
}
