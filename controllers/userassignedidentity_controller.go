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
	"github.com/upbound/provider-azure/apis/managedidentity/v1beta1"
)

type UserAssignedIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

func (r *UserAssignedIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("userassignedidentity", req.NamespacedName)

	var identity v1beta1.UserAssignedIdentity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		log.Error(err, "Unable to fetch UserAssignedIdentity")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	clientID := identity.Status.AtProvider.ClientID
	log.Info("Fetched UserAssignedIdentity", "clientID", clientID)

	if clientID != nil && *clientID != "" {
		appName := extractAppName(*identity.Spec.ForProvider.Name)
		log.Info("Extracted application name", "appName", appName)

		if appName == "" {
			err := fmt.Errorf("invalid format for UserAssignedIdentity name, cannot extract appName: %s", *identity.Spec.ForProvider.Name)
			log.Error(err, "Application name could not be extracted")
			return ctrl.Result{}, err
		}

		var namespaces corev1.NamespaceList
		if err := r.List(ctx, &namespaces); err != nil {
			log.Error(err, "Unable to list namespaces")
			return ctrl.Result{}, err
		}

		for _, ns := range namespaces.Items {
			saName := fmt.Sprintf("workload-identity-%s", appName)
			saKey := client.ObjectKey{Name: saName, Namespace: ns.Name}
			var sa corev1.ServiceAccount
			if err := r.Get(ctx, saKey, &sa); err == nil {
				// Found the service account, update it
				if sa.Annotations == nil {
					sa.Annotations = make(map[string]string)
				}

				log.Info("Current ServiceAccount Annotations", "Annotations", sa.Annotations)

				sa.Annotations["azure.workload.identity/client-id"] = *clientID
				if err := r.Update(ctx, &sa); err != nil {
					log.Error(err, "Failed to update ServiceAccount annotations", "ServiceAccount", saName)
					continue
				}
				log.Info("Successfully updated ServiceAccount with clientId", "ServiceAccount", saName, "clientId", *clientID)
			} else if !errors.IsNotFound(err) {
				log.Error(err, "Failed to get ServiceAccount", "ServiceAccount", saName)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func extractAppName(managedIdentityName string) string {
	parts := strings.Split(managedIdentityName, "-")
	if len(parts) < 4 {
		return ""
	}
	// Assuming the app name is always the third part in e.g `id-ats-appname-dv-azunea-001`
	return parts[2]
}

func (r *UserAssignedIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.UserAssignedIdentity{}).
		Complete(r)
}
