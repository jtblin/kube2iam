ARG target
FROM $target/alpine

COPY qemu-* .dummy /usr/bin/

RUN apk --no-cache add ca-certificates iptables

COPY kube2iam /bin/kube2iam

ENTRYPOINT ["kube2iam"]
