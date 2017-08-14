package processor

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"

	"encoding/json"

	"github.com/jtblin/kube2iam/iam"
)

// RoleProcessor handles relevant logic around associating IPs with a given IAM role
type RoleProcessor struct {
	defaultRoleARN       string
	iamRoleKey           string
	namespaceKey         string
	namespaceRestriction bool
	iam                  *iam.Client
	kubeStore            kubeStore
}

type kubeStore interface {
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
func (r *RoleProcessor) GetRoleMapping(IP string) (*RoleMappingResult, error) {
	pod, err := r.kubeStore.PodByIP(IP)
	//If attempting to get a Pod that maps to multiple IPs
	if err != nil {
		return nil, err
	}

	role, err := r.extractRoleARN(pod)
	if err != nil {
		return nil, err
	}

	//Determine if normalized role is allowed to be used in pod's namespace
	if r.checkRoleForNamespace(role, pod.GetNamespace()) {
		return &RoleMappingResult{Role: role, Namespace: pod.GetNamespace(), IP: IP}, nil
	}

	return nil, fmt.Errorf("Role requested %s not valid for namespace of pod at %s with namespace %s", role, IP, pod.GetNamespace())
}

// extractQualifiedRoleName extracts a fully qualified ARN for a given pod,
// taking into consideration the appropriate fallback logic and defaulting
// logic along with the namespace role restrictions
func (r *RoleProcessor) extractRoleARN(pod *v1.Pod) (string, error) {
	rawRoleName, annotationPresent := pod.GetAnnotations()[r.iamRoleKey]

	if !annotationPresent && r.defaultRoleARN == "" {
		return "", fmt.Errorf("Unable to find role for IP %s", pod.Status.PodIP)
	}

	if !annotationPresent {
		log.Warnf("Using fallback role for IP %s", pod.Status.PodIP)
		rawRoleName = r.defaultRoleARN
	}

	return r.iam.RoleARN(rawRoleName), nil
}

// checkRoleForNamespace checks the 'database' for a role allowed in a namespace,
// returns true if the role is found, otheriwse false
func (r *RoleProcessor) checkRoleForNamespace(roleArn string, namespace string) bool {
	if !r.namespaceRestriction || roleArn == r.defaultRoleARN {
		return true
	}

	ns, err := r.kubeStore.NamespaceByName(namespace)
	if err != nil {
		log.Debug("Unable to find an indexed namespace of [%s]", ns)
		return false
	}

	ar := GetNamespaceRoleAnnotation(ns, r.namespaceKey)
	for _, role := range ar {
		if r.iam.RoleARN(role) == roleArn {
			log.Debugf("Role:%s on namespace:%s found.", roleArn, namespace)
			return true
		}
	}
	log.Warnf("Role: [%s] on namespace: [%s] not found.", roleArn, namespace)
	return false
}

// DumpDebugInfo outputs all the roles by IP addresr.
func (r *RoleProcessor) DumpDebugInfo() map[string]interface{} {
	output := make(map[string]interface{})
	rolesByIP := make(map[string]string)
	namespacesByIP := make(map[string]string)
	rolesByNamespace := make(map[string][]string)

	for _, ip := range r.kubeStore.ListPodIPs() {
		// When pods have `hostNetwork: true` they share an IP and we receive an error
		if pod, err := r.kubeStore.PodByIP(ip); err == nil {
			namespacesByIP[ip] = pod.Namespace
			if role, ok := pod.GetAnnotations()[r.iamRoleKey]; ok {
				rolesByIP[ip] = role
			} else {
				rolesByIP[ip] = ""
			}
		}
	}

	for _, namespaceName := range r.kubeStore.ListNamespaces() {
		if namespace, err := r.kubeStore.NamespaceByName(namespaceName); err == nil {
			rolesByNamespace[namespace.GetName()] = GetNamespaceRoleAnnotation(namespace, r.namespaceKey)
		}
	}

	output["rolesByIP"] = rolesByIP
	output["namespaceByIP"] = namespacesByIP
	output["rolesByNamespace"] = rolesByNamespace
	return output
}

// GetNamespaceRoleAnnotation reads the "iam.amazonawr.com/allowed-roles" annotation off a namespace
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

// NewRoleProcessor returns a new RoleProcessor for use.
func NewRoleProcessor(roleKey string, defaultRole string, namespaceRestriction bool, namespaceKey string, iamInstance *iam.Client, kubeStore kubeStore) *RoleProcessor {
	return &RoleProcessor{
		defaultRoleARN:       iamInstance.RoleARN(defaultRole),
		iamRoleKey:           roleKey,
		namespaceKey:         namespaceKey,
		namespaceRestriction: namespaceRestriction,
		iam:                  iamInstance,
		kubeStore:            kubeStore,
	}
}
