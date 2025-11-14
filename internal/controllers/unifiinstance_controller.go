/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1alpha1"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/unifi"
)

// UnifiInstanceReconciler reconciles a UnifiInstance object
type UnifiInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *UnifiInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the UnifiInstance
	instance := &ipamv1alpha1.UnifiInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch UnifiInstance")
		return ctrl.Result{}, err
	}

	// Get the credentials secret
	secret := &corev1.Secret{}
	// LocalObjectReference doesn't have Namespace, use instance namespace
	secretName := types.NamespacedName{
		Name:      instance.Spec.CredentialsRef.Name,
		Namespace: instance.Namespace,
	}

	if err := r.Get(ctx, secretName, secret); err != nil {
		logger.Error(err, "unable to fetch credentials secret", "secret", secretName.Name)
		instance.Status.Ready = false
		instance.Status.FailureReason = ptr("SecretNotFound")
		instance.Status.FailureMessage = ptr(fmt.Sprintf("failed to get secret %s: %v", secretName, err))
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "unable to update UnifiInstance status")
		}
		return ctrl.Result{}, err
	}

	apiKey := string(secret.Data["apiKey"])

	if apiKey == "" {
		err := fmt.Errorf("secret %s must contain 'apiKey' key", secretName)
		logger.Error(err, "invalid credentials secret")
		instance.Status.Ready = false
		instance.Status.FailureReason = ptr("InvalidCredentials")
		instance.Status.FailureMessage = ptr(err.Error())
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "unable to update UnifiInstance status")
		}
		return ctrl.Result{}, err
	}

	// Create Unifi client and validate credentials
	client, err := unifi.NewClient(unifi.Config{
		Host:     instance.Spec.Host,
		APIKey:   apiKey,
		Site:     instance.Spec.Site,
		Insecure: instance.Spec.Insecure,
	})
	if err != nil {
		logger.Error(err, "unable to create Unifi client")
		instance.Status.Ready = false
		instance.Status.FailureReason = ptr("ClientCreationFailed")
		instance.Status.FailureMessage = ptr(fmt.Sprintf("failed to create Unifi client: %v", err))
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "unable to update UnifiInstance status")
		}
		return ctrl.Result{}, err
	}

	if err := client.ValidateCredentials(ctx); err != nil {
		logger.Error(err, "unable to validate Unifi credentials")
		instance.Status.Ready = false
		instance.Status.FailureReason = ptr("CredentialsValidationFailed")
		instance.Status.FailureMessage = ptr(err.Error())
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "unable to update UnifiInstance status")
		}
		return ctrl.Result{}, err
	}

	// Update status to ready
	instance.Status.Ready = true
	instance.Status.FailureReason = nil
	instance.Status.FailureMessage = nil
	now := metav1.Now()
	instance.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "unable to update UnifiInstance status")
		return ctrl.Result{}, err
	}

	logger.Info("successfully validated UnifiInstance", "instance", req.NamespacedName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnifiInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.UnifiInstance{}).
		Complete(r)
}

func ptr(s string) *string {
	return &s
}
