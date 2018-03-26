package mappings

import (
	"fmt"

	glob "github.com/ryanuber/go-glob"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/jtblin/kube2iam"
	"github.com/jtblin/kube2iam/iam"
)

// RoleMapper handles relevant logic around associating IPs with a given IAM role
type RoleMapper struct {
	defaultRoleARN       string
	iamRoleKey           string
	namespaceKey         string
	roleAliasConfigMaps  map[string][]string
	namespaceRestriction bool
	iam                  *iam.Client
	store                store
}

type store interface {
	ListPodIPs() []string
	PodByIP(string) (*v1.Pod, error)
	ListNamespaces() []string
	NamespaceByName(string) (*v1.Namespace, error)
	ConfigMap(name, ns string) (*v1.ConfigMap, error)
}

// RoleMappingResult represents the relevant information for a given mapping request
type RoleMappingResult struct {
	Role      string
	IP        string
	Namespace string
}

// GetRoleMapping returns the normalized iam RoleMappingResult based on IP address
func (r *RoleMapper) GetRoleMapping(IP string) (*RoleMappingResult, error) {
	pod, err := r.store.PodByIP(IP)
	// If attempting to get a Pod that maps to multiple IPs
	if err != nil {
		return nil, err
	}

	role, err := r.extractRoleARN(pod)
	if err != nil {
		return nil, err
	}

	// Determine if normalized role is allowed to be used in pod's namespace
	if r.checkRoleForNamespace(role, pod.GetNamespace()) {
		return &RoleMappingResult{Role: role, Namespace: pod.GetNamespace(), IP: IP}, nil
	}

	return nil, fmt.Errorf("Role requested %s not valid for namespace of pod at %s with namespace %s", role, IP, pod.GetNamespace())
}

// extractQualifiedRoleName extracts a fully qualified ARN for a given pod,
// taking into consideration the appropriate fallback logic and defaulting
// logic along with the namespace role restrictions
func (r *RoleMapper) extractRoleARN(pod *v1.Pod) (string, error) {
	rawRoleName, annotationPresent := pod.GetAnnotations()[r.iamRoleKey]

	if !annotationPresent && r.defaultRoleARN == "" {
		return "", fmt.Errorf("Unable to find role for IP %s", pod.Status.PodIP)
	}

	if !annotationPresent {
		log.Warnf("Using fallback role for IP %s", pod.Status.PodIP)
		rawRoleName = r.defaultRoleARN
	}

	roleAlias, err := r.findRoleAlias(rawRoleName)
	if err != nil {
		return "", err
	}

	if roleAlias != "" {
		rawRoleName = roleAlias
	}

	return r.iam.RoleARN(rawRoleName), nil
}

func (r *RoleMapper) findRoleAlias(roleName string) (string, error) {
	for ns, names := range r.roleAliasConfigMaps {
		for _, name := range names {
			cm, err := r.store.ConfigMap(name, ns)
			if err != nil {
				return "", err
			}

			if roleAlias, ok := cm.Data[roleName]; ok {
				return roleAlias, nil
			}
		}
	}

	return "", nil
}

// checkRoleForNamespace checks the 'database' for a role allowed in a namespace,
// returns true if the role is found, otheriwse false
func (r *RoleMapper) checkRoleForNamespace(roleArn string, namespace string) bool {
	if !r.namespaceRestriction || roleArn == r.defaultRoleARN {
		return true
	}

	ns, err := r.store.NamespaceByName(namespace)
	if err != nil {
		log.Debug("Unable to find an indexed namespace of %s", namespace)
		return false
	}

	ar := kube2iam.GetNamespaceRoleAnnotation(ns, r.namespaceKey)
	for _, rolePattern := range ar {
		normalized := r.iam.RoleARN(rolePattern)
		if glob.Glob(normalized, roleArn) {
			log.Debugf("Role: %s matched %s on namespace:%s.", roleArn, rolePattern, namespace)
			return true
		}
	}
	log.Warnf("Role: %s on namespace: %s not found.", roleArn, namespace)
	return false
}

// DumpDebugInfo outputs all the roles by IP address.
func (r *RoleMapper) DumpDebugInfo() map[string]interface{} {
	output := make(map[string]interface{})
	rolesByIP := make(map[string]string)
	namespacesByIP := make(map[string]string)
	rolesByNamespace := make(map[string][]string)
	roleAliasesByNamespace := make(map[string]map[string]string)

	for _, ip := range r.store.ListPodIPs() {
		// When pods have `hostNetwork: true` they share an IP and we receive an error
		if pod, err := r.store.PodByIP(ip); err == nil {
			namespacesByIP[ip] = pod.Namespace
			if role, ok := pod.GetAnnotations()[r.iamRoleKey]; ok {
				rolesByIP[ip] = role
			} else {
				rolesByIP[ip] = ""
			}
		}
	}

	for _, namespaceName := range r.store.ListNamespaces() {
		if namespace, err := r.store.NamespaceByName(namespaceName); err == nil {
			rolesByNamespace[namespace.GetName()] = kube2iam.GetNamespaceRoleAnnotation(namespace, r.namespaceKey)
		}
	}

	for ns, cmns := range r.roleAliasConfigMaps {
		for _, cmn := range cmns {
			cm, err := r.store.ConfigMap(cmn, ns)
			if err == nil {
				cmm, ok := roleAliasesByNamespace[ns]
				if !ok {
					cmm = make(map[string]string)
					roleAliasesByNamespace[ns] = cmm
				}

				for k, v := range cm.Data {
					cmm[k] = v
				}
			}
		}
	}

	output["rolesByIP"] = rolesByIP
	output["namespaceByIP"] = namespacesByIP
	output["rolesByNamespace"] = rolesByNamespace
	output["roleAlisesByNamespace"] = roleAliasesByNamespace
	return output
}

// NewRoleMapper returns a new RoleMapper for use.
func NewRoleMapper(
	roleKey string,
	defaultRole string,
	namespaceRestriction bool,
	namespaceKey string,
	roleAliasConfigMaps map[string][]string,
	iamInstance *iam.Client,
	kubeStore store,
) *RoleMapper {
	return &RoleMapper{
		defaultRoleARN:       iamInstance.RoleARN(defaultRole),
		iamRoleKey:           roleKey,
		namespaceKey:         namespaceKey,
		namespaceRestriction: namespaceRestriction,
		roleAliasConfigMaps:  roleAliasConfigMaps,
		iam:                  iamInstance,
		store:                kubeStore,
	}
}
