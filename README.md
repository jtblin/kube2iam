# kube2iam

Provide IAM credentials to containers running inside a kubernetes cluster based on annotations.

## Context

Traditionally in AWS, service level isolation is done using IAM roles. IAM roles are attributed through instance
profiles and are accessible by services through the transparent usage by the aws-sdk of the ec2 metadata API.
When using the aws-sdk, a call is made to the ec2 metadata API which provides temporary credentials
that are then used to make calls to the AWS service.

## Problem statement

The problem is that in a multi-tenanted containers based world, multiple containers will be sharing the underlying
nodes. Given containers will share the same underlying nodes, providing access to AWS
resources via IAM roles would mean that one needs to create an IAM role which is a union of all
IAM roles. This is not acceptable from a security perspective.

## Solution

The solution is to redirect the traffic that is going to the ec2 metadata API for docker containers to a container
running on each instance, make a call to the AWS API to retrieve temporary credentials and return these to the caller.
Other calls will be proxied to the ec2 metadata API. This container will need to run with host networking enabled
so that it can call the ec2 metadata API itself.

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

The roles that will be assumed must have a Trust Relationship which allows them to be assumed by the kubernetes worker role.
See this [StackOverflow post](http://stackoverflow.com/a/33850060) for more details.

```
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

```
---
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

To prevent containers from directly accessing the ec2 metadata API and gaining unwanted access to AWS resources,
the traffic to `169.254.169.254` must be proxied for docker containers.

```
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

```
---
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

```
---
apiVersion: v1
kind: Pod
metadata:
  name: aws-cli
  labels:
    name: aws-cli
  annotations:
    iam.amazonaws.com/role: role-name
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

### Options

By default, `kube2iam` will use the in-cluster method to connect to the kubernetes master, and use the `iam.amazonaws.com/role`
annotation to retrieve the role for the container. Either set the `base-role-arn` option to apply to all roles
and only pass the role name in the `iam.amazonaws.com/role` annotation, otherwise pass the full role ARN in the annotation.

```
$ kube2iam --help
Usage of kube2iam:
      --api-server string       Endpoint for the api server
      --api-token string        Token to authenticate with the api server
      --app-port string         Http port (default "8181")
      --base-role-arn string    Base role ARN
      --default-role string     Fallback role to use when annotation is not set
      --host-interface string   Host interface for proxying AWS metadata (default "docker0")
      --host-ip string          IP address of host
      --iam-role-key string     Pod annotation key used to retrieve the IAM role (default "iam.amazonaws.com/role")
      --insecure                Kubernetes server should be accessed without verifying the TLS. Testing only
      --iptables                Add iptables rule (also requires --host-ip)
      --metadata-addr string    Address for the ec2 metadata (default "169.254.169.254")
      --verbose                 Verbose
      --version                 Print the version and exits

```

# Author

Jerome Touffe-Blin, [@jtblin](https://twitter.com/jtblin), [About me](http://about.me/jtblin)

# License

kube2iam is copyright 2016 Jerome Touffe-Blin and contributors.
It is licensed under the BSD license. See the included LICENSE file for details.
