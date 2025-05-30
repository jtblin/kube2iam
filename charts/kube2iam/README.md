# kube2iam

Installs [kube2iam](https://github.com/jtblin/kube2iam) to provide IAM credentials to pods based on annotations.

## TL;DR;

```console
$ helm install my-release ./kube2iam
```

## Introduction

This chart bootstraps a [kube2iam](https://github.com/jtblin/kube2iam) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

*   Kubernetes 1.29+ (or a version supported by the deployed kube2iam)

## Installing the Chart

To install the chart with the release name `my-release`:

```console
# Add the chart repository (if you are using a repository)
$ helm repo add kube2iam https://jtblin.github.io/kube2iam/ # Replace with your actual repository URL if different
```

# Update your Helm repositories
`$ helm repo update`

# Install the chart
`$ helm install my-release kube2iam/kube2iam # If installing from a repository`

Alternatively, if you have the chart files locally:

```console
# Change to the directory containing the chart
$ cd charts/kube2iam

# Install the chart
$ helm install my-release ./
```

The command deploys kube2iam on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm uninstall my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the kube2iam chart and their default values.

| Parameter | Description | Default |
| --- | --- | --- |
| `affinity` | affinity configuration for pod assignment | `{}` |
| `extraArgs` | Additional container arguments | `{}` |
| `extraEnv` | Additional container environment variables | `{}` |
| `host.ip` | IP address of host | `$(HOST_IP)` |
| `host.iptables` | Add iptables rule | `false` |
| `host.interface` | Host interface for proxying AWS metadata | `docker0` |
| `host.port` | Port to listen on | `8181` |
| `image.repository` | Image | `jtblin/kube2iam` |
| `image.tag` | Image tag | `0.13.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.pullSecrets` | Image pull secrets | `[]` |
| `livenessProbe.enabled` | Enable/disable pod liveness probe | `true` |
| `livenessProbe.initialDelaySeconds` | Liveness probe initial delay | `30` |
| `livenessProbe.periodSeconds` | Liveness probe check interval | `5` |
| `livenessProbe.successThreshold` | Liveness probe success threshold | `1` |
| `livenessProbe.failureThreshold` | Liveness probe fail threshold | `3` |
| `livenessProbe.timeoutSeconds` | Liveness probe timeout | `1` |
| `nodeSelector` | node labels for pod assignment | `{}` |
| `podAnnotations` | annotations to be added to pods | `{}` |
| `priorityClassName` | priorityClassName to be added to pods | `{}` |
| `prometheus.metricsPort` | Port to expose prometheus metrics on (if unspecified, `host.port` is used) | `host.port` |
| `prometheus.service.enabled` | If true, create a Service resource for Prometheus | `false` |
| `prometheus.service.annotations` | Annotations to be added to the service | `{}` |
| `prometheus.serviceMonitor.enabled` | If true, create a Prometheus Operator ServiceMonitor resource | `false` |
| `prometheus.serviceMonitor.interval` | Interval at which the metrics endpoint is scraped | `10s` |
| `prometheus.serviceMonitor.namespace` | An alternative namespace in which to install the ServiceMonitor | `""` |
| `prometheus.serviceMonitor.labels` | Labels to add to the ServiceMonitor | `{}` |
| `rbac.create` | If true, create & use RBAC resources. Recommended for production. | `false` |
| `rbac.serviceAccountName` | existing ServiceAccount to use (ignored if rbac.create=true) | `default` |
| `readinessProbe.enabled` | Enable/disable pod readiness probe | `true` |
| `readinessProbe.initialDelaySeconds` | Readiness probe initial delay | `0` |
| `readinessProbe.periodSeconds` | Readiness probe check interval | `5` |
| `readinessProbe.successThreshold` | Readiness probe success threshold | `1` |
| `readinessProbe.failureThreshold` | Readiness probe fail threshold | `3` |
| `readinessProbe.timeoutSeconds` | Liveness probe timeout | `1` |
| `resources` | pod resource requests & limits | `{}` (Recommended to set in production) |
| `updateStrategy` | Strategy for DaemonSet updates. `RollingUpdate` recommended. | `OnDelete` |
| `maxUnavailable` | Maximum number of Pods that can be unavailable during the `RollingUpdate`. | `1` |
| `verbose` | Enable verbose output | `false` |
| `tolerations` | List of node taints to tolerate | `[]` |
| `aws.secret_key` | The value to use for AWS\_SECRET\_ACCESS\_KEY | `""` (Use `existingSecret` or other secure methods in production) |
| `aws.access_key` | The value to use for AWS\_ACCESS\_KEY\_ID | `""` (Use `existingSecret` or other secure methods in production) |
| `aws.region` | The AWS region to use | `""` |
| `existingSecret` | Set the AWS credentials using an existing secret | `""` (Recommended for production) |
| `podSecurityPolicy.enabled` | If true, create a podSecurityPolicy object. RBAC must also be enabled. (Deprecated in Kubernetes 1.25+, removed in 1.29+) | `false` |
| `podSecurityPolicy.annotations` | The annotations to add to the podSecurityPolicy object | `{}` |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```console
$ helm install my-release ./kube2iam \\
  --set=extraArgs.base-role-arn=arn:aws:iam::0123456789:role/,extraArgs.default-role=kube2iam-default,host.iptables=true,host.interface=cbr0
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example:

```console
$ helm install my-release ./kube2iam -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)