package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tunnelv1alpha1 "github.com/bovf/pangolin-operator/api/v1alpha1"
)

const (
	BindingFinalizerName = "binding.pangolin.io/finalizer"
)

// PangolinBindingReconciler reconciles a PangolinBinding object
type PangolinBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels,verbs=get;list;watch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PangolinBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PangolinBinding instance
	binding := &tunnelv1alpha1.PangolinBinding{}
	err := r.Get(ctx, req.NamespacedName, binding)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PangolinBinding resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PangolinBinding")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if binding.DeletionTimestamp != nil {
		return r.handleBindingDeletion(ctx, binding)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(binding, BindingFinalizerName) {
		controllerutil.AddFinalizer(binding, BindingFinalizerName)
		return ctrl.Result{}, r.Update(ctx, binding)
	}

	// Get the referenced service
	service, err := r.getServiceForBinding(ctx, binding)
	if err != nil {
		logger.Error(err, "Failed to get referenced service")
		return r.updateBindingStatus(ctx, binding, "Error", err.Error())
	}

	// Get the referenced organization
	org, err := r.getOrganizationForBinding(ctx, binding)
	if err != nil {
		logger.Error(err, "Failed to get referenced organization")
		return r.updateBindingStatus(ctx, binding, "Error", err.Error())
	}

	// Check if organization is ready
	if org.Status.Status != "Ready" {
		logger.Info("Organization not ready yet, waiting", "organization", org.Name)
		return r.updateBindingStatus(ctx, binding, "Waiting", "Waiting for organization to be ready")
	}

	// Get or create tunnel
	tunnel, err := r.ensureTunnelForBinding(ctx, binding, org)
	if err != nil {
		logger.Error(err, "Failed to ensure tunnel for binding")
		return r.updateBindingStatus(ctx, binding, "Error", err.Error())
	}

	// Check if tunnel is ready
	if tunnel.Status.Status != "Ready" {
		logger.Info("Tunnel not ready yet, waiting", "tunnel", tunnel.Name)
		return r.updateBindingStatus(ctx, binding, "Waiting", "Waiting for tunnel to be ready")
	}

	// Create or update PangolinResource
	resource, err := r.reconcileResourceForBinding(ctx, binding, tunnel, service)
	if err != nil {
		logger.Error(err, "Failed to reconcile resource for binding")
		return r.updateBindingStatus(ctx, binding, "Error", err.Error())
	}

	// Check if resource is ready
	if resource.Status.Status != "Ready" {
		logger.Info("Resource not ready yet, waiting", "resource", resource.Name)
		return r.updateBindingStatus(ctx, binding, "Waiting", "Waiting for resource to be ready")
	}

	// Update service endpoints if auto-update is enabled
	if binding.Spec.AutoUpdateTargets == nil || *binding.Spec.AutoUpdateTargets {
		err = r.updateServiceEndpoints(ctx, binding, service)
		if err != nil {
			logger.Error(err, "Failed to update service endpoints")
			// Don't fail the reconciliation for this
		}
	}

	// Update binding status from resource
	binding.Status.GeneratedResourceName = resource.Name
	binding.Status.URL = resource.Status.URL
	binding.Status.ProxyEndpoint = resource.Status.ProxyEndpoint

	return r.updateBindingStatus(ctx, binding, "Ready", "Binding is ready")
}

// getServiceForBinding retrieves the service referenced by the binding
func (r *PangolinBindingReconciler) getServiceForBinding(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: binding.Spec.ServiceRef.Namespace,
		Name:      binding.Spec.ServiceRef.Name,
	}, service)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", binding.Spec.ServiceRef.Namespace, binding.Spec.ServiceRef.Name, err)
	}
	return service, nil
}

// getOrganizationForBinding retrieves the organization referenced by the binding
func (r *PangolinBindingReconciler) getOrganizationForBinding(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding) (*tunnelv1alpha1.PangolinOrganization, error) {
	org := &tunnelv1alpha1.PangolinOrganization{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: binding.Namespace,
		Name:      binding.Spec.OrganizationRef.Name,
	}, org)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", binding.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}

// ensureTunnelForBinding gets existing tunnel or creates default one
func (r *PangolinBindingReconciler) ensureTunnelForBinding(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding, org *tunnelv1alpha1.PangolinOrganization) (*tunnelv1alpha1.PangolinTunnel, error) {
	if binding.Spec.TunnelRef != nil {
		// Use specific tunnel
		tunnel := &tunnelv1alpha1.PangolinTunnel{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: binding.Namespace,
			Name:      binding.Spec.TunnelRef.Name,
		}, tunnel)
		if err != nil {
			return nil, fmt.Errorf("failed to get tunnel %s: %w", binding.Spec.TunnelRef.Name, err)
		}
		return tunnel, nil
	}

	// TODO: Create or find default tunnel for the organization
	// For now, return an error indicating tunnel ref is required
	return nil, fmt.Errorf("tunnelRef is required - automatic tunnel creation not yet implemented")
}

