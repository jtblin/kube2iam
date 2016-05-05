# kube2iam

Provide IAM credentials to containers running inside a kubernetes cluster based on annotations.

## iptables

To prevent containers to directly access the ec2 metadata API and gain unwanted access to AWS resources
(escalation of privileges), the traffic to `169.254.169.254` must be proxied for docker containers.

    iptables -t nat -A OUTPUT -p tcp -d 169.254.169.254 -i docker0 -j DNAT --to-destination 127.0.0.1:8000