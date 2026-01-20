package controllers

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ra2 "github.com/upbound/provider-azure/v2/apis/cluster/authorization/v1beta1"
	mi2 "github.com/upbound/provider-azure/v2/apis/cluster/managedidentity/v1beta1"
	ra "github.com/upbound/provider-azure/v2/apis/namespaced/authorization/v1beta1"
	mi "github.com/upbound/provider-azure/v2/apis/namespaced/managedidentity/v1beta1"
)

func TestUserAssignedIdentityReconciler_Reconcile(t *testing.T) {
	// Register schemes
	s := scheme.Scheme
	_ = appsv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = mi.AddToScheme(s)
	_ = mi2.AddToScheme(s)
	_ = ra.AddToScheme(s)
	_ = ra2.AddToScheme(s)

	// Test data
	namespace := "default"
	identityName := "id-service-testapp-dv-azunea-001"
	appName := "testapp"
	clientID := "test-client-id"
	principalID := "test-principal-id"
	oldPrincipalID := "old-principal-id"

	// 1. UserAssignedIdentity
	namePtr := identityName
	identity := &mi.UserAssignedIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identityName,
			Namespace: namespace,
		},
		Spec: mi.UserAssignedIdentitySpec{
			ForProvider: mi.UserAssignedIdentityParameters{
				Name: &namePtr,
			},
		},
		Status: mi.UserAssignedIdentityStatus{
			AtProvider: mi.UserAssignedIdentityObservation{
				ClientID:    &clientID,
				PrincipalID: &principalID,
			},
		},
	}

	// 2. ServiceAccount
	saName := "workload-identity-" + appName
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}

	// 3. Deployment using the ServiceAccount
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: saName,
				},
			},
		},
	}

	// 4. RoleAssignment
	raName := "test-role-assignment"
	roleAssignment := &ra.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      raName,
			Namespace: namespace,
			Labels: map[string]string{
				"application": appName,
				"type":        "roleassignment",
			},
		},
		Spec: ra.RoleAssignmentSpec{
			ForProvider: ra.RoleAssignmentParameters{
				PrincipalID: &oldPrincipalID, // Needs update
			},
		},
	}

	// 5. Namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	// Objects to seed into the fake client
	objs := []client.Object{identity, sa, deployment, roleAssignment, ns}

	// Create fake client
	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithIndex(&appsv1.Deployment{}, "spec.template.spec.serviceAccountName", func(rawObj client.Object) []string {
			deployment := rawObj.(*appsv1.Deployment)
			return []string{deployment.Spec.Template.Spec.ServiceAccountName}
		}).
		Build()

	// Reconciler
	r := &UserAssignedIdentityReconciler{
		Client: cl,
		Scheme: s,
		Log:    zap.New(zap.UseDevMode(true)),
	}

	// Create Request
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      identityName,
			Namespace: namespace,
		},
	}

	// Run Reconcile
	ctx := context.Background()
	_, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify ServiceAccount Update
	updatedSA := &corev1.ServiceAccount{}
	err = cl.Get(ctx, types.NamespacedName{Name: saName, Namespace: namespace}, updatedSA)
	if err != nil {
		t.Fatalf("Failed to get ServiceAccount: %v", err)
	}
	if val, ok := updatedSA.Annotations["azure.workload.identity/client-id"]; !ok || val != clientID {
		t.Errorf("ServiceAccount annotation incorrect. Expected %s, got %s", clientID, val)
	}

	// Verify Deployment Restart (Annotation added)
	updatedDeploy := &appsv1.Deployment{}
	err = cl.Get(ctx, types.NamespacedName{Name: "test-deployment", Namespace: namespace}, updatedDeploy)
	if err != nil {
		t.Fatalf("Failed to get Deployment: %v", err)
	}
	if _, ok := updatedDeploy.Spec.Template.Annotations["azure.workload.identity/restart"]; !ok {
		t.Error("Deployment missing restart annotation")
	}

	// Verify RoleAssignment Update
	updatedRA := &ra.RoleAssignment{}
	err = cl.Get(ctx, types.NamespacedName{Name: raName, Namespace: namespace}, updatedRA)
	if err != nil {
		t.Fatalf("Failed to get RoleAssignment: %v", err)
	}
	if *updatedRA.Spec.ForProvider.PrincipalID != principalID {
		t.Errorf("RoleAssignment PrincipalID incorrect. Expected %s, got %s", principalID, *updatedRA.Spec.ForProvider.PrincipalID)
	}
}
