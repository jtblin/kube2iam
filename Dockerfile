FROM golang:1.14.0 AS BUILDER
WORKDIR /go/src/github.com/jtblin/kube2iam
ENV ARCH=linux
ENV CGO_ENABLED=0
COPY . ./
RUN make setup && make build

FROM alpine:3.11.3
RUN apk --no-cache add \
    ca-certificates \
    iptables
COPY --from=BUILDER /go/src/github.com/jtblin/kube2iam/build/bin/linux/kube2iam /bin/kube2iam
ENTRYPOINT ["kube2iam"]
