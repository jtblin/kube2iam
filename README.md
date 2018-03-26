[![Build Status](https://travis-ci.org/jtblin/kube2iam.svg?branch=master)](https://travis-ci.org/jtblin/kube2iam)
[![GitHub tag](https://img.shields.io/github/tag/jtblin/kube2iam.svg?maxAge=86400)](https://github.com/atlassian/gostatsd)
[![Docker Pulls](https://img.shields.io/docker/pulls/jtblin/kube2iam.svg)]()
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
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube2iam
  labels:
    app: kube2iam
spec:
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

Note that the interface `--in-interface` above or using the `--host-interface` cli flag may be
different than `docker0` depending on which virtual network you use e.g.

* for Calico, use `cali+` (the interface name is something like cali1234567890
* for kops (on kubenet), use `cbr0`
* for CNI, use `cni0`
* for weave use `weave`
* for flannel use `cni0`
* for [kube-router](https://github.com/cloudnativelabs/kube-router) use `kube-bridge`

```yaml
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube2iam
  labels:
    app: kube2iam
spec:
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

### kubernetes annotation

Add an `iam.amazonaws.com/role` annotation to your pods with the role that you want to assume for this pod.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: aws-cli
  labels:
    name: aws-cli
  annotations:
    iam.amazonaws.com/role: role-arn
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
apiVersion: extensions/v1beta1
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
apiVersion: batch/v2alpha1
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
_Note:_ You can also use glob-based matching for namespace restrictions, which works nicely with the path-based namespacing supported for AWS IAM roles. 

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
```

You will notice this lives in the kube-system namespace to allow for easier seperation between system services and other services.

Here is what a kube2iam daemonset yaml might look like.

```yaml
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube2iam
  namespace: kube-system
  labels:
    app: kube2iam
spec:
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


### Debug

By using the --debug flag you can enable some extra features making debugging easier:

- `/debug/store` endpoint enabled to dump knowledge of namespaces and role association.

### Base ARN auto discovery

By using the `--auto-discover-base-arn` flag, kube2iam will auto discover the base ARN via the EC2 metadata service.

### Using ec2 instance role as default role

By using the `--auto-discover-default-role` flag, kube2iam will auto discover the base ARN and the IAM role attached to
the instance and use it as the fallback role to use when annotation is not set.

### Options

By default, `kube2iam` will use the in-cluster method to connect to the kubernetes master, and use the
`iam.amazonaws.com/role` annotation to retrieve the role for the container. Either set the `base-role-arn` option to
apply to all roles and only pass the role name in the `iam.amazonaws.com/role` annotation, otherwise pass the full role
ARN in the annotation.

```bash
$ kube2iam --help
Usage of ./build/bin/darwin/kube2iam:
      --api-server string                   Endpoint for the api server
      --api-token string                    Token to authenticate with the api server
      --app-port string                     Http port (default "8181")
      --auto-discover-base-arn              Queries EC2 Metadata to determine the base ARN
      --auto-discover-default-role          Queries EC2 Metadata to determine the default Iam Role and base ARN, cannot be used with --default-role, overwrites any previous setting for --base-role-arn
      --backoff-max-elapsed-time duration   Max elapsed time for backoff when querying for role. (default 2s)
      --backoff-max-interval duration       Max interval for backoff when querying for role. (default 1s)
      --base-role-arn string                Base role ARN
      --debug                               Enable debug features
      --default-role string                 Fallback role to use when annotation is not set
      --host-interface string               Host interface for proxying AWS metadata (default "docker0")
      --host-ip string                      IP address of host
      --iam-role-key string                 Pod annotation key used to retrieve the IAM role (default "iam.amazonaws.com/role")
      --insecure                            Kubernetes server should be accessed without verifying the TLS. Testing only
      --iptables                            Add iptables rule (also requires --host-ip)
      --log-level string                    Log level (default "info")
      --metadata-addr string                Address for the ec2 metadata (default "169.254.169.254")
      --namespace-key string                Namespace annotation key used to retrieve the IAM roles allowed (value in annotation should be json array) (default "iam.amazonaws.com/allowed-roles")
      --namespace-restrictions              Enable namespace restrictions
      --verbose                             Verbose
      --version                             Print the version and exits
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

kube2iam is copyright 2017 Jerome Touffe-Blin and contributors.
It is licensed under the BSD license. See the included LICENSE file for details.
