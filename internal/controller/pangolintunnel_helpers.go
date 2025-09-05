// internal/controller/pangolintunnel_helpers.go
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	tunnelv1alpha1 "github.com/bovf/pangolin-operator/api/v1alpha1"
	"github.com/bovf/pangolin-operator/pkg/pangolin"
)

// handleDeletion is called when the CR has a deletionTimestamp and the finalizer is present.
func (r *PangolinTunnelReconciler) handleDeletion(ctx context.Context, pt *tunnelv1alpha1.PangolinTunnel) (ctrl.Result, error) {
	// TODO: call Pangolin API to delete site/resources, ensure idempotency, then remove finalizer
	// Example placeholder, replace with real cleanup logic.
	return ctrl.Result{}, nil
}

// ensureOrganization returns the org ID to use, discovering it if not provided in spec.
func (r *PangolinTunnelReconciler) ensureOrganization(ctx context.Context, api *pangolin.Client, pt *tunnelv1alpha1.PangolinTunnel) (string, error) {
	if pt.Spec.OrganizationID != "" {
		return pt.Spec.OrganizationID, nil
	}
	// TODO: call api.ListOrganizations and choose the correct org deterministically.
	return "", fmt.Errorf("organizationId not set and auto-discovery not implemented")
}

// reconcileSite ensures a Pangolin Site exists and returns its details.
func (r *PangolinTunnelReconciler) reconcileSite(ctx context.Context, api *pangolin.Client, orgID string, pt *tunnelv1alpha1.PangolinTunnel) (*pangolin.Site, error) {
	// TODO: list sites and create if missing; update pt.Status on success
	// Placeholder site struct; replace with real API calls
	site := &pangolin.Site{
		Name:     pt.Spec.SiteName,
		Type:     pt.Spec.SiteType,
		Endpoint: "",
	}
	return site, nil
}

// reconcileNewtSecret ensures a Secret with Newt credentials is present if needed.
func (r *PangolinTunnelReconciler) reconcileNewtSecret(ctx context.Context, pt *tunnelv1alpha1.PangolinTunnel, site *pangolin.Site) error {
	if pt.Spec.SiteType != "newt" || pt.Spec.NewtClient == nil || !pt.Spec.NewtClient.Enabled {
		return nil
	}
	// TODO: populate from site.NewtID / secret material from Pangolin API
	// Example: fetch/create Secret
	var _ corev1.Secret
	return nil
}

// reconcileNewtDeployment ensures the Newt client Deployment exists if enabled.
func (r *PangolinTunnelReconciler) reconcileNewtDeployment(ctx context.Context, pt *tunnelv1alpha1.PangolinTunnel, site *pangolin.Site) error {
	if pt.Spec.SiteType != "newt" || pt.Spec.NewtClient == nil || !pt.Spec.NewtClient.Enabled {
		return nil
	}
	// TODO: create/update Deployment and set pt.Status.ReadyReplicas
	return nil
}

// getNewtSecretName returns the secret name used to store Newt credentials.
func (r *PangolinTunnelReconciler) getNewtSecretName(pt *tunnelv1alpha1.PangolinTunnel) string {
	return fmt.Sprintf("%s-newt", pt.Name)
}
