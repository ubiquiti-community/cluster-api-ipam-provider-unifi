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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/unifi"
)

const (
	// DefaultUnifiSite is the default Unifi site name when not specified.
	DefaultUnifiSite = "default"
)

// UnifiInstanceReconciler reconciles a UnifiInstance object.
type UnifiInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *UnifiInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &v1beta2.UnifiInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch UnifiInstance")
		return ctrl.Result{}, err
	}

	apiKey, err := r.getAPIKey(ctx, instance, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	client, err := r.createAndValidateClient(ctx, instance, apiKey, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.updateStatusReady(ctx, instance, logger, client != nil)
}

func (r *UnifiInstanceReconciler) getAPIKey(ctx context.Context, instance *v1beta2.UnifiInstance, logger logr.Logger) (string, error) {
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Name:      instance.Spec.CredentialsRef.Name,
		Namespace: instance.Namespace,
	}

	if err := r.Get(ctx, secretName, secret); err != nil {
		return "", r.updateStatusError(ctx, instance, logger, "SecretNotFound", fmt.Sprintf("failed to get secret %s: %v", secretName, err), err)
	}

	apiKey := string(secret.Data["apiKey"])
	if apiKey == "" {
		err := fmt.Errorf("secret %s must contain 'apiKey' key", secretName)
		return "", r.updateStatusError(ctx, instance, logger, "InvalidCredentials", err.Error(), err)
	}

	return apiKey, nil
}

func (r *UnifiInstanceReconciler) createAndValidateClient(ctx context.Context, instance *v1beta2.UnifiInstance, apiKey string, logger logr.Logger) (*unifi.Client, error) {
	site := DefaultUnifiSite
	if instance.Spec.Site != nil {
		site = *instance.Spec.Site
	}
	insecure := false
	if instance.Spec.Insecure != nil {
		insecure = *instance.Spec.Insecure
	}
	client, err := unifi.NewClient(unifi.Config{
		Host:     instance.Spec.Host,
		APIKey:   apiKey,
		Site:     site,
		Insecure: insecure,
	})
	if err != nil {
		return nil, r.updateStatusError(ctx, instance, logger, "ClientCreationFailed", fmt.Sprintf("failed to create Unifi client: %v", err), err)
	}

	if err := client.ValidateCredentials(ctx); err != nil {
		return nil, r.updateStatusError(ctx, instance, logger, "CredentialsValidationFailed", err.Error(), err)
	}

	return client, nil
}

func (r *UnifiInstanceReconciler) updateStatusError(ctx context.Context, instance *v1beta2.UnifiInstance, logger logr.Logger, reason, message string, origErr error) error {
	logger.Error(origErr, "validation failed")
	falseVal := false
	instance.Status.Ready = &falseVal
	instance.Status.FailureReason = &reason
	instance.Status.FailureMessage = &message
	if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
		logger.Error(updateErr, "unable to update UnifiInstance status")
	}
	return origErr
}

func (r *UnifiInstanceReconciler) updateStatusReady(ctx context.Context, instance *v1beta2.UnifiInstance, logger logr.Logger, ready bool) (ctrl.Result, error) {
	instance.Status.Ready = &ready
	instance.Status.FailureReason = nil
	instance.Status.FailureMessage = nil
	now := metav1.Now()
	instance.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "unable to update UnifiInstance status")
		return ctrl.Result{}, err
	}

	logger.Info("successfully validated UnifiInstance", "instance", client.ObjectKeyFromObject(instance))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnifiInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.UnifiInstance{}).
		Complete(r)
}

func ptr(s string) *string {
	return &s
}
