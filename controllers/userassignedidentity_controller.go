package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	ra "github.com/crossplane-contrib/provider-upjet-azure/tree/main/apis/cluster/authorization/v1beta1"
	ra2 "github.com/crossplane-contrib/provider-upjet-azure/tree/main/apis/namespaced/authorization/v1beta1"
	mi "github.com/crossplane-contrib/provider-upjet-azure/tree/main/apis/cluster/managedidentity/v1beta1"
	mi2 "github.com/crossplane-contrib/provider-upjet-azure/tree/main/apis/namespaced/managedidentity/v1beta1"
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

	var identity mi2.UserAssignedIdentity
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
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if appName == "" {
		log.Error(fmt.Errorf("invalid name format"), "Cannot extract appName", "name", *identity.Spec.ForProvider.Name)
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	updateNeeded, err := r.updateServiceAccounts(ctx, appName, *clientID, log)
	if err != nil {
		log.Error(err, "Failed to update ServiceAccounts")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	roleUpdateNeeded, err := r.updateRoleAssignments(ctx, appName, *principalID, log)
	if err != nil {
		log.Error(err, "Failed to update RoleAssignments")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
	}

	if updateNeeded || roleUpdateNeeded {
		log.Info("Updates applied, rechecking in 60 seconds to ensure state.")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *UserAssignedIdentityReconciler) updateServiceAccounts(ctx context.Context, appName, clientID string, log logr.Logger) (bool, error) {
	var namespaces corev1.NamespaceList
	if err := r.List(ctx, &namespaces); err != nil {
		return false, err
	}
	updateNeeded := false
	for _, ns := range namespaces.Items {
		saName := fmt.Sprintf("workload-identity-%s", appName)
		var sa corev1.ServiceAccount
		if err := r.Get(ctx, client.ObjectKey{Name: saName, Namespace: ns.Name}, &sa); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, err
		}

		if sa.Annotations == nil || sa.Annotations["azure.workload.identity/client-id"] != clientID {
			if sa.Annotations == nil {
				sa.Annotations = make(map[string]string)
			}
			sa.Annotations["azure.workload.identity/client-id"] = clientID
			if err := r.Update(ctx, &sa); err != nil {
				return false, err
			}
			updateNeeded = true
		}
		// trigger a restart of the deployment that is using the service account to ensure correct client ID is usee
		if updateNeeded {
			if err := r.restartDeployment(ctx, saName, ns.Name, log); err != nil {
				log.Error(err, "Failed to restart deployment after updating service account annotation", "ServiceAccount", saName)
				continue
			}
		}
	}
	return updateNeeded, nil
}

func (r *UserAssignedIdentityReconciler) restartDeployment(ctx context.Context, saName, namespace string, log logr.Logger) error {
	var deployments appsv1.DeploymentList
	// check what deployments are using the service account
	if err := r.List(ctx, &deployments, client.InNamespace(namespace), client.MatchingFields(map[string]string{
		"spec.template.spec.serviceAccountName": saName,
	})); err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		// patch deploy with annotation to trigger restart
		patch := client.MergeFrom(deployment.DeepCopy())
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = map[string]string{}
		}
		deployment.Spec.Template.Annotations["azure.workload.identity/restart"] = time.Now().Format(time.RFC3339)
		if err := r.Patch(ctx, &deployment, patch); err != nil {
			log.Error(err, "Failed to add annotation to deployment", "Deployment", deployment.Name)
			continue
		}
		log.Info("Successfully restarted deployment after updating service account annotation", "Deployment", deployment.Name)
	}
	return nil
}

func (r *UserAssignedIdentityReconciler) updateRoleAssignments(ctx context.Context, appName, principalID string, log logr.Logger) (bool, error) {
	if principalID == "" {
		log.Error(fmt.Errorf("principalID is empty"), "Invalid principalID provided")
		return false, fmt.Errorf("principalID is emtpy")
	}
	
	var roleAssignments ra.RoleAssignmentList
	selector := client.MatchingLabels{"application": appName, "type": "roleassignment"}
	if err := r.Client.List(ctx, &roleAssignments, selector); err != nil {
		return false, err
	}

	roleUpdateNeeded := false
	for _, roleAssignment := range roleAssignments.Items {
		if roleAssignment.Spec.ForProvider.PrincipalID == nil || *roleAssignment.Spec.ForProvider.PrincipalID != principalID {
			if roleAssignment.Spec.ForProvider.PrincipalID == nil {
				roleAssignment.Spec.ForProvider.PrincipalID = new(string)
			}
			*roleAssignment.Spec.ForProvider.PrincipalID = principalID
			if err := r.Client.Update(ctx, &roleAssignment); err != nil {
				continue // Log error but proceed with the next RoleAssignment
			}
			roleUpdateNeeded = true
		}
	}
	return roleUpdateNeeded, nil
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
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, "spec.template.spec.serviceAccountName", func(rawObj client.Object) []string {
        deployment := rawObj.(*appsv1.Deployment)
        return []string{deployment.Spec.Template.Spec.ServiceAccountName}
    }); err != nil {
        return err
    }
	
	return ctrl.NewControllerManagedBy(mgr).
		For(&mi.UserAssignedIdentity{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&ra.RoleAssignment{}).
		Complete(r)
}
