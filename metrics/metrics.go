package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/jtblin/kube2iam/version"
)

const (
	namespace = "kube2iam"

	// IamSuccessCode is the code used for metrics when an IAM request succeeds.
	IamSuccessCode = "Success"
	// IamUnknownFailCode is the code used for metrics when an IAM request fails with an error not reported by AWS.
	IamUnknownFailCode = "UnknownError"
)

var (
	// IamRequestSec tracks timing of IAM requests.
	IamRequestSec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "iam",
			Name:      "request_duration_seconds",
			Help:      "Time taken to complete IAM request in seconds.",

			Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
		},
		[]string{
			// The HTTP status code AWS returned
			"code",
			// The arn of the IAM role being requested
			"role_arn",
		},
	)

	// IamCacheHitCount tracks total number of IAM cache hits. Cache misses can be
	// calculated by looking at the total number of IAM requests for the same role_arn.
	IamCacheHitCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "iam",
			Name:      "cache_hits_total",
			Help:      "Total number of IAM cache hits.",
		},
		[]string{
			// The arn of the IAM role being requested
			"role_arn",
		},
	)

	// K8sAPIDupReqCount tracks total number of K8s Api requests performed when duplicated pods are identified in the cache.
	K8sAPIDupReqCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "iam",
			Name:      "k8s_dup_req_count",
			Help:      "Total number of K8s API requests performed when duplicated pods are identified in the cache.",
		},
	)

	// K8sAPIDupReqSuccesCount tracks total number of times we successfully retrieve the pod from the K8s Api.
	K8sAPIDupReqSuccesCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "iam",
			Name:      "k8s_dup_req_success_count",
			Help:      "Total number of times we successfully retrieve the pod from the K8s API.",
		},
	)

	// PodNotFoundInCache tracks total number of times we don't have the pod info in the cache.
	PodNotFoundInCache = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "iam",
			Name:      "k8s_pod_not_in_pod_cache",
			Help:      "Total number of times we don't have the pod info in the cache.",
		},
	)

	// HTTPRequestSec tracks timing of served HTTP requests.
	HTTPRequestSec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Time taken for kube2iam to serve HTTP request.",

			Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
		},
		[]string{
			// The HTTP status code kube2iam returned
			"code",
			// The HTTP method being served
			"method",
			// The name of the handler being served
			"handler",
		},
	)

	// HealthcheckStatus reports the current healthcheck status of kube2iam.
	HealthcheckStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "healthcheck",
			Name:      "status",
			Help:      "The healthcheck status at scrape time. A value of 1 means it is passing, 0 means it is failing.",
		},
	)

	// Info reports various static information about the running kube2iam binary.
	Info = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "info",
			Help:      "Informational labels about the kube2iam process.",
		},
		[]string{
			// The version of kube2iam running
			"version",
			// The build date of the kube2iam version
			"build_date",
			// The commit hash of the kube2iam version
			"commit_hash",
		},
	)
)

func init() {
	prometheus.MustRegister(IamRequestSec)
	prometheus.MustRegister(IamCacheHitCount)
	prometheus.MustRegister(K8sAPIDupReqCount)
	prometheus.MustRegister(K8sAPIDupReqSuccesCount)
	prometheus.MustRegister(PodNotFoundInCache)
	prometheus.MustRegister(HTTPRequestSec)
	prometheus.MustRegister(HealthcheckStatus)
	prometheus.MustRegister(Info)

	for _, val := range []string{IamSuccessCode, IamUnknownFailCode} {
		IamRequestSec.WithLabelValues(val, "")
	}
	Info.WithLabelValues(version.Version, version.BuildDate, version.GitCommit).Set(1)
}

// StartMetricsServer registers a prometheus /metrics handler and starts a HTTP server
// listening on the provided port to service it.
func StartMetricsServer(metricsPort string) {
	r := mux.NewRouter()
	r.Handle("/metrics", GetHandler())

	go func() {
		if err := http.ListenAndServe(":"+metricsPort, r); err != nil {
			log.Fatalf("Error creating metrics http server: %+v", err)
		}
	}()
}

// GetHandler creates a prometheus HTTP handler that will serve metrics.
func GetHandler() http.Handler {
	return promhttp.Handler()
}

// NewFunctionTimer creates a new timer for a generic function that can be observed to time the duration of the handler.
// The metric is labeled with the values produced by the lvsProducer to allow for late binding of label values.
// If provided, the timer value is stored in storeValue to allow callers access to the reported value.
func NewFunctionTimer(histVec *prometheus.HistogramVec, lvsFn func() []string, storeValue *float64) *prometheus.Timer {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		if storeValue != nil {
			*storeValue = v
		}
		histVec.WithLabelValues(lvsFn()...).Observe(v)
	}))
	return timer
}
