[![Build Status](https://travis-ci.org/jtblin/kube2iam.svg?branch=master)](https://travis-ci.org/jtblin/kube2iam)
[![GitHub tag](https://img.shields.io/github/tag/jtblin/kube2iam.svg?maxAge=86400)](https://github.com/jtblin/kube2iam)
[![Docker Pulls](https://img.shields.io/docker/pulls/jtblin/kube2iam.svg)](https://hub.docker.com/r/jtblin/kube2iam/)
[![Go Report Card](https://goreportcard.com/badge/github.com/jtblin/kube2iam)](https://goreportcard.com/report/github.com/jtblin/kube2iam)
[![license](https://img.shields.io/github/license/jtblin/kube2iam.svg)](https://github.com/jtblin/kube2iam/blob/master/LICENSE)

# kube2iam

Provide IAM credentials to containers running inside a kubernetes cluster based on annotations.

## Context

Traditionally in AWS, service level isolation is done using IAM roles. IAM roles are attributed through instance
profiles and are accessible by services through the transparent usage by the aws-sdk of the ec2 metadata API.
When using the aws-sdk, a call is made to the EC2 metadata API which provides temporary credentials
that are then used to make calls to the AWS service.

## Problem statement

The problem is that in a multi-tenanted containers based world, multiple containers will be sharing the underlying
nodes. Given containers will share the same underlying nodes, providing access to AWS
resources via IAM roles would mean that one needs to create an IAM role which is a union of all
IAM roles. This is not acceptable from a security perspective.

## Solution

The solution is to redirect the traffic that is going to the ec2 metadata API for docker containers to a container
running on each instance, make a call to the AWS API to retrieve temporary credentials and return these to the caller.
Other calls will be proxied to the EC2 metadata API. This container will need to run with host networking enabled
so that it can call the EC2 metadata API itself.

## Usage

### IAM roles

It is necessary to create an IAM role which can assume other roles and assign it to each kubernetes worker.

```
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "sts:AssumeRole"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

The roles that will be assumed must have a Trust Relationship which allows them to be assumed by the kubernetes worker
role. See this [StackOverflow post](http://stackoverflow.com/a/33850060) for more details.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    },
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::123456789012:role/kubernetes-worker-role"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### kube2iam daemonset

Run the kube2iam container as a daemonset (so that it runs on each worker) with `hostNetwork: true`.
The kube2iam daemon and iptables rule (see below) need to run before all other pods that would require
access to AWS resources.

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube2iam
  labels:
    app: kube2iam
spec:
  selector:
    matchLabels:
      name: kube2iam
  template:
    metadata:
      labels:
        name: kube2iam
    spec:
      hostNetwork: true
      containers:
        - image: jtblin/kube2iam:latest
          name: kube2iam
          args:
            - "--base-role-arn=arn:aws:iam::123456789012:role/"
            - "--node=$(NODE_NAME)"
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          ports:
            - containerPort: 8181
              hostPort: 8181
              name: http
```

### iptables

To prevent containers from directly accessing the EC2 metadata API and gaining unwanted access to AWS resources,
the traffic to `169.254.169.254` must be proxied for docker containers.

```bash
iptables \
  --append PREROUTING \
  --protocol tcp \
  --destination 169.254.169.254 \
  --dport 80 \
  --in-interface docker0 \
  --jump DNAT \
  --table nat \
  --to-destination `curl 169.254.169.254/latest/meta-data/local-ipv4`:8181
```

This rule can be added automatically by setting `--iptables=true`, setting the `HOST_IP` environment
variable, and running the container in a privileged security context.

**Warning**: It is possible that other pods are started on an instance before kube2iam has started. Using `--iptables=true` (instead of applying the rule before starting the kubelet) **could give those pods the opportunity to access the real EC2 metadata API, assume the role of the EC2 instance and thereby have all permissions the instance role has** (including assuming potential other roles). Use with care if you don't trust the users of your kubernetes cluster or if you are running pods (that could be exploited) that have permissions to create other pods (e.g. controllers / operators).

Note that the interface `--in-interface` above or using the `--host-interface` cli flag may be
different than `docker0` depending on which virtual network you use e.g.

* for Calico, use `cali+` (the interface name is something like cali1234567890)
* for kops (on kubenet), use `cbr0`
* for CNI, use `cni0`
* for [EKS](https://docs.aws.amazon.com/eks/latest/userguide/what-is-eks.html)/[amazon-vpc-cni-k8s](https://github.com/aws/amazon-vpc-cni-k8s), even with calico installed uses `eni+`. (Each pod gets an interface like `eni4c0e15dfb05`)
  * If using security groups per pod however, you will need to instead use `!eth0` as pods making use of security groups per pod [will use](https://aws.amazon.com/blogs/containers/introducing-security-groups-for-pods/) `vlan` interfaces as well as the `eni+` interfaces used for other pods.
* for weave use `weave`
* for flannel use `cni0`
* for [kube-router](https://github.com/cloudnativelabs/kube-router) use `kube-bridge`
* for [OpenShift](https://www.openshift.org/) use `tun0`
* for [Cilium](https://www.cilium.io) use `lxc+`

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube2iam
  labels:
    app: kube2iam
spec:
  selector:
    matchLabels:
      name: kube2iam
  template:
    metadata:
      labels:
        name: kube2iam
    spec:
      hostNetwork: true
      containers:
        - image: jtblin/kube2iam:latest
          name: kube2iam
          args:
            - "--base-role-arn=arn:aws:iam::123456789012:role/"
            - "--iptables=true"
            - "--host-ip=$(HOST_IP)"
            - "--node=$(NODE_NAME)"
          env:
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          ports:
            - containerPort: 8181
              hostPort: 8181
              name: http
          securityContext:
            privileged: true
```

### kubernetes annotation

Add an `iam.amazonaws.com/role` annotation to your pods with the role that you want to assume for this pod.
The optional `iam.amazonaws.com/external-id` will allow the use of an ExternalId as part of the assume role

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: aws-cli
  labels:
    name: aws-cli
  annotations:
    iam.amazonaws.com/role: role-arn
    iam.amazonaws.com/external-id: external-id
spec:
  containers:
  - image: fstab/aws-cli
    command:
      - "/home/aws/aws/env/bin/aws"
      - "s3"
      - "ls"
      - "some-bucket"
    name: aws-cli
```

You can use `--default-role` to set a fallback role to use when annotation is not set.

#### ReplicaSet, CronJob, Deployment, etc.

When creating higher-level abstractions than pods, you need to pass the annotation in the pod template of the
resource spec.

Example for a `Deployment`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        iam.amazonaws.com/role: role-arn
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.9.1
        ports:
        - containerPort: 80
```

Example for a `CronJob`:

```yaml
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: my-cronjob
spec:
  schedule: "00 11 * * 2"
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 3600
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            iam.amazonaws.com/role: role-arn
        spec:
          restartPolicy: OnFailure
          containers:
          - name: job
            image: my-image
```

### Namespace Restrictions

By using the flag --namespace-restrictions you can enable a mode in which the roles that pods can assume is restricted
by an annotation on the pod's namespace. This annotation should be in the form of a json array.

To allow the aws-cli pod specified above to run in the default namespace your namespace would look like the following.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    iam.amazonaws.com/allowed-roles: |
      ["role-arn"]
  name: default
```

_Note:_ You can also use glob-based matching for namespace restrictions, which works nicely with the path-based
namespacing supported for AWS IAM roles.

Example: to allow all roles prefixed with `my-custom-path/` to be assumed by pods in the default namespace, the
default namespace would be annotated as follows:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    iam.amazonaws.com/allowed-roles: |
      ["my-custom-path/*"]
  name: default
```

If you prefer `regexp` to glob-based matching you can specify `--namespace-restriction-format=regexp`, then you can
use a `regexp` in your annotation:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    iam.amazonaws.com/allowed-roles: |
      ["my-custom-path/.*"]
  name: default
```

### RBAC Setup

This is the basic RBAC setup to get kube2iam working correctly when your cluster is using rbac. Below is the bare minimum to get kube2iam working.

First we need to make a service account.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube2iam
  namespace: kube-system
```

Next we need to setup roles and binding for the the process.

```yaml
---
apiVersion: v1
items:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: kube2iam
    rules:
      - apiGroups: [""]
        resources: ["namespaces","pods"]
        verbs: ["get","watch","list"]
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: kube2iam
    subjects:
    - kind: ServiceAccount
      name: kube2iam
      namespace: kube-system
    roleRef:
      kind: ClusterRole
      name: kube2iam
      apiGroup: rbac.authorization.k8s.io
kind: List
```

You will notice this lives in the kube-system namespace to allow for easier seperation between system services and other services.

Here is what a kube2iam daemonset yaml might look like.

```yaml
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube2iam
  namespace: kube-system
  labels:
    app: kube2iam
spec:
  selector:
    matchLabels:
      name: kube2iam
  template:
    metadata:
      labels:
        name: kube2iam
    spec:
      serviceAccountName: kube2iam
      hostNetwork: true
      containers:
        - image: jtblin/kube2iam:latest
          imagePullPolicy: Always
          name: kube2iam
          args:
            - "--app-port=8181"
            - "--base-role-arn=arn:aws:iam::xxxxxxx:role/"
            - "--iptables=true"
            - "--host-ip=$(HOST_IP)"
            - "--host-interface=weave"
            - "--verbose"
          env:
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          ports:
            - containerPort: 8181
              hostPort: 8181
              name: http
          securityContext:
            privileged: true
```

### Using on OpenShift

#### OpenShift 3

To use `kube2iam` on OpenShift one needs to configure additional resources.

A complete example for OpenShift 3 looks like this. For OpenShift 4, see the next section.
```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube2iam
  namespace: kube-system
---
apiVersion: v1
items:
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRole
    metadata:
      name: kube2iam
    rules:
      - apiGroups: [""]
        resources: ["namespaces","pods"]
        verbs: ["get","watch","list"]
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRoleBinding
    metadata:
      name: kube2iam
    subjects:
    - kind: ServiceAccount
      name: kube2iam
      namespace: kube-system
    roleRef:
      kind: ClusterRole
      name: kube2iam
      apiGroup: rbac.authorization.k8s.io
kind: List
---
kind: SecurityContextConstraints
apiVersion: v1
metadata:
  name: kube2iam
allowPrivilegedContainer: true
allowHostPorts: true
allowHostNetwork: true
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: MustRunAs
users:
- system:serviceacount:kube-system:kube2iam
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube2iam
  namespace: kube-system
  labels:
    app: kube2iam
spec:
  selector:
    matchLabels:
      name: kube2iam
  template:
    metadata:
      labels:
        name: kube2iam
    spec:
      serviceAccountName: kube2iam
      hostNetwork: true
      nodeSelector:
        role: app
      containers:
        - image: docker.io/jtblin/kube2iam:latest
          imagePullPolicy: Always
          name: kube2iam
          args:
            - "--app-port=8181"
            - "--auto-discover-base-arn"
            - "--iptables=true"
            - "--host-ip=$(HOST_IP)"
            - "--host-interface=tun0"
            - "--verbose"
          env:
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          ports:
            - containerPort: 8181
              hostPort: 8181
              name: http
          securityContext:
            privileged: true
```

**Note**: In (OpenShift) multi-tenancy setups it is recommended to restrict the assumable roles on the namespace level to prevent cross-namespace trust stealing.

#### OpenShift 4

To use `kube2iam` on OpenShift 4, the additional resources are slightly different from those for OpenShift 3 shown above. OpenShift 4 has [hard-coded iptables rules](https://github.com/openshift/origin/blob/release-4.1/cmd/sdn-cni-plugin/openshift-sdn_linux.go#L129) that block connections from containers to the EC2 metadata service 169.254.169.254. The `kube2iam` pods already run with host networking enabled, they are not affected by these OpenShift iptables rules.

The OpenShift iptables rules have implications for pods authenticating through `kube2iam` though. But let's look at an example for deploying `kube2iam` on OpenShift 4 first:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube2iam
  namespace: kube-system
---
apiVersion: v1
items:
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRole
    metadata:
      name: kube2iam
    rules:
      - apiGroups: [""]
        resources: ["namespaces","pods"]
        verbs: ["get","watch","list"]
  - apiVersion: rbac.authorization.k8s.io/v1beta1
    kind: ClusterRoleBinding
    metadata:
      name: kube2iam
    subjects:
    - kind: ServiceAccount
      name: kube2iam
      namespace: kube-system
    roleRef:
      kind: ClusterRole
      name: kube2iam
      apiGroup: rbac.authorization.k8s.io
kind: List
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube2iam
  namespace: kube-system
  labels:
    app: kube2iam
spec:
  selector:
    matchLabels:
      name: kube2iam
  template:
    metadata:
      labels:
        name: kube2iam
    spec:
      serviceAccountName: kube2iam
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/worker: ''
      containers:
        - image: docker.io/jtblin/kube2iam:latest
          imagePullPolicy: Always
          name: kube2iam
          args:
            - "--app-port=8181"
            - "--auto-discover-base-arn"
            - "--host-ip=$(HOST_IP)"
            - "--host-interface=tun0"
            - "--verbose"
          env:
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          ports:
            - containerPort: 8181
              hostPort: 8181
              name: http
```

Compared to the OpenShift 3 example in the previous section, we removed the `kube2iam` SecurityContextConstraint. In the `kube2iam` DaemonSet, we changed the nodeSelector to the match OpenShift 4 worker nodes, removed the iptables argument, and removed the `privileged` securityContext.

We use the OpenShift `hostnetwork` SecurityContextConstraint for `kube2iam`:

```
oc adm policy add-scc-to-user hostnetwork -n kube-system -z kube2iam
```

For applications, the iptables rule that `kube2iam` would create to redirect 169.254.169.254 connections to the `kube2iam` pods has no effect because the [hard-coded iptables rules](https://github.com/openshift/origin/blob/release-4.1/cmd/sdn-cni-plugin/openshift-sdn_linux.go#L129) block those connections on OpenShift 4.

As a workaround, the environment variables http_proxy and no_proxy can be set to use `kube2iam` as a HTTP proxy when accessing the metadata service. Below is an example for the aws-service-operator:

```
- kind: Deployment
  apiVersion: apps/v1beta1
  metadata:
    name: aws-service-operator
    namespace: aws-service-operator
  spec:
    replicas: 1
    template:
      metadata:
        annotations:
          iam.amazonaws.com/role: aws-service-operator
        labels:
          app: aws-service-operator
      spec:
        serviceAccountName: aws-service-operator
        containers:
        - name: aws-service-operator
          image: awsserviceoperator/aws-service-operator:v0.0.1-alpha4
          imagePullPolicy: Always
          command:
            - /bin/sh
          args:
          - "-c"
          - export http_proxy=${HOST_IP}:8181; /usr/local/bin/aws-service-operator server --cluster-name=<CLUSTER_NAME> --region=<REGION> --account-id=<ACCOUNT_ID> --k8s-namespace=<K8S_NAMESPACE>
        env:
          - name: HOST_IP
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: status.hostIP
          - name: no_proxy
            value: "*.amazonaws.com,<KUBE_API_IP>:443"
```

Compared to the Deployment definition from [aws-service-operator/configs/aws-service-operator.yaml](https://github.com/awslabs/aws-service-operator/blob/master/configs/aws-service-operator.yaml), this adds the http_proxy and no_proxy environment variables.

Because we use the IP address of the OpenShift node to access the `kube2iam` pod, we cannot set http_proxy in the `env` list, but use a shell command instead.

The value for the no_proxy environment variable is specific to the application. `kube2iam` only allows proxy connections to 169.254.169.254. All other hostnames or IP addresses that the application connects to through HTTP or HTTPS need to be listed in the no_proxy variable.

For example, the aws-service-operator needs access to various AWS APIs and the Kubernetes API. The Kubernetes API listens on the first IP address in the OpenShift service network. If `172.31.0.0/16` is the OpenShift cluster service network, KUBE_API_IP is `172.31.0.1`.

### Debug

By using the --debug flag you can enable some extra features making debugging easier:

* `/debug/store` endpoint enabled to dump knowledge of namespaces and role association.

### Base ARN auto discovery

By using the `--auto-discover-base-arn` flag, kube2iam will auto discover the base ARN via the EC2 metadata service.

### Using ec2 instance role as default role

By using the `--auto-discover-default-role` flag, kube2iam will auto discover the base ARN and the IAM role attached to
the instance and use it as the fallback role to use when annotation is not set.

### AWS STS Endpoint and Regions

STS is a unique service in that it is actually considered a global service that defaults to endpoint at **https://sts.amazonaws.com**, regardless of your region setting. However, unlike other global services (e.g. CloudFront, IAM), STS also has regional endpoints which can only be explicitly used programatically. The use of a regional sts endpoint can reduce the latency for STS requests.

`kube2iam` supports the use of STS regional endpoints by using the `--use-regional-sts-endpoint` flag as well as by setting the appropriate `AWS_REGION` environment variable in your daemonset environment. With these two settings configured, `kube2iam` will use the STS api endpoint for that region. If you enable debug level logging, the sts endpoint used to retrieve credentials will be logged.

### Metrics

`kube2iam` exports a number of [Prometheus](https://github.com/prometheus/prometheus) metrics to assist with monitoring
the system's performance. By default, these are exported at the `/metrics` HTTP endpoint on the
application server port (specified by `--app-port`). This does not always make sense, as anything with access to the
application server port can assume roles via `kube2iam`. To mitigate this use the `--metrics-port` argument to specify
a different port that will host the `/metrics` endpoint.

All of the exported metrics are prefixed with `kube2iam_`. See the [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/getting_started/)
for more information on how to get up and running with Prometheus.

### Options

By default, `kube2iam` will use the in-cluster method to connect to the kubernetes master, and use the
`iam.amazonaws.com/role` annotation to retrieve the role for the container. Either set the `base-role-arn` option to
apply to all roles and only pass the role name in the `iam.amazonaws.com/role` annotation, otherwise pass the full role
ARN in the annotation.

```bash
$ kube2iam --help
Usage of kube2iam:
      --api-server string                     Endpoint for the api server
      --api-token string                      Token to authenticate with the api server
      --app-port string                       Kube2iam server http port (default "8181")
      --auto-discover-base-arn                Queries EC2 Metadata to determine the base ARN
      --auto-discover-default-role            Queries EC2 Metadata to determine the default Iam Role and base ARN, cannot be used with --default-role, overwrites any previous setting for --base-role-arn
      --backoff-max-elapsed-time duration     Max elapsed time for backoff when querying for role. (default 2s)
      --backoff-max-interval duration         Max interval for backoff when querying for role. (default 1s)
      --base-role-arn string                  Base role ARN
      --iam-role-session-ttl                  Length of session when assuming the roles (default 15m)
      --debug                                 Enable debug features
      --default-role string                   Fallback role to use when annotation is not set
      --host-interface string                 Host interface for proxying AWS metadata (default "docker0")
      --host-ip string                        IP address of host
      --iam-role-key string                   Pod annotation key used to retrieve the IAM role (default "iam.amazonaws.com/role")
      --iam-external-id string                Pod annotation key used to retrieve the IAM ExternalId (default "iam.amazonaws.com/external-id")
      --insecure                              Kubernetes server should be accessed without verifying the TLS. Testing only
      --iptables                              Add iptables rule (also requires --host-ip)
      --log-format string                     Log format (text/json) (default "text")
      --log-level string                      Log level (default "info")
      --metadata-addr string                  Address for the ec2 metadata (default "169.254.169.254")
      --metrics-port string                   Metrics server http port (default: same as kube2iam server port) (default "8181")
      --namespace-key string                  Namespace annotation key used to retrieve the IAM roles allowed (value in annotation should be json array) (default "iam.amazonaws.com/allowed-roles")
      --cache-resync-period                   Refresh interval for pod and namespace caches
      --resolve-duplicate-cache-ips           Queries the k8s api server to find the source of truth when the pod cache contains multiple pods with the same IP
      --namespace-restriction-format string   Namespace Restriction Format (glob/regexp) (default "glob")
      --namespace-restrictions                Enable namespace restrictions
      --node string                           Name of the node where kube2iam is running
      --use-regional-sts-endpoint             use the regional sts endpoint if AWS_REGION is set
      --verbose                               Verbose
      --version                               Print the version and exits
```

## Development loop

* Use [minikube](https://github.com/kubernetes/minikube) to run cluster locally
* Build and push dev image to docker hub: `make docker-dev DOCKER_REPO=<your docker hub username>`
* Update `deployment.yaml` as needed
* Deploy to local kubernetes cluster: `kubectl create -f deployment.yaml` or
  `kubectl delete -f deployment.yaml && kubectl create -f deployment.yaml`
* Expose as service: `kubectl expose deployment kube2iam --type=NodePort`
* Retrieve the services url: `minikube service kube2iam --url`
* Test your changes e.g. `curl -is $(minikube service kube2iam --url)/healthz`

# Author

Jerome Touffe-Blin, [@jtblin](https://twitter.com/jtblin), [About me](http://about.me/jtblin)

# License

kube2iam is copyright 2020 Jerome Touffe-Blin and contributors.
It is licensed under the BSD license. See the included LICENSE file for details.
