# PSP/Calico example overlay

This [kustomize](https://github.com/kubernetes-sigs/kustomize) overlay does the following:

- adds a PodSecurity policy and required RBAC to the `base` resources
- overrides the container's `base` args
  - sets `host-interface=cali+` (for Calico)
  - enables `auto-discover-default-role`
  - enables `verbose`
- configures the container's livenessProbe
- sets the container's resource requests & limits
- overrides the container's image
- configures the pod's securityContext
- sets the `app.kubernetes.io/name: kube2iam` label on all resources
- deploys all resources to the `kube2iam` namespace (which must already exist)

**NOTE: This overlay is provided only as an example. It is strongly advised that users create & maintain their own to avoid unexpected configuration changes.**
