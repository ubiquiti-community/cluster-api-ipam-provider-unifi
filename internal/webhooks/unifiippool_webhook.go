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
	"net/netip"

	"go4.org/netipx"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

const (
	skipValidateDeleteWebhookAnnotation = "ipam.cluster.x-k8s.io/skip-validate-delete-webhook"
)

// UnifiIPPoolWebhook implements validating and defaulting webhooks for UnifiIPPool.
type UnifiIPPoolWebhook struct {
	Client client.Client
}

// SetupWebhookWithManager registers the webhook with the controller manager.
func (w *UnifiIPPoolWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w.Client = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1beta2.UnifiIPPool{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-ipam-cluster-x-k8s-io-v1alpha1-unifiippool,mutating=true,failurePolicy=fail,sideEffects=None,groups=ipam.cluster.x-k8s.io,resources=unifiippools,verbs=create;update,versions=v1alpha1,name=munifiippool.kb.io,admissionReviewVersions=v1

// Default implements webhook.Defaulter.
func (w *UnifiIPPoolWebhook) Default(ctx context.Context, obj runtime.Object) error {
	pool, ok := obj.(*v1beta2.UnifiIPPool)
	if !ok {
		return fmt.Errorf("expected UnifiIPPool, got %T", obj)
	}

	// Set default namespace for InstanceRef if not specified.
	if pool.Spec.InstanceRef.Namespace == "" {
		pool.Spec.InstanceRef.Namespace = pool.Namespace
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-ipam-cluster-x-k8s-io-v1alpha1-unifiippool,mutating=false,failurePolicy=fail,sideEffects=None,groups=ipam.cluster.x-k8s.io,resources=unifiippools,verbs=create;update;delete,versions=v1alpha1,name=vunifiippool.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.CustomValidator.
func (w *UnifiIPPoolWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pool, ok := obj.(*v1beta2.UnifiIPPool)
	if !ok {
		return nil, fmt.Errorf("expected UnifiIPPool, got %T", obj)
	}

	return nil, w.validate(ctx, pool)
}

// ValidateUpdate implements webhook.CustomValidator.
func (w *UnifiIPPoolWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldPool, ok := oldObj.(*v1beta2.UnifiIPPool)
	if !ok {
		return nil, fmt.Errorf("expected UnifiIPPool, got %T", oldObj)
	}

	newPool, ok := newObj.(*v1beta2.UnifiIPPool)
	if !ok {
		return nil, fmt.Errorf("expected UnifiIPPool, got %T", newObj)
	}

	// Validate the new pool.
	if err := w.validate(ctx, newPool); err != nil {
		return nil, err
	}

	// Check if allocated IPs would be orphaned by the update.
	return nil, w.validateUpdate(ctx, oldPool, newPool)
}

// ValidateDelete implements webhook.CustomValidator.
func (w *UnifiIPPoolWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pool, ok := obj.(*v1beta2.UnifiIPPool)
	if !ok {
		return nil, fmt.Errorf("expected UnifiIPPool, got %T", obj)
	}

	// Allow deletion if skip annotation is set.
	if _, ok := pool.Annotations[skipValidateDeleteWebhookAnnotation]; ok {
		return nil, nil
	}

	// Check if there are allocated IPAddresses.
	addresses, err := poolutil.ListAddressesInUse(ctx, w.Client, pool.Namespace, pool.Name, "UnifiIPPool", "ipam.cluster.x-k8s.io")
	if err != nil {
		return nil, fmt.Errorf("failed to list allocated addresses: %w", err)
	}

	if len(addresses) > 0 {
		return nil, field.Forbidden(
			field.NewPath("metadata"),
			fmt.Sprintf("cannot delete UnifiIPPool with %d allocated IP address(es). Delete IPAddress resources first or add annotation %s=true to bypass this check", len(addresses), skipValidateDeleteWebhookAnnotation),
		)
	}

	return nil, nil
}

// validate performs common validation for create and update.
func (w *UnifiIPPoolWebhook) validate(ctx context.Context, pool *v1beta2.UnifiIPPool) error {
	var allErrs field.ErrorList

	// NetworkID is now optional (can be auto-discovered)
	// No validation needed

	// Validate InstanceRef.
	if pool.Spec.InstanceRef.Name == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "instanceRef", "name"), "instanceRef.name is required"))
	}

	// Validate that referenced UnifiInstance exists.
	instanceNamespace := pool.Spec.InstanceRef.Namespace
	if instanceNamespace == "" {
		instanceNamespace = pool.Namespace
	}

	instance := &v1beta2.UnifiInstance{}
	instanceKey := client.ObjectKey{
		Name:      pool.Spec.InstanceRef.Name,
		Namespace: instanceNamespace,
	}
	if err := w.Client.Get(ctx, instanceKey, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			allErrs = append(allErrs, field.NotFound(field.NewPath("spec", "instanceRef"), pool.Spec.InstanceRef.Name))
		} else {
			allErrs = append(allErrs, field.InternalError(field.NewPath("spec", "instanceRef"), err))
		}
	}

	// Validate subnets.
	if len(pool.Spec.Subnets) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec", "subnets"), "at least one subnet is required"))
	}

	for i, subnet := range pool.Spec.Subnets {
		subnetPath := field.NewPath("spec", "subnets").Index(i)
		allErrs = append(allErrs, validateSubnet(&subnet, subnetPath)...)
	}

	// Validate PreAllocations
	allErrs = append(allErrs, validatePreAllocations(pool)...)

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	}

	return nil
}

