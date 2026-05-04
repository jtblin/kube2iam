# kube2iam

Provide IAM credentials to pods based on annotations.

**Homepage:** <https://github.com/jtblin/kube2iam>

## TL;DR;

```console
$ helm install my-release ./kube2iam
```

## Introduction

This chart bootstraps a [kube2iam](https://github.com/jtblin/kube2iam) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

*   Kubernetes 1.29+ (or a version supported by the deployed kube2iam)

To install the chart with the release name `my-release`:

```console
$ helm install my-release oci://ghcr.io/jtblin/kube2iam-chart --version [VERSION]
```

Alternatively, if you have the chart files locally:

```console
$ helm install my-release ./charts/kube2iam
```

The command deploys kube2iam on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm uninstall my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| mariusv | <myself@mariusv.com> |  |

## Source Code

* <https://github.com/jtblin/kube2iam>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity configuration for pod assignment (ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) |
| aws.access_key | string | `""` | The value to use for AWS_ACCESS_KEY_ID |
| aws.region | string | `""` | The AWS region to use |
| aws.secret_key | string | `""` | The value to use for AWS_SECRET_ACCESS_KEY |
| existingSecret | string | `""` | Set the AWS credentials using an existing secret |
| extraArgs | object | `{}` | Additional container arguments |
| extraEnv | object | `{}` | Additional container environment variables |
| host.interface | string | `"docker0"` | Host interface for proxying AWS metadata |
| host.ip | string | `"$(HOST_IP)"` | IP address of host |
| host.iptables | bool | `false` | Add iptables rule |
| host.port | int | `8181` | Port to listen on |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"jtblin/kube2iam"` | Image repository |
| image.tag | string | `"0.13.0"` | Image tag |
| livenessProbe.enabled | bool | `true` | Enable/disable pod liveness probe |
| livenessProbe.failureThreshold | int | `3` | Liveness probe fail threshold |
| livenessProbe.initialDelaySeconds | int | `30` | Liveness probe initial delay |
| livenessProbe.periodSeconds | int | `5` | Liveness probe check interval |
| livenessProbe.successThreshold | int | `1` | Liveness probe success threshold |
| livenessProbe.timeoutSeconds | int | `1` | Liveness probe timeout |
| maxUnavailable | int | `1` | Maximum number of Pods that can be unavailable during update (ref: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) |
| nodeSelector | object | `{}` | Node labels for pod assignment (ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) |
| podAnnotations | object | `{}` | Annotations to be added to pods |
| podLabels | object | `{}` | Labels to be added to pods |
| podSecurityPolicy.annotations | object | `{}` | Annotations to add to the podSecurityPolicy object |
| podSecurityPolicy.enabled | bool | `false` | If true, create a podSecurityPolicy object (Deprecated in K8s 1.25+) |
| priorityClassName | string | `""` | priorityClassName to be added to pods |
| prometheus.service.enabled | bool | `false` | If true, create a Service resource for Prometheus |
| prometheus.serviceMonitor.enabled | bool | `false` | Create prometheus-operator ServiceMonitor |
| prometheus.serviceMonitor.interval | string | `"10s"` | Interval at which the metrics endpoint is scraped |
| prometheus.serviceMonitor.labels | object | `{}` | Labels to add to the service monitor |
| prometheus.serviceMonitor.namespace | string | `""` | Alternative namespace to install the ServiceMonitor in |
| rbac.create | bool | `false` | If true, create & use RBAC resources. Recommended for production. |
| rbac.serviceAccountName | string | `"default"` | existing ServiceAccount to use (ignored if rbac.create=true) |
| readinessProbe.enabled | bool | `true` | Enable/disable pod readiness probe |
| readinessProbe.failureThreshold | int | `3` | Readiness probe failure threshold |
| readinessProbe.initialDelaySeconds | int | `0` | Readiness probe initial delay |
| readinessProbe.periodSeconds | int | `5` | Readiness probe check interval |
| readinessProbe.successThreshold | int | `1` | Readiness probe success threshold |
| readinessProbe.timeoutSeconds | int | `1` | Readiness probe timeout |
| resources | object | `{}` | pod resource requests & limits |
| tolerations | list | `[]` | List of node taints to tolerate |
| updateStrategy | string | `"OnDelete"` | Strategy for DaemonSet updates (ref: https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/) |
| verbose | bool | `false` | Enable verbose output |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```console
$ helm install my-release ./kube2iam \
  --set=extraArgs.base-role-arn=arn:aws:iam::0123456789:role/,extraArgs.default-role=kube2iam-default,host.iptables=true,host.interface=cbr0
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example:

```console
$ helm install my-release ./kube2iam -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)
