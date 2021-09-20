module github.com/jtblin/kube2iam

go 1.16

require (
	github.com/aws/aws-sdk-go v1.40.45
	github.com/cenk/backoff v2.2.1+incompatible
	github.com/coreos/go-iptables v0.6.0
	github.com/gorilla/mux v1.8.0
	github.com/karlseguin/ccache v2.0.3+incompatible
	github.com/prometheus/client_golang v1.11.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
)

require (
	github.com/karlseguin/expect v1.0.8 // indirect
	github.com/mattn/goveralls v0.0.10 // indirect
)