// validatePreAllocations checks PreAllocations map for issues.
func validatePreAllocations(pool *v1beta2.UnifiIPPool) field.ErrorList {
	var allErrs field.ErrorList

	if len(pool.Spec.PreAllocations) == 0 {
		return allErrs
	}

	// Get default prefix for IPInSubnets check
	defaultPrefix := int32(24)
	if pool.Spec.Prefix != nil {
		defaultPrefix = *pool.Spec.Prefix
	}

	// Track seen IPs to detect duplicates
	seenIPs := make(map[string]string) // IP -> claim name

	for claimName, ipStr := range pool.Spec.PreAllocations {
		preAllocPath := field.NewPath("spec", "preAllocations").Key(claimName)

		// Validate IP format
		_, err := netip.ParseAddr(ipStr)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(preAllocPath, ipStr, fmt.Sprintf("invalid IP address: %v", err)))
			continue
		}

		// Check if IP is in configured subnets
		if !poolutil.IPInSubnets(ipStr, pool.Spec.Subnets, defaultPrefix) {
			allErrs = append(allErrs, field.Invalid(
				preAllocPath,
				ipStr,
				fmt.Sprintf("preallocated IP %s is not within any configured subnet", ipStr),
			))
		}

		// Check for duplicate IPs
		if existingClaim, exists := seenIPs[ipStr]; exists {
			allErrs = append(allErrs, field.Duplicate(
				preAllocPath,
				fmt.Sprintf("IP %s is already preallocated to claim %s", ipStr, existingClaim),
			))
		} else {
			seenIPs[ipStr] = claimName
		}
	}

	return allErrs
}

