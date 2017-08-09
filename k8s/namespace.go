package k8s

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	"fmt"

	"github.com/jtblin/kube2iam/processor"
)

// NamespaceHandler outputs change events from K8
type NamespaceHandler struct {
	namespaceKey string
}

func namespaceIndexFunc(obj interface{}) ([]string, error) {
	namespace, ok := obj.(*v1.Namespace)
	if !ok {
		return nil, fmt.Errorf("Expected namespace but recieved: %+v", obj)
	}

	return []string{namespace.GetName()}, nil

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

	roles := processor.GetNamespaceRoleAnnotation(ns, h.namespaceKey)
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

	roles := processor.GetNamespaceRoleAnnotation(nns, h.namespaceKey)

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

// NewNamespaceHandler returns a new namespace handler.
func NewNamespaceHandler(namespaceKey string) *NamespaceHandler {
	return &NamespaceHandler{
		namespaceKey: namespaceKey,
	}
}
