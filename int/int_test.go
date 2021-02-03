package int_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/route-monitor-operator/api/v1alpha1"
	. "github.com/openshift/route-monitor-operator/int"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Integrationtests", func() {
	var (
		i *Integration
	)
	BeforeSuite(func() {
		var err error
		i, err = NewIntegration()
		Expect(err).NotTo(HaveOccurred())
	})
	AfterSuite(func() {
		i.Shutdown()
	})

	Context("ClusterUrlMonitor creation", func() {
		var (
			resourceIdentity           types.NamespacedName
			clusterUrlMonitor          v1alpha1.ClusterUrlMonitor
			expectedServiceMonitorName types.NamespacedName
		)
		BeforeEach(func() {
			resourceIdentity = types.NamespacedName{
				Name:      "fake-url-monitor",
				Namespace: "default",
			}

			// clean possible leftovers
			err := i.RemoveClusterUrlMonitor(resourceIdentity)
			Expect(err).NotTo(HaveOccurred())

			expectedServiceMonitorName = resourceIdentity
			clusterUrlMonitor = v1alpha1.ClusterUrlMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: resourceIdentity.Namespace,
					Name:      resourceIdentity.Name,
				},
				Spec: v1alpha1.ClusterUrlMonitorSpec{
					Prefix: "fake-prefix.",
					Port:   "1234",
					Suffix: "/fake-suffix",
				},
			}
		})
		AfterEach(func() {
			err := i.RemoveClusterUrlMonitor(resourceIdentity)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the ClusterUrlMonitor does not exist", func() {
			It("creates a ServiceMonitor within 20 seconds", func() {
				err := i.Client.Create(context.TODO(), &clusterUrlMonitor)
				Expect(err).NotTo(HaveOccurred())

				serviceMonitor, err := i.WaitForServiceMonitor(expectedServiceMonitorName, 20)
				Expect(err).NotTo(HaveOccurred())

				Expect(serviceMonitor.Name).To(Equal(expectedServiceMonitorName.Name))
				Expect(serviceMonitor.Namespace).To(Equal(expectedServiceMonitorName.Namespace))

				clusterConfig := configv1.Ingress{}
				err = i.Client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, &clusterConfig)
				Expect(err).NotTo(HaveOccurred())
				spec := clusterUrlMonitor.Spec
				expectedUrl := spec.ConstructUrl(clusterConfig.Spec.Domain)
				Expect(serviceMonitor.Spec.Endpoints).To(HaveLen(1))
				Expect(serviceMonitor.Spec.Endpoints[0].Params["target"]).To(HaveLen(1))
				Expect(serviceMonitor.Spec.Endpoints[0].Params["target"][0]).To(Equal(expectedUrl))

				updatedClusterUrlMonitor := v1alpha1.ClusterUrlMonitor{}
				err = i.Client.Get(context.TODO(), resourceIdentity, &updatedClusterUrlMonitor)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedClusterUrlMonitor.Status.ServiceMonitorRef.Name).To(Equal(serviceMonitor.Name))
				Expect(updatedClusterUrlMonitor.Status.ServiceMonitorRef.Namespace).To(Equal(serviceMonitor.Namespace))
			})
		})

		When("the ClusterUrlMonitor is deleted", func() {
			BeforeEach(func() {
				err := i.Client.Create(context.TODO(), &clusterUrlMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForServiceMonitor(expectedServiceMonitorName, 20)
				Expect(err).NotTo(HaveOccurred())

				err = i.RemoveClusterUrlMonitor(resourceIdentity)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the ServiceMonitor as well within 20 seconds", func() {
				serviceMonitor := monitoringv1.ServiceMonitor{}
				err := i.Client.Get(context.TODO(), expectedServiceMonitorName, &serviceMonitor)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Context("RouteMonitor creation", func() {
		var (
			resourceIdentity          types.NamespacedName
			percentile                string
			routeMonitor              v1alpha1.RouteMonitor
			expectedDependentResource types.NamespacedName
		)
		BeforeEach(func() {
			resourceIdentity = types.NamespacedName{
				Name:      "fake-route-monitor",
				Namespace: "default",
			}
			percentile = "0.9995"
			err := i.RemoveRouteMonitor(resourceIdentity)
			expectedDependentResource = types.NamespacedName{Name: resourceIdentity.Name, Namespace: resourceIdentity.Namespace}
			Expect(err).NotTo(HaveOccurred())
			routeMonitor = v1alpha1.RouteMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: resourceIdentity.Namespace,
					Name:      resourceIdentity.Name,
				},
				Spec: v1alpha1.RouteMonitorSpec{
					Slo: v1alpha1.SloSpec{
						TargetAvailabilityPercentile: percentile,
					},
					Route: v1alpha1.RouteMonitorRouteSpec{
						Name:      "console",
						Namespace: "openshift-console",
					},
				},
			}
		})
		AfterEach(func() {
			err := i.RemoveRouteMonitor(resourceIdentity)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the RouteMonitor does not exist", func() {
			It("creates a ServiceMonitor within 20 seconds", func() {
				err := i.Client.Create(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				serviceMonitor, err := i.WaitForServiceMonitor(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceMonitor.Name).To(Equal(expectedDependentResource.Name))
				Expect(serviceMonitor.Namespace).To(Equal(expectedDependentResource.Namespace))

				prometheusRule, err := i.WaitForPrometheusRule(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())
				Expect(prometheusRule.Name).To(Equal(expectedDependentResource.Name))
				Expect(prometheusRule.Namespace).To(Equal(expectedDependentResource.Namespace))

				updatedRouteMonitor := v1alpha1.RouteMonitor{}
				err = i.Client.Get(context.TODO(), resourceIdentity, &updatedRouteMonitor)
				Expect(err).NotTo(HaveOccurred())

				Expect(updatedRouteMonitor.Status.PrometheusRuleRef.Name).To(Equal(prometheusRule.Name))
				Expect(updatedRouteMonitor.Status.PrometheusRuleRef.Namespace).To(Equal(prometheusRule.Namespace))

				Expect(updatedRouteMonitor.Status.ServiceMonitorRef.Name).To(Equal(serviceMonitor.Name))
				Expect(updatedRouteMonitor.Status.ServiceMonitorRef.Namespace).To(Equal(serviceMonitor.Namespace))
			})
		})

		When("slo is modified in-flight", func() {
			It("modifies the status resource ", func() {
				err := i.Client.Create(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForServiceMonitor(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForPrometheusRule(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				updatedRouteMonitor := v1alpha1.RouteMonitor{}
				err = i.Client.Get(context.TODO(), resourceIdentity, &updatedRouteMonitor)
				Expect(err).NotTo(HaveOccurred())

				inflightRouteMonitor := updatedRouteMonitor.DeepCopy()
				veryVeryAvailablePercentile := "0.9999995"
				inflightRouteMonitor.Spec.Slo.TargetAvailabilityPercentile = veryVeryAvailablePercentile
				err = i.Client.Update(context.TODO(), inflightRouteMonitor)
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(10 * time.Second)

				newestRouteMonitor := v1alpha1.RouteMonitor{}
				err = i.Client.Get(context.TODO(), resourceIdentity, &newestRouteMonitor)
				Expect(err).NotTo(HaveOccurred())

				Expect(newestRouteMonitor.Status.CurrentTargetAvailabilityPercentile).To(Equal(veryVeryAvailablePercentile))
			})
		})

		When("slo is created/deleted in-flight", func() {
			It("deletes the prometheusRule ", func() {
				// clean SLO before creating
				routeMonitor.Spec.Slo = v1alpha1.SloSpec{}

				err := i.Client.Create(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForServiceMonitor(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				// Add slo
				err = i.Client.Get(context.TODO(), resourceIdentity, &routeMonitor)
				Expect(err).NotTo(HaveOccurred())
				routeMonitor.Spec.Slo.TargetAvailabilityPercentile = percentile
				err = i.Client.Update(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForPrometheusRule(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				// Remove slo
				err = i.Client.Get(context.TODO(), resourceIdentity, &routeMonitor)
				Expect(err).NotTo(HaveOccurred())
				routeMonitor.Spec.Slo = v1alpha1.SloSpec{}
				err = i.Client.Update(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForPrometheusRuleToDelete(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				err = i.Client.Get(context.TODO(), resourceIdentity, &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				Expect(routeMonitor.Status.PrometheusRuleRef).To(Equal(*new(v1alpha1.NamespacedName)))
				Expect(routeMonitor.Status.CurrentTargetAvailabilityPercentile).To(Equal(""))

			})
		})
		When("the RouteMonitor is deleted", func() {
			BeforeEach(func() {
				err := i.Client.Create(context.TODO(), &routeMonitor)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForServiceMonitor(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				_, err = i.WaitForPrometheusRule(expectedDependentResource, 20)
				Expect(err).NotTo(HaveOccurred())

				err = i.RemoveRouteMonitor(resourceIdentity)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the Dependant resources as well", func() {
				serviceMonitor := monitoringv1.ServiceMonitor{}
				err := i.Client.Get(context.TODO(), expectedDependentResource, &serviceMonitor)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())

				prometheusRule := monitoringv1.PrometheusRule{}
				err = i.Client.Get(context.TODO(), expectedDependentResource, &prometheusRule)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})