// validateSubnet validates a single subnet specification.
func validateSubnet(subnet *v1beta2.SubnetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate that subnet has CIDR XOR Start/End
	hasCIDR := subnet.CIDR != ""
	hasRange := subnet.Start != "" || subnet.End != ""

	if !hasCIDR && !hasRange {
		allErrs = append(allErrs, field.Required(fldPath, "must specify either 'cidr' or both 'start' and 'end'"))
		return allErrs
	}

	if hasCIDR && hasRange {
		allErrs = append(allErrs, field.Invalid(fldPath, subnet, "cannot specify both 'cidr' and 'start'/'end' - use one or the other"))
		return allErrs
	}

	// If using CIDR notation
	if hasCIDR {
		cidr, err := netip.ParsePrefix(subnet.CIDR)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cidr"), subnet.CIDR, fmt.Sprintf("invalid CIDR: %v", err)))
			return allErrs
		}

		allErrs = append(allErrs, validatePrefix(subnet, cidr, fldPath)...)
		allErrs = append(allErrs, validateGatewayInCIDR(subnet, cidr, fldPath)...)
		allErrs = append(allErrs, validateExcludeRanges(subnet, cidr, fldPath)...)
	} else {
		// Using Start/End range notation
		if subnet.Start == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("start"), "start is required when using range notation"))
		}
		if subnet.End == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("end"), "end is required when using range notation"))
		}

		if subnet.Start != "" && subnet.End != "" {
			startIP, startErr := netip.ParseAddr(subnet.Start)
			endIP, endErr := netip.ParseAddr(subnet.End)

			if startErr != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("start"), subnet.Start, fmt.Sprintf("invalid start IP: %v", startErr)))
			}
			if endErr != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("end"), subnet.End, fmt.Sprintf("invalid end IP: %v", endErr)))
			}

			if startErr == nil && endErr == nil {
				if startIP.Compare(endIP) > 0 {
					allErrs = append(allErrs, field.Invalid(
						fldPath.Child("start"),
						subnet.Start,
						fmt.Sprintf("start IP %s must be less than or equal to end IP %s", subnet.Start, subnet.End),
					))
				}

				// Validate gateway is in range if specified
				if subnet.Gateway != "" {
					allErrs = append(allErrs, validateGatewayInRange(subnet, startIP, endIP, fldPath)...)
				}
			}
		}
	}

	allErrs = append(allErrs, validateDNSServers(subnet, fldPath)...)

	return allErrs
}

func validatePrefix(subnet *v1beta2.SubnetSpec, cidr netip.Prefix, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if subnet.Prefix != nil && cidr.Bits() != int(*subnet.Prefix) {
		allErrs = append(allErrs, field.Invalid(
			fldPath.Child("prefix"),
			subnet.Prefix,
			fmt.Sprintf("prefix %d does not match CIDR %s (expected %d)", *subnet.Prefix, subnet.CIDR, cidr.Bits()),
		))
	}

	return allErrs
}

func validateGatewayInCIDR(subnet *v1beta2.SubnetSpec, cidr netip.Prefix, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if subnet.Gateway == "" {
		return allErrs // Gateway is optional
	}

	gateway, err := netip.ParseAddr(subnet.Gateway)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gateway"), subnet.Gateway, fmt.Sprintf("invalid gateway IP: %v", err)))
		return allErrs
	}

	if !cidr.Contains(gateway) {
		allErrs = append(allErrs, field.Invalid(
			fldPath.Child("gateway"),
			subnet.Gateway,
			fmt.Sprintf("gateway %s is not within CIDR %s", subnet.Gateway, subnet.CIDR),
		))
	}

	return allErrs
}

func validateGatewayInRange(subnet *v1beta2.SubnetSpec, start, end netip.Addr, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	gateway, err := netip.ParseAddr(subnet.Gateway)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gateway"), subnet.Gateway, fmt.Sprintf("invalid gateway IP: %v", err)))
		return allErrs
	}

	// Check gateway is between start and end
	if gateway.Compare(start) < 0 || gateway.Compare(end) > 0 {
		allErrs = append(allErrs, field.Invalid(
			fldPath.Child("gateway"),
			subnet.Gateway,
			fmt.Sprintf("gateway %s is not within range %s-%s", subnet.Gateway, subnet.Start, subnet.End),
		))
	}

	return allErrs
}

