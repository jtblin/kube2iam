FROM alpine:3.10

RUN apk --no-cache add \
    ca-certificates \
    iptables

COPY build/bin/linux/kube2iam /bin/kube2iam

ENTRYPOINT ["kube2iam"]
