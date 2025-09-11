package controller

import (
	"context"
	"fmt"
	"time"

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

	tunnelv1alpha1 "github.com/bovf/pangolin-operator/api/v1alpha1"
	"github.com/bovf/pangolin-operator/pkg/pangolin"
)

const (
	TunnelFinalizerName = "tunnel.pangolin.io/finalizer"
)

// PangolinTunnelReconciler reconciles a PangolinTunnel object
type PangolinTunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels/finalizers,verbs=update
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations,verbs=get;list;watch
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

	// Get the referenced organization
	org, err := r.getOrganizationForTunnel(ctx, tunnel)
	if err != nil {
		logger.Error(err, "Failed to get referenced organization")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Check if organization is ready
	if org.Status.Status != "Ready" {
		logger.Info("Organization not ready yet, waiting", "organization", org.Name)
		return r.updateStatus(ctx, tunnel, "Waiting", "Waiting for organization to be ready")
	}

	// Create Pangolin API client using organization credentials
	apiClient, err := r.createPangolinClientFromOrganization(ctx, org)
	if err != nil {
		logger.Error(err, "Failed to create Pangolin API client")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Get organization ID from organization status
	orgID := org.Status.OrganizationID
	if orgID == "" {
		return r.updateStatus(ctx, tunnel, "Error", "Organization missing organization ID")
	}

	// Reconcile site with flexible binding
	site, err := r.reconcileSite(ctx, apiClient, orgID, tunnel)
	if err != nil {
		logger.Error(err, "Failed to reconcile site")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Create Newt secret if needed
	err = r.reconcileNewtSecret(ctx, tunnel, site)
	if err != nil {
		logger.Error(err, "Failed to reconcile Newt secret")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	// Create Newt deployment if needed
	err = r.reconcileNewtDeployment(ctx, tunnel, site)
	if err != nil {
		logger.Error(err, "Failed to reconcile Newt deployment")
		return r.updateStatus(ctx, tunnel, "Error", err.Error())
	}

	return r.updateStatus(ctx, tunnel, "Ready", "Tunnel is ready")
}

// getOrganizationForTunnel retrieves the organization referenced by the tunnel
func (r *PangolinTunnelReconciler) getOrganizationForTunnel(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel) (*tunnelv1alpha1.PangolinOrganization, error) {
	org := &tunnelv1alpha1.PangolinOrganization{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: tunnel.Namespace,
		Name:      tunnel.Spec.OrganizationRef.Name,
	}, org)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", tunnel.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}

// createPangolinClientFromOrganization creates API client using organization's credentials
func (r *PangolinTunnelReconciler) createPangolinClientFromOrganization(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization) (*pangolin.Client, error) {
	// Get API key from secret referenced by organization
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: org.Namespace,
		Name:      org.Spec.APIKeyRef.Name,
	}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}

	apiKeyBytes, ok := secret.Data[org.Spec.APIKeyRef.Key]
	if !ok {
		return nil, fmt.Errorf("API key not found in secret")
	}

	return pangolin.NewClient(org.Spec.APIEndpoint, string(apiKeyBytes)), nil
}

// reconcileSite handles flexible site binding and creation
func (r *PangolinTunnelReconciler) reconcileSite(ctx context.Context, apiClient *pangolin.Client, orgID string, tunnel *tunnelv1alpha1.PangolinTunnel) (*pangolin.Site, error) {

	// BINDING MODE: Check if binding to existing site
	if tunnel.Spec.SiteID != nil || tunnel.Spec.NiceID != "" {
		var site *pangolin.Site
		var err error

		if tunnel.Spec.SiteID != nil {
			// Bind by numeric siteId
			site, err = apiClient.GetSiteByID(ctx, *tunnel.Spec.SiteID)
		} else {
			// Bind by niceId
			site, err = apiClient.GetSiteByNiceID(ctx, orgID, tunnel.Spec.NiceID)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to bind to existing site: %w", err)
		}

		// Populate status with all API fields
		tunnel.Status.SiteID = site.SiteID
		tunnel.Status.NiceID = site.NiceID
		tunnel.Status.SiteName = site.Name
		tunnel.Status.SiteType = site.Type
		tunnel.Status.Subnet = site.Subnet
		tunnel.Status.Address = site.Address
		tunnel.Status.Online = site.Online
		tunnel.Status.Endpoint = site.Endpoint
		tunnel.Status.BindingMode = "Bound"

		return site, nil
	}

	// CREATE MODE: Create new site
	if tunnel.Status.SiteID == 0 {
		// Use defaults from organization if not specified
		siteName := tunnel.Spec.SiteName
		siteType := tunnel.Spec.SiteType

		// TODO: Apply organization defaults if fields are empty

		site, err := apiClient.CreateSite(ctx, orgID, siteName, siteType)
		if err != nil {
			return nil, fmt.Errorf("failed to create site: %w", err)
		}

		// Populate status
		tunnel.Status.SiteID = site.SiteID
		tunnel.Status.NiceID = site.NiceID
		tunnel.Status.SiteName = site.Name
		tunnel.Status.SiteType = site.Type
		tunnel.Status.BindingMode = "Created"

		return site, nil
	}

	// Already exists - return existing info (could validate with API)
	return &pangolin.Site{
		SiteID: tunnel.Status.SiteID,
		NiceID: tunnel.Status.NiceID,
		Name:   tunnel.Status.SiteName,
	}, nil
}

// reconcileNewtSecret ensures a Secret with Newt credentials is present if needed
func (r *PangolinTunnelReconciler) reconcileNewtSecret(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel, site *pangolin.Site) error {
	if tunnel.Status.SiteType != "newt" || tunnel.Spec.NewtClient == nil || !tunnel.Spec.NewtClient.Enabled {
		return nil
	}
	// TODO: populate from site.NewtID / secret material from Pangolin API
	// Example: fetch/create Secret
	var _ corev1.Secret
	return nil
}

// reconcileNewtDeployment ensures the Newt client Deployment exists if enabled
func (r *PangolinTunnelReconciler) reconcileNewtDeployment(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel, site *pangolin.Site) error {
	if tunnel.Status.SiteType != "newt" || tunnel.Spec.NewtClient == nil || !tunnel.Spec.NewtClient.Enabled {
		return nil
	}
	// TODO: create/update Deployment and set tunnel.Status.ReadyReplicas
	return nil
}

// handleDeletion handles cleanup when tunnel is being deleted
func (r *PangolinTunnelReconciler) handleDeletion(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel) (ctrl.Result, error) {
	// TODO: call Pangolin API to delete site/resources, ensure idempotency, then remove finalizer
	controllerutil.RemoveFinalizer(tunnel, TunnelFinalizerName)
	return ctrl.Result{}, r.Update(ctx, tunnel)
}

// updateStatus updates the tunnel status and returns appropriate reconcile result
func (r *PangolinTunnelReconciler) updateStatus(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel, status, message string) (ctrl.Result, error) {
	tunnel.Status.Status = status
	tunnel.Status.ObservedGeneration = tunnel.Generation

	// Create condition with required timestamp
	newCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "ReconcileError",
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
		ObservedGeneration: tunnel.Generation,
	}

	if status == "Ready" {
		newCondition.Status = metav1.ConditionTrue
		newCondition.Reason = "ReconcileSuccess"
	}

	tunnel.Status.Conditions = []metav1.Condition{newCondition}

	err := r.Status().Update(ctx, tunnel)
	return ctrl.Result{RequeueAfter: time.Minute}, err
}

// SetupWithManager sets up the controller with the Manager
func (r *PangolinTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunnelv1alpha1.PangolinTunnel{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