func validateExcludeRanges(subnet *v1beta2.SubnetSpec, cidr netip.Prefix, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for j, excludeRange := range subnet.ExcludeRanges {
		excludePath := fldPath.Child("excludeRanges").Index(j)
		allErrs = append(allErrs, validateExcludeRange(excludeRange, cidr, subnet.CIDR, excludePath)...)
	}

	return allErrs
}

func validateExcludeRange(excludeRange string, cidr netip.Prefix, cidrStr string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Try parsing as CIDR.
	if prefix, err := netip.ParsePrefix(excludeRange); err == nil {
		if !cidr.Overlaps(prefix) {
			allErrs = append(allErrs, field.Invalid(
				fldPath,
				excludeRange,
				fmt.Sprintf("exclude range %s is not within CIDR %s", excludeRange, cidrStr),
			))
		}
		return allErrs
	}

	// Try parsing as single IP.
	if ip, err := netip.ParseAddr(excludeRange); err == nil {
		if !cidr.Contains(ip) {
			allErrs = append(allErrs, field.Invalid(
				fldPath,
				excludeRange,
				fmt.Sprintf("exclude IP %s is not within CIDR %s", excludeRange, cidrStr),
			))
		}
		return allErrs
	}

	// If neither CIDR nor IP, it's invalid.
	allErrs = append(allErrs, field.Invalid(
		fldPath,
		excludeRange,
		"exclude range must be a valid IP address or CIDR",
	))

	return allErrs
}

func validateDNSServers(subnet *v1beta2.SubnetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for j, dns := range subnet.DNSServers {
		dnsPath := fldPath.Child("dnsServers").Index(j)
		if _, err := netip.ParseAddr(dns); err != nil {
			allErrs = append(allErrs, field.Invalid(dnsPath, dns, fmt.Sprintf("invalid DNS server IP: %v", err)))
		}
	}

	return allErrs
}

// validateUpdate checks if the update would orphan allocated IPs.
func (w *UnifiIPPoolWebhook) validateUpdate(ctx context.Context, oldPool, newPool *v1beta2.UnifiIPPool) error {
	addresses, err := poolutil.ListAddressesInUse(ctx, w.Client, oldPool.Namespace, oldPool.Name, "UnifiIPPool", "ipam.cluster.x-k8s.io")
	if err != nil {
		return fmt.Errorf("failed to list allocated addresses: %w", err)
	}

	if len(addresses) == 0 {
		return nil // No allocated addresses, update is safe.
	}

	newIPSet, err := buildNewPoolIPSet(newPool)
	if err != nil {
		return err
	}

	orphanedIPs := findOrphanedIPs(addresses, newIPSet)
	if len(orphanedIPs) > 0 {
		return field.Forbidden(
			field.NewPath("spec", "subnets"),
			fmt.Sprintf("cannot update pool: %d allocated IP address(es) would be outside new pool range: %v", len(orphanedIPs), orphanedIPs),
		)
	}

	return nil
}

func buildNewPoolIPSet(newPool *v1beta2.UnifiIPPool) (*netipx.IPSet, error) {
	var newIPSet *netipx.IPSet
	if len(newPool.Spec.Subnets) > 0 {
		var err error
		newIPSet, err = poolutil.PoolSpecToIPSet(&newPool.Spec.Subnets[0])
		if err != nil {
			return nil, fmt.Errorf("failed to build new pool IPSet: %w", err)
		}
	}
	return newIPSet, nil
}

func findOrphanedIPs(addresses []ipamv1beta2.IPAddress, newIPSet *netipx.IPSet) []string {
	var orphanedIPs []string
	for _, addr := range addresses {
		if addr.Spec.Address == "" {
			continue
		}

		ip, err := netip.ParseAddr(addr.Spec.Address)
		if err != nil {
			continue
		}

		if newIPSet != nil && !newIPSet.Contains(ip) {
			orphanedIPs = append(orphanedIPs, addr.Spec.Address)
		}
	}
	return orphanedIPs
}
