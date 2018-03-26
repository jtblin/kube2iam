package kube2iam

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
)

// ConfigMapHandler outputs change events from K8.
type ConfigMapHandler struct {
	watchConfigMaps map[string][]string
}

func (c *ConfigMapHandler) configMapFields(cm *v1.ConfigMap) log.Fields {
	return log.Fields{
		"configmap.name": cm.ObjectMeta.Name,
	}
}

func (c *ConfigMapHandler) isWatchConfigMap(cm *v1.ConfigMap) bool {
	ns := cm.ObjectMeta.Namespace
	cmn := cm.ObjectMeta.Name

	for wns, wcmns := range c.watchConfigMaps {
		for _, wcmn := range wcmns {
			if ns == wns && cmn == wcmn {
				return true
			}
		}
	}

	return false
}

// OnAdd is called when a configmap is added.
func (c *ConfigMapHandler) OnAdd(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		log.Errorf("Expected ConfigMap but OnAdd handler received %+v", obj)
		return
	}

	logger := log.WithFields(c.configMapFields(cm))
	logger.Debug("ConfigMap OnAdd")

	if c.isWatchConfigMap(cm) {
		logger.Info("Discovered watch configmap (OnAdd)")
	}
}

// OnUpdate called with a configmap is updated inside k8s.
func (c *ConfigMapHandler) OnUpdate(oldObj, newObj interface{}) {
	_, ok1 := oldObj.(*v1.ConfigMap)
	ncm, ok2 := newObj.(*v1.ConfigMap)
	if !ok1 || !ok2 {
		log.Errorf("Expected ConfigMap but OnUpdate handler received %+v %+v", oldObj, newObj)
		return
	}
	logger := log.WithFields(c.configMapFields(ncm))
	logger.Debug("ConfigMap OnUpdate")

	if c.isWatchConfigMap(ncm) {
		logger.Info("Discovered watch configmap (OnUpdate)")
	}
}

// OnDelete is called when a configmap is deleted.
func (c *ConfigMapHandler) OnDelete(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		log.Errorf("Expected ConfigMap but OnDelete handler received %+v", obj)
		return
	}

	logger := log.WithFields(c.configMapFields(cm))
	logger.Debug("ConfigMap OnDelete")

	if c.isWatchConfigMap(cm) {
		logger.Info("Deleting watch configmap (OnDelete)")
	}
}

// ConfigMapIndexFunc returns an index func that maps a given ConfigMap to its name & namespace for caching.
func ConfigMapIndexFunc(roleAliasConfigMaps map[string][]string) func(obj interface{}) ([]string, error) {
	return func(obj interface{}) ([]string, error) {
		cm, ok := obj.(*v1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("obj not configmap: %+v", obj)
		}

		ns := cm.ObjectMeta.Namespace
		cmn := cm.ObjectMeta.Name

		names, ok := roleAliasConfigMaps[ns]
		if !ok {
			return nil, nil
		}

		for _, name := range names {
			if name == cmn {
				return []string{cmn}, nil
			}
		}

		return nil, nil
	}
}

// NewConfigMapHandler returns a new configmap handler.
func NewConfigMapHandler(watchConfigMaps map[string][]string) *ConfigMapHandler {
	return &ConfigMapHandler{
		watchConfigMaps: watchConfigMaps,
	}
}
