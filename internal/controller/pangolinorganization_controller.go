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
	"github.com/bovf/pangolin-operator/pkg/pangolin"
)

const (
	OrganizationFinalizerName = "organization.pangolin.io/finalizer"
)

// PangolinOrganizationReconciler reconciles a PangolinOrganization object
type PangolinOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinorganizations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PangolinOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PangolinOrganization instance
	org := &tunnelv1alpha1.PangolinOrganization{}
	err := r.Get(ctx, req.NamespacedName, org)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PangolinOrganization resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PangolinOrganization")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if org.DeletionTimestamp != nil {
		return r.handleOrganizationDeletion(ctx, org)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(org, OrganizationFinalizerName) {
		controllerutil.AddFinalizer(org, OrganizationFinalizerName)
		return ctrl.Result{}, r.Update(ctx, org)
	}

	// Create Pangolin API client
	apiClient, err := r.createPangolinClient(ctx, org)
	if err != nil {
		logger.Error(err, "Failed to create Pangolin API client")
		return r.updateOrganizationStatus(ctx, org, "Error", err.Error())
	}

	// Reconcile organization
	err = r.reconcileOrganization(ctx, org, apiClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile organization")
		return r.updateOrganizationStatus(ctx, org, "Error", err.Error())
	}

	// Reconcile domains - NEW
	err = r.reconcileDomains(ctx, org, apiClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile domains")
		return r.updateOrganizationStatus(ctx, org, "Error", err.Error())
	}

	return r.updateOrganizationStatus(ctx, org, "Ready", "Organization is ready")
}

// createPangolinClient creates a Pangolin API client from the org spec
func (r *PangolinOrganizationReconciler) createPangolinClient(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization) (*pangolin.Client, error) {
	// Get API key from secret
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: org.Namespace,
		Name:      org.Spec.APIKeyRef.Name,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key secret: %w", err)
	}

	apiKey, ok := secret.Data[org.Spec.APIKeyRef.Key]
	if !ok {
		return nil, fmt.Errorf("API key not found in secret")
	}

	return pangolin.NewClient(org.Spec.APIEndpoint, string(apiKey)), nil
}

// reconcileOrganization handles organization binding/discovery
func (r *PangolinOrganizationReconciler) reconcileOrganization(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization, apiClient *pangolin.Client) error {
	if org.Spec.OrganizationID != "" {
		// BINDING MODE: Bind to existing organization
		orgs, err := apiClient.ListOrganizations(ctx)
		if err != nil {
			return fmt.Errorf("failed to list organizations: %w", err)
		}

		// Find the specified organization
		var targetOrg *pangolin.Organization
		for _, o := range orgs {
			if o.OrgID == org.Spec.OrganizationID {
				targetOrg = &o
				break
			}
		}

		if targetOrg == nil {
			return fmt.Errorf("organization %s not found", org.Spec.OrganizationID)
		}

		// Update status from API
		org.Status.OrganizationID = targetOrg.OrgID
		org.Status.OrganizationName = targetOrg.Name
		org.Status.Subnet = targetOrg.Subnet
		org.Status.BindingMode = "Bound"

	} else {
		// DISCOVERY MODE: Auto-discover first available org
		orgs, err := apiClient.ListOrganizations(ctx)
		if err != nil {
			return fmt.Errorf("failed to list organizations: %w", err)
		}

		if len(orgs) == 0 {
			return fmt.Errorf("no organizations found")
		}

		// Use first organization
		org.Status.OrganizationID = orgs[0].OrgID
		org.Status.OrganizationName = orgs[0].Name
		org.Status.Subnet = orgs[0].Subnet
		org.Status.BindingMode = "Discovered"
	}

	return nil
}

