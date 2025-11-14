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

package webhooks

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ipamv1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

// UnifiInstanceWebhook implements validating and defaulting webhooks for UnifiInstance.
type UnifiInstanceWebhook struct {
	Client client.Client
}

// SetupWebhookWithManager registers the webhook with the controller manager.
func (w *UnifiInstanceWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ipamv1beta2.UnifiInstance{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ipam-cluster-x-k8s-io-v1alpha1-unifiinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=ipam.cluster.x-k8s.io,resources=unifiinstances,verbs=create;update,versions=v1alpha1,name=munifiinstance.kb.io,admissionReviewVersions=v1

// Default implements webhook.Defaulter.
func (w *UnifiInstanceWebhook) Default(ctx context.Context, obj runtime.Object) error {
	instance, ok := obj.(*ipamv1beta2.UnifiInstance)
	if !ok {
		return fmt.Errorf("expected UnifiInstance, got %T", obj)
	}

	// Set default site if not specified.
	if instance.Spec.Site == nil || *instance.Spec.Site == "" {
		defaultSite := "default"
		instance.Spec.Site = &defaultSite
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-ipam-cluster-x-k8s-io-v1alpha1-unifiinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=ipam.cluster.x-k8s.io,resources=unifiinstances,verbs=create;update;delete,versions=v1alpha1,name=vunifiinstance.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.CustomValidator.
func (w *UnifiInstanceWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	instance, ok := obj.(*ipamv1beta2.UnifiInstance)
	if !ok {
		return nil, fmt.Errorf("expected UnifiInstance, got %T", obj)
	}

	return nil, w.validate(ctx, instance)
}

// ValidateUpdate implements webhook.CustomValidator.
func (w *UnifiInstanceWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := oldObj.(*ipamv1beta2.UnifiInstance)
	if !ok {
		return nil, fmt.Errorf("expected UnifiInstance, got %T", oldObj)
	}

	newInstance, ok := newObj.(*ipamv1beta2.UnifiInstance)
	if !ok {
		return nil, fmt.Errorf("expected UnifiInstance, got %T", newObj)
	}

	return nil, w.validate(ctx, newInstance)
}

// ValidateDelete implements webhook.CustomValidator.
func (w *UnifiInstanceWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	instance, ok := obj.(*ipamv1beta2.UnifiInstance)
	if !ok {
		return nil, fmt.Errorf("expected UnifiInstance, got %T", obj)
	}

	// Allow deletion if skip annotation is set.
	if _, ok := instance.Annotations[skipValidateDeleteWebhookAnnotation]; ok {
		return nil, nil
	}

	// Check if there are UnifiIPPools referencing this instance.
	poolList := &ipamv1beta2.UnifiIPPoolList{}
	if err := w.Client.List(ctx, poolList, client.InNamespace(instance.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list UnifiIPPools: %w", err)
	}

	referencingPools := []string{}
	for _, pool := range poolList.Items {
		instanceNamespace := pool.Spec.InstanceRef.Namespace
		if instanceNamespace == "" {
			instanceNamespace = pool.Namespace
		}

		if pool.Spec.InstanceRef.Name == instance.Name && instanceNamespace == instance.Namespace {
			referencingPools = append(referencingPools, pool.Name)
		}
	}

	if len(referencingPools) > 0 {
		return nil, field.Forbidden(
			field.NewPath("metadata"),
			fmt.Sprintf("cannot delete UnifiInstance: %d UnifiIPPool(s) reference this instance: %v. Delete pools first or add annotation %s=true to bypass this check", len(referencingPools), referencingPools, skipValidateDeleteWebhookAnnotation),
		)
	}

	return nil, nil
}

// validate performs common validation for create and update.
func (w *UnifiInstanceWebhook) validate(ctx context.Context, instance *ipamv1beta2.UnifiInstance) error {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateHost(instance.Spec.Host)...)
	allErrs = append(allErrs, w.validateCredentialsRef(ctx, instance)...)
	if instance.Spec.Site != nil {
		allErrs = append(allErrs, validateSiteName(*instance.Spec.Site)...)
	}

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	}

	return nil
}

func validateHost(host string) field.ErrorList {
	var allErrs field.ErrorList

	if host == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "host"), "host is required"))
		return allErrs
	}

	parsedURL, err := url.Parse(host)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "host"), host, fmt.Sprintf("invalid URL: %v", err)))
		return allErrs
	}

	// Validate scheme is http or https.
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "host"),
			host,
			"URL scheme must be http or https",
		))
	}

	// Validate host is not empty.
	if parsedURL.Host == "" {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "host"),
			host,
			"URL must include a host",
		))
	}

	return allErrs
}

func (w *UnifiInstanceWebhook) validateCredentialsRef(ctx context.Context, instance *ipamv1beta2.UnifiInstance) field.ErrorList {
	var allErrs field.ErrorList

	if instance.Spec.CredentialsRef.Name == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "credentialsRef", "name"), "credentialsRef.name is required"))
		return allErrs
	}

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Name:      instance.Spec.CredentialsRef.Name,
		Namespace: instance.Namespace,
	}

	if err := w.Client.Get(ctx, secretKey, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			allErrs = append(allErrs, field.NotFound(field.NewPath("spec", "credentialsRef"), instance.Spec.CredentialsRef.Name))
		} else {
			allErrs = append(allErrs, field.InternalError(field.NewPath("spec", "credentialsRef"), err))
		}
		return allErrs
	}

	// Validate secret has required fields.
	if _, ok := secret.Data["apiKey"]; !ok {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "credentialsRef"),
			instance.Spec.CredentialsRef.Name,
			"referenced secret must contain 'apiKey' field",
		))
	}

	return allErrs
}

func validateSiteName(siteName string) field.ErrorList {
	var allErrs field.ErrorList

	if siteName == "" {
		return allErrs
	}

	// Site name validation - Unifi allows alphanumeric, dash, underscore.
	if !isValidSiteName(siteName) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "site"),
			siteName,
			"site name must contain only alphanumeric characters, dashes, and underscores",
		))
	}

	return allErrs
}

func isValidSiteName(siteName string) bool {
	for _, char := range siteName {
		if !isValidSiteChar(char) {
			return false
		}
	}
	return true
}

func isValidSiteChar(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') || char == '-' || char == '_'
}
