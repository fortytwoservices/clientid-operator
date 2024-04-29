package controllers

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	ra "github.com/upbound/provider-azure/apis/authorization/v1beta1"
	mi "github.com/upbound/provider-azure/apis/managedidentity/v1beta1"
)

type UserAssignedIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

func (r *UserAssignedIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("userassignedidentity", req.NamespacedName)

	var identity mi.UserAssignedIdentity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		log.Error(err, "Unable to fetch UserAssignedIdentity")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clientID := identity.Status.AtProvider.ClientID
	principalID := identity.Status.AtProvider.PrincipalID
	appName := extractAppName(*identity.Spec.ForProvider.Name)

	log.Info("Fetched UserAssignedIdentity", "clientID", clientID, "principalID", principalID, "appName", appName)

	if clientID == nil || *clientID == "" || principalID == nil || *principalID == "" {
		log.Info("Missing critical ID information, skipping update.")
		return ctrl.Result{}, nil
	}

	if appName == "" {
		log.Error(fmt.Errorf("invalid name format"), "Cannot extract appName", "name", *identity.Spec.ForProvider.Name)
		return ctrl.Result{}, nil
	}

	if err := r.updateServiceAccounts(ctx, appName, *clientID, log); err != nil {
		log.Error(err, "Failed to update ServiceAccounts")
		return ctrl.Result{}, err
	}

	if err := r.updateRoleAssignments(ctx, appName, *principalID, log); err != nil {
		log.Error(err, "Failed to update RoleAssignments")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *UserAssignedIdentityReconciler) updateServiceAccounts(ctx context.Context, appName, clientID string, log logr.Logger) error {
	var namespaces corev1.NamespaceList
	if err := r.List(ctx, &namespaces); err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		saName := fmt.Sprintf("workload-identity-%s", appName)
		var sa corev1.ServiceAccount
		if err := r.Get(ctx, client.ObjectKey{Name: saName, Namespace: ns.Name}, &sa); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string)
		}
		sa.Annotations["azure.workload.identity/client-id"] = clientID
		if err := r.Update(ctx, &sa); err != nil {
			return err
		}
		log.Info("Successfully updated ServiceAccount with ClientID", "ServiceAccount", saName, "ClientID", clientID)
	}
	return nil
}

func (r *UserAssignedIdentityReconciler) updateRoleAssignments(ctx context.Context, appName, principalID string, log logr.Logger) error {
	var roleAssignments ra.RoleAssignmentList
	selector := client.MatchingLabels{"application": appName, "type": "roleassignment"}
	if err := r.Client.List(ctx, &roleAssignments, selector); err != nil {
		log.Error(err, "Unable to list RoleAssignments")
		return err
	}

	log.Info("Number of RoleAssignments fetched", "count", len(roleAssignments.Items))

	for _, roleAssignment := range roleAssignments.Items {
		if roleAssignment.Spec.ForProvider.PrincipalID == nil || *roleAssignment.Spec.ForProvider.PrincipalID != principalID {
			if roleAssignment.Spec.ForProvider.PrincipalID == nil {
				roleAssignment.Spec.ForProvider.PrincipalID = new(string)
			}
			*roleAssignment.Spec.ForProvider.PrincipalID = principalID
			if err := r.Client.Update(ctx, &roleAssignment); err != nil {
				log.Error(err, "Failed to update RoleAssignment with new principalId", "RoleAssignment", roleAssignment.Name, "NewPrincipalID", principalID)
				continue // proceed with next RoleAssignment if the current update fails
			}
			log.Info("Successfully updated RoleAssignment with new PrincipalID", "RoleAssignment", roleAssignment.Name, "PrincipalID", principalID)
		} else {
			log.Info("No update required for RoleAssignment", "RoleAssignment", roleAssignment.Name)
		}
	}
	return nil
}

func extractAppName(managedIdentityName string) string {
	parts := strings.Split(managedIdentityName, "-")
	if len(parts) < 4 {
		return ""
	}
	// Assuming the app name is always the third part in e.g `id-service-appname-dv-azunea-001`
	return parts[2]
}

func (r *UserAssignedIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mi.UserAssignedIdentity{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&ra.RoleAssignment{}).
		Complete(r)
}
