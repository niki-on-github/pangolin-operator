package controller

import (
	"context"
	"fmt"
	pangolin "github.com/bovf/pangolin-operator/pkg/pangolin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	tunnelv1alpha1 "github.com/bovf/pangolin-operator/api/v1alpha1"
)

// PangolinTunnelReconciler reconciles a PangolinTunnel object
type PangolinTunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	TunnelFinalizerName = "tunnel.pangolin.io/finalizer"
)

//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels/finalizers,verbs=update

//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunel,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunel/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunel/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PangolinTunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PangolinTunnel instance
	tunnel := &tunnelv1alpha1.PangolinTunnel{}
	err := r.Get(ctx, req.NamespacedName, tunnel)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PangolinTunnel resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PangolinTunnel")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if tunnel.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, tunnel)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(tunnel, TunnelFinalizerName) {
		controllerutil.AddFinalizer(tunnel, TunnelFinalizerName)
		return ctrl.Result{}, r.Update(ctx, tunnel)
	}

	// Create Pangolin API client
	apiClient, err := r.createPangolinClient(ctx, tunnel)
	if err != nil {
		logger.Error(err, "Failed to create Pangolin API client")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Ensure organization exists and get ID
	orgID, err := r.ensureOrganization(ctx, apiClient, tunnel)
	if err != nil {
		logger.Error(err, "Failed to get organization")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Create or update site
	site, err := r.reconcileSite(ctx, apiClient, orgID, tunnel)
	if err != nil {
		logger.Error(err, "Failed to reconcile site")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Create Newt secret
	err = r.reconcileNewtSecret(ctx, tunnel, site)
	if err != nil {
		logger.Error(err, "Failed to reconcile Newt secret")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Create Newt deployment
	err = r.reconcileNewtDeployment(ctx, tunnel, site)
	if err != nil {
		logger.Error(err, "Failed to reconcile Newt deployment")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Update status
	tunnel.Status.SiteID = site.ID
	if tunnel.Status.SiteID == "" {
		tunnel.Status.SiteID = site.SiteID
	}
	tunnel.Status.Status = "Ready"
	tunnel.Status.Endpoint = site.Endpoint
	tunnel.Status.NewtID = site.NewtID
	tunnel.Status.NewtSecretRef = r.getNewtSecretName(tunnel)

	return r.updateStatus(ctx, tunnel, "Ready", "Tunnel is ready")
}

// createPangolinClient creates a Pangolin API client from the tunnel spec
func (r *PangolinTunnelReconciler) createPangolinClient(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel) (*pangolin.Client, error) {
	// Get API key from secret
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: tunnel.Namespace,
		Name:      tunnel.Spec.APIKeyRef.Name,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}

	apiKey, ok := secret.Data[tunnel.Spec.APIKeyRef.Key]
	if !ok {
		return nil, fmt.Errorf("API key not found in secret")
	}

	return pangolin.NewClient(tunnel.Spec.APIEndpoint, string(apiKey)), nil
}

// updateStatus updates the tunnel status and returns appropriate reconcile result
func (r *PangolinTunnelReconciler) updateStatus(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel, status, message string) (ctrl.Result, error) {
	tunnel.Status.Status = status

	// Update conditions
	condition := metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ReconcileSuccess",
		Message: message,
	}

	if status != "Ready" {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ReconcileError"
	}

	tunnel.Status.Conditions = []metav1.Condition{condition}
	tunnel.Status.ObservedGeneration = tunnel.Generation

	err := r.Status().Update(ctx, tunnel)

	if status != "Ready" {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager
func (r *PangolinTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunnelv1alpha1.PangolinTunnel{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
