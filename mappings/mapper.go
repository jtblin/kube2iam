package mappings

import (
	"fmt"
	"regexp"
	"strings"

	glob "github.com/ryanuber/go-glob"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/jtblin/kube2iam"
	"github.com/jtblin/kube2iam/iam"
)

// RoleMapper handles relevant logic around associating IPs with a given IAM role
type RoleMapper struct {
	defaultRoleARN             string
	iamRoleKey                 string
	namespaceKey               string
	namespaceRestriction       bool
	iam                        *iam.Client
	store                      store
	namespaceRestrictionFormat string
}

type store interface {
	ListPodIPs() []string
	PodByIP(string) (*v1.Pod, error)
	ListNamespaces() []string
	NamespaceByName(string) (*v1.Namespace, error)
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

	return nil, fmt.Errorf("role requested %s not valid for namespace of pod at %s with namespace %s", role, IP, pod.GetNamespace())
}

// extractQualifiedRoleName extracts a fully qualified ARN for a given pod,
// taking into consideration the appropriate fallback logic and defaulting
// logic along with the namespace role restrictions
func (r *RoleMapper) extractRoleARN(pod *v1.Pod) (string, error) {
	rawRoleName, annotationPresent := pod.GetAnnotations()[r.iamRoleKey]

	if !annotationPresent && r.defaultRoleARN == "" {
		return "", fmt.Errorf("unable to find role for IP %s", pod.Status.PodIP)
	}

	if !annotationPresent {
		log.Warnf("Using fallback role for IP %s", pod.Status.PodIP)
		rawRoleName = r.defaultRoleARN
	}

	return r.iam.RoleARN(rawRoleName), nil
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

		if strings.ToLower(r.namespaceRestrictionFormat) == "regexp" {
			matched, err := regexp.MatchString(normalized, roleArn)
			if err != nil {
				log.Errorf("Namespace annotation %s caused an error when trying to match: %s for namespace: %s", rolePattern, roleArn, namespace)
			}
			if matched {
				log.Debugf("Role: %s matched %s on namespace:%s.", roleArn, rolePattern, namespace)
				return true
			}
		} else {
			if glob.Glob(normalized, roleArn) {
				log.Debugf("Role: %s matched %s on namespace:%s.", roleArn, rolePattern, namespace)
				return true
			}
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

	output["rolesByIP"] = rolesByIP
	output["namespaceByIP"] = namespacesByIP
	output["rolesByNamespace"] = rolesByNamespace
	return output
}

// NewRoleMapper returns a new RoleMapper for use.
func NewRoleMapper(roleKey string, defaultRole string, namespaceRestriction bool, namespaceKey string, iamInstance *iam.Client, kubeStore store, namespaceRestrictionFormat string) *RoleMapper {
	return &RoleMapper{
		defaultRoleARN:       iamInstance.RoleARN(defaultRole),
		iamRoleKey:           roleKey,
		namespaceKey:         namespaceKey,
		namespaceRestriction: namespaceRestriction,
		iam:                  iamInstance,
		store:                kubeStore,
		namespaceRestrictionFormat: namespaceRestrictionFormat,
	}
}