// reconcileDomains fetches and stores available domains - NEW
func (r *PangolinOrganizationReconciler) reconcileDomains(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization, apiClient *pangolin.Client) error {
	logger := log.FromContext(ctx)

	if org.Status.OrganizationID == "" {
		return fmt.Errorf("organization ID not available")
	}

	// Fetch domains from API
	domains, err := apiClient.ListDomains(ctx, org.Status.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to list domains: %w", err)
	}

	logger.Info("Found domains for organization", "orgId", org.Status.OrganizationID, "domainCount", len(domains))

	// Convert to CRD format
	var crdDomains []tunnelv1alpha1.Domain
	for _, d := range domains {
		crdDomains = append(crdDomains, tunnelv1alpha1.Domain{
			DomainID:      d.DomainID,
			BaseDomain:    d.BaseDomain,
			Verified:      d.Verified,
			Type:          d.Type,
			Failed:        d.Failed,
			Tries:         d.Tries,
			ConfigManaged: d.ConfigManaged,
		})
	}

	// Update status with domains
	org.Status.Domains = crdDomains

	// Resolve default domain if specified
	if org.Spec.Defaults != nil && org.Spec.Defaults.DefaultDomain != "" {
		defaultDomainID, err := r.resolveDomainToID(org.Spec.Defaults.DefaultDomain, crdDomains)
		if err != nil {
			logger.Info("Failed to resolve default domain", "defaultDomain", org.Spec.Defaults.DefaultDomain, "error", err)
			// Don't fail the reconciliation for this
		} else {
			org.Status.DefaultDomainID = defaultDomainID
			logger.Info("Resolved default domain", "defaultDomain", org.Spec.Defaults.DefaultDomain, "domainId", defaultDomainID)
		}
	}

	// If no default domain specified, use first available verified domain
	if org.Status.DefaultDomainID == "" && len(crdDomains) > 0 {
		for _, domain := range crdDomains {
			if domain.Verified {
				org.Status.DefaultDomainID = domain.DomainID
				logger.Info("Using first verified domain as default", "domainId", domain.DomainID)
				break
			}
		}
	}

	return nil
}

// resolveDomainToID resolves domain name to domain ID - NEW
func (r *PangolinOrganizationReconciler) resolveDomainToID(domainInput string, domains []tunnelv1alpha1.Domain) (string, error) {
	// Check if it's already a domain ID
	for _, domain := range domains {
		if domain.DomainID == domainInput {
			return domain.DomainID, nil
		}
	}

	// Check if it's a domain name (base domain)
	for _, domain := range domains {
		if domain.BaseDomain == domainInput {
			return domain.DomainID, nil
		}
	}

	return "", fmt.Errorf("domain %s not found", domainInput)
}

// updateOrganizationStatus updates the organization status with proper conditions
func (r *PangolinOrganizationReconciler) updateOrganizationStatus(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization, status, message string) (ctrl.Result, error) {
	org.Status.Status = status
	org.Status.ObservedGeneration = org.Generation

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
		ObservedGeneration: org.Generation,
	}

	// Update or append condition
	conditionUpdated := false
	for i, condition := range org.Status.Conditions {
		if condition.Type == conditionType {
			if condition.Status != conditionStatus {
				newCondition.LastTransitionTime = now
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			org.Status.Conditions[i] = newCondition
			conditionUpdated = true
			break
		}
	}

	if !conditionUpdated {
		org.Status.Conditions = append(org.Status.Conditions, newCondition)
	}

	// Update status
	err := r.Status().Update(ctx, org)
	if status != "Ready" {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return ctrl.Result{}, err
}

// handleOrganizationDeletion handles cleanup when organization is being deleted
func (r *PangolinOrganizationReconciler) handleOrganizationDeletion(ctx context.Context, org *tunnelv1alpha1.PangolinOrganization) (ctrl.Result, error) {
	// TODO: Implement cleanup logic if needed
	// For organization, we typically don't delete from Pangolin API
	// Just remove finalizer
	controllerutil.RemoveFinalizer(org, OrganizationFinalizerName)
	return ctrl.Result{}, r.Update(ctx, org)
}

// SetupWithManager sets up the controller with the Manager
func (r *PangolinOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tunnelv1alpha1.PangolinOrganization{}).
		Complete(r)
}
