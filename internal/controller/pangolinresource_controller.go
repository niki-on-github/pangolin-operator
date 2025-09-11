package controller

import (
	"context"
	"fmt"
	"strconv"
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
	"github.com/bovf/pangolin-operator/pkg/pangolin"
)

const ResourceFinalizerName = "resource.pangolin.io/finalizer"

// PangolinResourceReconciler reconciles a PangolinResource object
type PangolinResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// RBAC
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinresources/finalizers,verbs=update
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels,verbs=get;list;watch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *PangolinResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PangolinResource
	resource := &tunnelv1alpha1.PangolinResource{}
	if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PangolinResource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PangolinResource")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if resource.DeletionTimestamp != nil {
		return r.handleResourceDeletion(ctx, resource)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(resource, ResourceFinalizerName) {
		controllerutil.AddFinalizer(resource, ResourceFinalizerName)
		return ctrl.Result{}, r.Update(ctx, resource)
	}

	// Get tunnel
	tunnel, err := r.getTunnelForResource(ctx, resource)
	if err != nil {
		logger.Error(err, "Failed to get referenced tunnel")
		return r.updateResourceStatus(ctx, resource, "Error", err.Error())
	}
	if tunnel.Status.Status != "Ready" {
		logger.Info("Tunnel not ready yet, waiting", "tunnel", tunnel.Name)
		return r.updateResourceStatus(ctx, resource, "Waiting", "Waiting for tunnel to be ready")
	}

	// Get organization from tunnel
	org, err := r.getOrganizationForTunnel(ctx, tunnel)
	if err != nil {
		logger.Error(err, "Failed to get organization for tunnel")
		return r.updateResourceStatus(ctx, resource, "Error", err.Error())
	}
	if org.Status.Status != "Ready" {
		logger.Info("Organization not ready yet, waiting", "organization", org.Name)
		return r.updateResourceStatus(ctx, resource, "Waiting", "Waiting for organization to be ready")
	}

	// Create API client from organization
	apiClient, err := r.createPangolinClientFromOrganization(ctx, org)
	if err != nil {
		logger.Error(err, "Failed to create Pangolin API client")
		return r.updateResourceStatus(ctx, resource, "Error", err.Error())
	}

	// IDs
	orgID := org.Status.OrganizationID
	if orgID == "" {
		return r.updateResourceStatus(ctx, resource, "Error", "Organization missing organization ID")
	}
	if tunnel.Status.SiteID == 0 {
		return r.updateResourceStatus(ctx, resource, "Error", "Tunnel missing site ID")
	}
	siteID := strconv.Itoa(tunnel.Status.SiteID)

	// Reconcile resource (includes domain resolution for HTTP)
	pRes, err := r.reconcilePangolinResource(ctx, apiClient, orgID, siteID, resource, org)
	if err != nil {
		logger.Error(err, "Failed to reconcile Pangolin resource")
		return r.updateResourceStatus(ctx, resource, "Error", err.Error())
	}

	// Normalize ID from either id or resourceId
	rid := pRes.EffectiveID()
	if rid == "" {
		return r.updateResourceStatus(ctx, resource, "Error", "empty resource id after CreateResource")
	}

	// Create/ensure target
	target, err := r.reconcilePangolinTarget(ctx, apiClient, rid, resource)
	if err != nil {
		logger.Error(err, "Failed to reconcile Pangolin target")
		return r.updateResourceStatus(ctx, resource, "Error", err.Error())
	}

	// Update status
	resource.Status.ResourceID = rid
	resource.Status.TargetID = target.ID
	if resource.Spec.Protocol == "http" && resource.Spec.HTTPConfig != nil {
		resource.Status.URL = fmt.Sprintf("https://%s", resource.Status.FullDomain)
	} else if resource.Spec.ProxyConfig != nil {
		resource.Status.ProxyEndpoint = fmt.Sprintf("%s:%d", tunnel.Status.Endpoint, resource.Spec.ProxyConfig.ProxyPort)
	}
	return r.updateResourceStatus(ctx, resource, "Ready", "Resource is ready")
}

