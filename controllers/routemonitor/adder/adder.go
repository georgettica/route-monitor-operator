package adder

import (
	"github.com/go-logr/logr"

	"context"

	// k8s packages
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//api's used
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	//local packages
	"github.com/openshift/route-monitor-operator/api/v1alpha1"
	"github.com/openshift/route-monitor-operator/controllers/routemonitor"
	"github.com/openshift/route-monitor-operator/pkg/consts"
	routemonitorconst "github.com/openshift/route-monitor-operator/pkg/consts"
	customerrors "github.com/openshift/route-monitor-operator/pkg/util/errors"
	"github.com/openshift/route-monitor-operator/pkg/util/finalizer"
	utilfinalizer "github.com/openshift/route-monitor-operator/pkg/util/finalizer"
	utilreconcile "github.com/openshift/route-monitor-operator/pkg/util/reconcile"
	"github.com/openshift/route-monitor-operator/pkg/util/templates"
)

// RouteMonitorAdder hold additional actions that supplement the Reconcile
type RouteMonitorAdder struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func New(r routemonitor.RouteMonitorReconciler) *RouteMonitorAdder {
	return &RouteMonitorAdder{
		Client: r.Client,
		Log:    r.Log,
		Scheme: r.Scheme,
	}
}

func (r *RouteMonitorAdder) EnsureServiceMonitorResourceExists(ctx context.Context, routeMonitor v1alpha1.RouteMonitor) (utilreconcile.Result, error) {
	// Was the RouteURL populated by a previous step?
	if routeMonitor.Status.RouteURL == "" {
		return utilreconcile.RequeueReconcileWith(customerrors.NoHost)
	}

	if !finalizer.HasFinalizer(&routeMonitor, consts.FinalizerKey) {
		// If the routeMonitor doesn't have a finalizer, add it
		utilfinalizer.Add(&routeMonitor, routemonitorconst.FinalizerKey)
		if err := r.Update(ctx, &routeMonitor); err != nil {
			return utilreconcile.RequeueReconcileWith(err)
		}
		return utilreconcile.StopReconcile()
	}

	namespacedName := types.NamespacedName{Name: routeMonitor.Name, Namespace: routeMonitor.Namespace}
	resource := &monitoringv1.ServiceMonitor{}
	populationFunc := func() monitoringv1.ServiceMonitor {
		return templates.TemplateForServiceMonitorResource(routeMonitor.Status.RouteURL, namespacedName)
	}

	// Does the resource already exist?
	if err := r.Get(ctx, namespacedName, resource); err != nil {
		// If this is an unknown error
		if !k8serrors.IsNotFound(err) {
			// return unexpectedly
			return utilreconcile.RequeueReconcileWith(err)
		}
		// populate the resource with the template
		resource := populationFunc()
		// and create it
		err = r.Create(ctx, &resource)
		if err != nil {
			return utilreconcile.RequeueReconcileWith(err)
		}
	}

	//Update status with serviceMonitorRef
	routeMonitor.Status.ServiceMonitorRef.Namespace = namespacedName.Namespace
	routeMonitor.Status.ServiceMonitorRef.Name = namespacedName.Name
	err := r.Status().Update(ctx, &routeMonitor)
	if err != nil {
		return utilreconcile.RequeueReconcileWith(err)
	}

	return utilreconcile.ContinueReconcile()
}
func (r *RouteMonitorAdder) EnsurePrometheusRuleResourceExists(ctx context.Context, routeMonitor v1alpha1.RouteMonitor) (utilreconcile.Result, error) {
	// Was the RouteURL populated by a previous step?
	if routeMonitor.Status.RouteURL == "" {
		return utilreconcile.RequeueReconcileWith(customerrors.NoHost)
	}

	// Is the SloSpec configured on this CR?
	tstSlo := v1alpha1.SloSpec{}
	if routeMonitor.Spec.Slo == tstSlo {
		return utilreconcile.ContinueReconcile()
	}

	if !routeMonitor.Spec.Slo.IsValid() {
		return utilreconcile.RequeueReconcileWith(customerrors.InvalidSLO)
	}

	if !finalizer.HasFinalizer(&routeMonitor, consts.FinalizerKey) {
		// If the routeMonitor doesn't have a finalizer, add it
		utilfinalizer.Add(&routeMonitor, routemonitorconst.FinalizerKey)
		if err := r.Update(ctx, &routeMonitor); err != nil {
			return utilreconcile.RequeueReconcileWith(err)
		}
		return utilreconcile.StopReconcile()
	}

	namespacedName := types.NamespacedName{Name: routeMonitor.Name, Namespace: routeMonitor.Namespace}
	normalizedPercent, succeed := routeMonitor.Spec.Slo.NormalizeValue()
	if !succeed {
		return utilreconcile.RequeueReconcileWith(customerrors.InvalidSLO)
	}

	resource := &monitoringv1.PrometheusRule{}
	populationFunc := func() monitoringv1.PrometheusRule {
		return templates.TemplateForPrometheusRuleResource(routeMonitor.Status.RouteURL, normalizedPercent, namespacedName)
	}

	// Does the resource already exist?
	if err := r.Get(ctx, namespacedName, resource); err != nil {
		// If this is an unknown error
		if !k8serrors.IsNotFound(err) {
			// return unexpectedly
			return utilreconcile.RequeueReconcileWith(err)
		}
		// populate the resource with the template
		resource := populationFunc()
		// and create it
		err = r.Create(ctx, &resource)
		if err != nil {
			return utilreconcile.RequeueReconcileWith(err)
		}
	}

	desiredPrometheusRuleRef := v1alpha1.NamespacedName{
		Name:      namespacedName.Name,
		Namespace: namespacedName.Namespace,
	}

	if routeMonitor.Status.PrometheusRuleRef != desiredPrometheusRuleRef {
		// Update status with PrometheusRuleRef
		routeMonitor.Status.PrometheusRuleRef = desiredPrometheusRuleRef
		err := r.Status().Update(ctx, &routeMonitor)
		if err != nil {
			return utilreconcile.RequeueReconcileWith(err)
		}
		return utilreconcile.StopReconcile()
	}
	return utilreconcile.ContinueReconcile()
}