// reconcileResourceForBinding creates or updates the PangolinResource
func (r *PangolinBindingReconciler) reconcileResourceForBinding(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding, tunnel *tunnelv1alpha1.PangolinTunnel, service *corev1.Service) (*tunnelv1alpha1.PangolinResource, error) {

	// Generate resource name
	resourceName := fmt.Sprintf("%s-binding", binding.Name)

	// Check if resource already exists
	resource := &tunnelv1alpha1.PangolinResource{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: binding.Namespace,
		Name:      resourceName,
	}, resource)

	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	if errors.IsNotFound(err) {
		// Create new resource
		resource = &tunnelv1alpha1.PangolinResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: binding.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: binding.APIVersion,
						Kind:       binding.Kind,
						Name:       binding.Name,
						UID:        binding.UID,
						Controller: &[]bool{true}[0],
					},
				},
			},
			Spec: tunnelv1alpha1.PangolinResourceSpec{
				TunnelRef: tunnelv1alpha1.LocalObjectReference{
					Name: tunnel.Name,
				},
				Name:     fmt.Sprintf("%s-%s", binding.Spec.ServiceRef.Name, binding.Spec.Protocol),
				Protocol: binding.Spec.Protocol,
				Target: tunnelv1alpha1.TargetConfig{
					IP:     service.Spec.ClusterIP,
					Port:   binding.Spec.ServicePort,
					Method: "http", // TODO: derive from protocol
				},
			},
		}

		// Set HTTP/Proxy config based on protocol
		if binding.Spec.Protocol == "http" && binding.Spec.HTTPConfig != nil {
			resource.Spec.HTTPConfig = binding.Spec.HTTPConfig
		} else if binding.Spec.ProxyConfig != nil {
			resource.Spec.ProxyConfig = binding.Spec.ProxyConfig
		}

		err = r.Create(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource: %w", err)
		}
	}

	return resource, nil
}

// updateServiceEndpoints updates the target endpoints based on service endpoints
func (r *PangolinBindingReconciler) updateServiceEndpoints(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding, service *corev1.Service) error {
	// Get endpoints for the service
	endpoints := &corev1.Endpoints{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}, endpoints)
	if err != nil {
		return fmt.Errorf("failed to get endpoints: %w", err)
	}

	// Extract endpoint addresses
	var endpointAddresses []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			endpointAddresses = append(endpointAddresses, address.IP)
		}
	}

	// Update binding status with current endpoints
	binding.Status.ServiceEndpoints = endpointAddresses

	// TODO: Update PangolinResource target to use multiple endpoints if supported

	return nil
}

// updateBindingStatus updates the binding status with proper conditions
func (r *PangolinBindingReconciler) updateBindingStatus(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding, status, message string) (ctrl.Result, error) {
	binding.Status.Status = status
	binding.Status.ObservedGeneration = binding.Generation

	// Create condition with proper timestamp
	conditionType := "Ready"
	conditionStatus := metav1.ConditionTrue
	reason := "ReconcileSuccess"

	if status != "Ready" {
		conditionStatus = metav1.ConditionFalse
		reason = "ReconcileError"
	}

	now := metav1.NewTime(time.Now())
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: binding.Generation,
	}

	// Update or append condition
	conditionUpdated := false
	for i, condition := range binding.Status.Conditions {
		if condition.Type == conditionType {
			if condition.Status != conditionStatus {
				newCondition.LastTransitionTime = now
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			binding.Status.Conditions[i] = newCondition
			conditionUpdated = true
			break
		}
	}

	if !conditionUpdated {
		binding.Status.Conditions = append(binding.Status.Conditions, newCondition)
	}

	// Update status
	err := r.Status().Update(ctx, binding)
	if status != "Ready" {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return ctrl.Result{}, err
}

// handleBindingDeletion handles cleanup when binding is being deleted
func (r *PangolinBindingReconciler) handleBindingDeletion(ctx context.Context, binding *tunnelv1alpha1.PangolinBinding) (ctrl.Result, error) {
	// The owned PangolinResource will be automatically deleted due to owner reference
	// Just remove finalizer
	controllerutil.RemoveFinalizer(binding, BindingFinalizerName)
	return ctrl.Result{}, r.Update(ctx, binding)
}

// SetupWithManager sets up the controller with the Manager
func (r *PangolinBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunnelv1alpha1.PangolinBinding{}).
		Owns(&tunnelv1alpha1.PangolinResource{}).
		Complete(r)
}
