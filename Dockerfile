FROM alpine:3.5

RUN apk --no-cache add \
    ca-certificates \
    iptables

ADD build/bin/linux/kube2iam /bin/kube2iam

ENTRYPOINT ["kube2iam"]
