package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/go-logr/logr"
)

type UserAssignedIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

func (r *UserAssignedIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("userassignedidentity", req.NamespacedName)

	// Try to fetch UserAssignedIdentity - try multiple API groups
	clientID, principalID, appName, err := r.fetchIdentityInfo(ctx, req.NamespacedName, log)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch UserAssignedIdentity from any known API group")
		return ctrl.Result{}, err
	}

	log.Info("Fetched UserAssignedIdentity", "clientID", clientID, "principalID", principalID, "appName", appName)

	if clientID == "" || principalID == "" {
		log.Info("Missing critical ID information, skipping update.")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if appName == "" {
		log.Error(fmt.Errorf("invalid name format"), "Cannot extract appName")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	updateNeeded, err := r.updateServiceAccounts(ctx, appName, clientID, log)
	if err != nil {
		log.Error(err, "Failed to update ServiceAccounts")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	roleUpdateNeeded, err := r.updateRoleAssignments(ctx, appName, principalID, log)
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

// fetchIdentityInfo tries to fetch UserAssignedIdentity from multiple API groups
func (r *UserAssignedIdentityReconciler) fetchIdentityInfo(ctx context.Context, namespacedName client.ObjectKey, log logr.Logger) (clientID, principalID, appName string, err error) {
	// List of API groups to try (in order of preference)
	apiGroups := []string{
		"managedidentity.azure.upbound.io",
		"managedidentity.azure.m.upbound.io",
	}

	for _, apiGroup := range apiGroups {
		gvk := schema.GroupVersionKind{
			Group:   apiGroup,
			Version: "v1beta1",
			Kind:    "UserAssignedIdentity",
		}

		identity := &unstructured.Unstructured{}
		identity.SetGroupVersionKind(gvk)

		if err := r.Get(ctx, namespacedName, identity); err != nil {
			if errors.IsNotFound(err) {
				log.V(1).Info("UserAssignedIdentity not found in API group", "apiGroup", apiGroup)
				continue
			}
			log.V(1).Info("Error fetching from API group", "apiGroup", apiGroup, "error", err)
			continue
		}

		// Successfully found the resource, extract the data
		log.Info("Found UserAssignedIdentity", "apiGroup", apiGroup)
		
		// Extract clientID from status.atProvider.clientID or status.atProvider.clientId
		clientIDVal, found, err := unstructured.NestedString(identity.Object, "status", "atProvider", "clientID")
		if err != nil || !found {
			// Try camelCase variant
			clientIDVal, found, err = unstructured.NestedString(identity.Object, "status", "atProvider", "clientId")
			if err != nil || !found {
				log.V(1).Info("clientID not found in status", "error", err)
			} else {
				clientID = clientIDVal
			}
		} else {
			clientID = clientIDVal
		}

		// Extract principalID from status.atProvider.principalID or status.atProvider.principalId
		principalIDVal, found, err := unstructured.NestedString(identity.Object, "status", "atProvider", "principalID")
		if err != nil || !found {
			// Try camelCase variant
			principalIDVal, found, err = unstructured.NestedString(identity.Object, "status", "atProvider", "principalId")
			if err != nil || !found {
				log.V(1).Info("principalID not found in status", "error", err)
			} else {
				principalID = principalIDVal
			}
		} else {
			principalID = principalIDVal
		}

		// Extract name from spec.forProvider.name
		nameVal, found, err := unstructured.NestedString(identity.Object, "spec", "forProvider", "name")
		if err != nil || !found {
			log.V(1).Info("name not found in spec", "error", err)
		} else {
			appName = extractAppName(nameVal)
		}

		return clientID, principalID, appName, nil
	}

	// If we get here, we didn't find the resource in any API group
	return "", "", "", errors.NewNotFound(schema.GroupResource{Resource: "userassignedidentities"}, namespacedName.Name)
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
		return false, fmt.Errorf("principalID is empty")
	}

	roleUpdateNeeded := false
	selector := client.MatchingLabels{"application": appName, "type": "roleassignment"}
	
	// Try multiple API groups for RoleAssignments
	apiGroups := []string{
		"authorization.azure.upbound.io",
		"authorization.azure.m.upbound.io",
	}

	for _, apiGroup := range apiGroups {
		roleAssignmentList := &unstructured.UnstructuredList{}
		roleAssignmentList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   apiGroup,
			Version: "v1beta1",
			Kind:    "RoleAssignmentList",
		})

		if err := r.Client.List(ctx, roleAssignmentList, selector); err != nil {
			log.V(1).Info("Could not list RoleAssignments from API group", "apiGroup", apiGroup, "error", err)
			continue
		}

		log.Info("Found RoleAssignments", "apiGroup", apiGroup, "count", len(roleAssignmentList.Items))

		for _, item := range roleAssignmentList.Items {
			// Extract current principalID from spec.forProvider.principalID or spec.forProvider.principalId
			currentPrincipalID, found, err := unstructured.NestedString(item.Object, "spec", "forProvider", "principalID")
			fieldName := "principalID"
			if err != nil || !found {
				// Try camelCase variant
				currentPrincipalID, found, err = unstructured.NestedString(item.Object, "spec", "forProvider", "principalId")
				fieldName = "principalId"
				if err != nil {
					log.V(1).Info("Error reading principalID", "error", err)
				}
			}

			if !found || currentPrincipalID != principalID {
				// Update using the field name we detected
				if err := unstructured.SetNestedField(item.Object, principalID, "spec", "forProvider", fieldName); err != nil {
					log.Error(err, "Failed to set principalID", "name", item.GetName(), "fieldName", fieldName)
					continue
				}

				if err := r.Client.Update(ctx, &item); err != nil {
					log.Error(err, "Failed to update RoleAssignment", "name", item.GetName(), "apiGroup", apiGroup)
					continue
				}

				log.Info("Updated RoleAssignment", "name", item.GetName(), "apiGroup", apiGroup, "fieldName", fieldName)
				roleUpdateNeeded = true
			}
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

	// Create the primary watch for the first UserAssignedIdentity API group
	primaryIdentityGVK := schema.GroupVersionKind{
		Group:   "managedidentity.azure.m.upbound.io",
		Version: "v1beta1",
		Kind:    "UserAssignedIdentity",
	}
	primaryIdentity := &unstructured.Unstructured{}
	primaryIdentity.SetGroupVersionKind(primaryIdentityGVK)

	builder := ctrl.NewControllerManagedBy(mgr).
		For(primaryIdentity).
		Owns(&corev1.ServiceAccount{})

	// Add additional watch for the alternative UserAssignedIdentity API group
	altIdentityGVK := schema.GroupVersionKind{
		Group:   "managedidentity.azure.upbound.io",
		Version: "v1beta1",
		Kind:    "UserAssignedIdentity",
	}
	altIdentity := &unstructured.Unstructured{}
	altIdentity.SetGroupVersionKind(altIdentityGVK)
	builder = builder.Watches(
		altIdentity,
		&handler.EnqueueRequestForObject{},
	)

	// Watch RoleAssignment resources from multiple API groups
	roleAssignmentGVKs := []schema.GroupVersionKind{
		{Group: "authorization.azure.upbound.io", Version: "v1beta1", Kind: "RoleAssignment"},
		{Group: "authorization.azure.m.upbound.io", Version: "v1beta1", Kind: "RoleAssignment"},
	}

	for _, gvk := range roleAssignmentGVKs {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		builder = builder.Owns(u)
	}

	return builder.Complete(r)
}