func (r *PangolinResourceReconciler) getTunnelForResource(ctx context.Context, resource *tunnelv1alpha1.PangolinResource) (*tunnelv1alpha1.PangolinTunnel, error) {
	tunnel := &tunnelv1alpha1.PangolinTunnel{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: resource.Namespace,
		Name:      resource.Spec.TunnelRef.Name,
	}, tunnel); err != nil {
		return nil, fmt.Errorf("failed to get tunnel %s: %w", resource.Spec.TunnelRef.Name, err)
	}
	return tunnel, nil
}

func (r *PangolinResourceReconciler) getOrganizationForTunnel(ctx context.Context, tunnel *tunnelv1alpha1.PangolinTunnel) (*tunnelv1alpha1.PangolinOrganization, error) {
	org := &tunnelv1alpha1.PangolinOrganization{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: tunnel.Namespace,
		Name:      tunnel.Spec.OrganizationRef.Name,
	}, org); err != nil {
		return nil, fmt.Errorf("failed to get organization %s: %w", tunnel.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}

func (r *PangolinResourceReconciler) createPangolinClientFromOrganization(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization) (*pangolin.Client, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: org.Namespace, Name: org.Spec.APIKeyRef.Name}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}
	apiKeyBytes, ok := secret.Data[org.Spec.APIKeyRef.Key]
	if !ok {
		return nil, fmt.Errorf("API key not found in secret")
	}
	return pangolin.NewClient(org.Spec.APIEndpoint, string(apiKeyBytes)), nil
}

func (r *PangolinResourceReconciler) reconcilePangolinResource(
	ctx context.Context,
	api *pangolin.Client,
	orgID, siteID string,
	resource *tunnelv1alpha1.PangolinResource,
	org *tunnelv1alpha1.PangolinOrganization,
) (*pangolin.Resource, error) {
	logger := log.FromContext(ctx)

	// Bind to existing
	if resource.Spec.ResourceID != "" {
		resource.Status.BindingMode = "Bound"
		return &pangolin.Resource{ID: resource.Spec.ResourceID, Name: resource.Spec.Name}, nil
	}
	// Already created
	if resource.Status.ResourceID != "" {
		resource.Status.BindingMode = "Created"
		return &pangolin.Resource{ID: resource.Status.ResourceID, Name: resource.Spec.Name}, nil
	}

	// Create spec
	var resSpec pangolin.ResourceCreateSpec
	if resource.Spec.Protocol == "http" && resource.Spec.HTTPConfig != nil {
		domainID, fullDomain, err := r.resolveDomainForResource(ctx, resource, org)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve domain: %w", err)
		}
		resource.Status.ResolvedDomainID = domainID
		resource.Status.FullDomain = fullDomain

		resSpec = pangolin.ResourceCreateSpec{
			Name:      resource.Spec.Name,
			SiteID:    mustParseInt(siteID),
			HTTP:      true,
			Protocol:  "tcp", // API expects "tcp" for HTTP resources
			Subdomain: resource.Spec.HTTPConfig.Subdomain,
			DomainID:  domainID,
		}
	} else if resource.Spec.ProxyConfig != nil {
		resSpec = pangolin.ResourceCreateSpec{
			Name:        resource.Spec.Name,
			SiteID:      mustParseInt(siteID),
			HTTP:        false,
			Protocol:    resource.Spec.Protocol,
			ProxyPort:   resource.Spec.ProxyConfig.ProxyPort,
			EnableProxy: *resource.Spec.ProxyConfig.EnableProxy,
		}
	} else {
		return nil, fmt.Errorf("invalid resource configuration: must specify either httpConfig or proxyConfig")
	}

	logger.Info("Creating Pangolin resource", "orgID", orgID, "siteID", siteID, "resourceSpec", resSpec)
	pRes, err := api.CreateResource(ctx, orgID, siteID, resSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pangolin resource: %w", err)
	}
	resource.Status.BindingMode = "Created"
	return pRes, nil
}

