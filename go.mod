module github.com/openshift/route-monitor-operator

go 1.13

require (
	github.com/go-logr/logr v0.3.0
	github.com/golang/mock v1.4.4
	github.com/google/gofuzz v1.2.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/openshift/api v0.0.0-20200917102736-0a191b5b9bb0
	github.com/operator-framework/operator-sdk v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.1
	github.com/prometheus/common v0.10.0
	github.com/stretchr/testify v1.6.1 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/inf.v0 v0.9.1
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
