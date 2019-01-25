package kube2iam

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
)

// NamespaceHandler outputs change events from K8.
type NamespaceHandler struct {
	namespaceKey string
}

func (h *NamespaceHandler) namespaceFields(ns *v1.Namespace) log.Fields {
	return log.Fields{
		"ns.name": ns.GetName(),
	}
}

// OnAdd called with a namespace is added to k8s.
func (h *NamespaceHandler) OnAdd(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnAdd handler received %+v", obj)
		return
	}

	logger := log.WithFields(h.namespaceFields(ns))
	logger.Debug("Namespace OnAdd")

	roles := GetNamespaceRoleAnnotation(ns, h.namespaceKey)
	for _, role := range roles {
		logger.WithField("ns.role", role).Info("Discovered role on namespace (OnAdd)")
	}
}

// OnUpdate called with a namespace is updated inside k8s.
func (h *NamespaceHandler) OnUpdate(oldObj, newObj interface{}) {
	nns, ok := newObj.(*v1.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnUpdate handler received %+v %+v", oldObj, newObj)
		return
	}
	logger := log.WithFields(h.namespaceFields(nns))
	logger.Debug("Namespace OnUpdate")

	roles := GetNamespaceRoleAnnotation(nns, h.namespaceKey)

	for _, role := range roles {
		logger.WithField("ns.role", role).Info("Discovered role on namespace (OnUpdate)")
	}
}

// OnDelete called with a namespace is removed from k8s.
func (h *NamespaceHandler) OnDelete(obj interface{}) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		log.Errorf("Expected Namespace but OnDelete handler received %+v", obj)
		return
	}
	log.WithFields(h.namespaceFields(ns)).Info("Deleting namespace (OnDelete)")
}

// GetNamespaceRoleAnnotation reads the "iam.amazonaws.com/allowed-roles" annotation off a namespace
// and splits them as a JSON list (["role1", "role2", "role3"])
func GetNamespaceRoleAnnotation(ns *v1.Namespace, namespaceKey string) []string {
	rolesString := ns.GetAnnotations()[namespaceKey]
	if rolesString != "" {
		var decoded []string
		if err := json.Unmarshal([]byte(rolesString), &decoded); err != nil {
			log.Errorf("Unable to decode roles on namespace %s ( role annotation is '%s' ) with error: %s", ns.Name, rolesString, err)
		}
		return decoded
	}
	return nil
}

// NamespaceIndexFunc maps a namespace to it's name.
func NamespaceIndexFunc(obj interface{}) ([]string, error) {
	namespace, ok := obj.(*v1.Namespace)
	if !ok {
		return nil, fmt.Errorf("expected namespace but received: %+v", obj)
	}

	return []string{namespace.GetName()}, nil
}

// NewNamespaceHandler returns a new namespace handler.
func NewNamespaceHandler(namespaceKey string) *NamespaceHandler {
	return &NamespaceHandler{
		namespaceKey: namespaceKey,
	}
}