func (r *PangolinResourceReconciler) resolveDomainForResource(
	_ context.Context,
	resource *tunnelv1alpha1.PangolinResource,
	org *tunnelv1alpha1.PangolinOrganization,
) (string, string, error) {
	httpCfg := resource.Spec.HTTPConfig
	var domainID string
	var baseDomain string

	// Priority: explicit ID -> explicit name -> org default
	if httpCfg.DomainID != "" {
		domainID = httpCfg.DomainID
		for _, d := range org.Status.Domains {
			if d.DomainID == domainID {
				baseDomain = d.BaseDomain
				break
			}
		}
		if baseDomain == "" {
			return "", "", fmt.Errorf("domainId %s not found in organization domains", domainID)
		}
	} else if httpCfg.DomainName != "" {
		for _, d := range org.Status.Domains {
			if d.BaseDomain == httpCfg.DomainName {
				domainID = d.DomainID
				baseDomain = d.BaseDomain
				break
			}
		}
		if baseDomain == "" {
			return "", "", fmt.Errorf("domainName %s not found in organization domains", httpCfg.DomainName)
		}
	} else {
		if org.Status.DefaultDomainID == "" {
			return "", "", fmt.Errorf("no domain specified and no default domain available")
		}
		domainID = org.Status.DefaultDomainID
		for _, d := range org.Status.Domains {
			if d.DomainID == domainID {
				baseDomain = d.BaseDomain
				break
			}
		}
		if baseDomain == "" {
			return "", "", fmt.Errorf("default domainId %s not found in organization domains", domainID)
		}
	}

	full := fmt.Sprintf("%s.%s", httpCfg.Subdomain, baseDomain)
	return domainID, full, nil
}

func (r *PangolinResourceReconciler) reconcilePangolinTarget(
	ctx context.Context,
	api *pangolin.Client,
	resourceID string,
	resource *tunnelv1alpha1.PangolinResource,
) (*pangolin.Target, error) {
	if resource.Status.TargetID != "" {
		return &pangolin.Target{ID: resource.Status.TargetID}, nil
	}
	tSpec := pangolin.TargetCreateSpec{
		IP:      resource.Spec.Target.IP,
		Port:    resource.Spec.Target.Port,
		Method:  resource.Spec.Target.Method,
		Enabled: *resource.Spec.Enabled,
	}
	target, err := api.CreateTarget(ctx, resourceID, tSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pangolin target: %w", err)
	}
	return target, nil
}

func (r *PangolinResourceReconciler) updateResourceStatus(ctx context.Context, resource *tunnelv1alpha1.PangolinResource, status, message string) (ctrl.Result, error) {
	resource.Status.Status = status
	resource.Status.ObservedGeneration = resource.Generation

	condType := "Ready"
	condStatus := metav1.ConditionTrue
	reason := "ReconcileSuccess"
	if status != "Ready" {
		condStatus = metav1.ConditionFalse
		reason = "ReconcileError"
	}
	now := metav1.NewTime(time.Now())
	newCond := metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: resource.Generation,
	}

	updated := false
	for i, c := range resource.Status.Conditions {
		if c.Type == condType {
			if c.Status != condStatus {
				newCond.LastTransitionTime = now
			} else {
				newCond.LastTransitionTime = c.LastTransitionTime
			}
			resource.Status.Conditions[i] = newCond
			updated = true
			break
		}
	}
	if !updated {
		resource.Status.Conditions = append(resource.Status.Conditions, newCond)
	}

	err := r.Status().Update(ctx, resource)
	if status != "Ready" {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	return ctrl.Result{}, err
}

func (r *PangolinResourceReconciler) handleResourceDeletion(ctx context.Context, resource *tunnelv1alpha1.PangolinResource) (ctrl.Result, error) {
	// TODO: optional cleanup via API
	controllerutil.RemoveFinalizer(resource, ResourceFinalizerName)
	return ctrl.Result{}, r.Update(ctx, resource)
}

func (r *PangolinResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunnelv1alpha1.PangolinResource{}).
		Complete(r)
}

func mustParseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
