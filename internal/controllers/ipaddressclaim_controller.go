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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1alpha1"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/unifi"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/pkg/ipamutil"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/pkg/predicates"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

// UnifiProviderAdapter implements the ipamutil.ProviderAdapter interface.
type UnifiProviderAdapter struct {
	client.Client
}

var _ ipamutil.ProviderAdapter = &UnifiProviderAdapter{}

// UnifiClaimHandler implements the ipamutil.ClaimHandler interface.
type UnifiClaimHandler struct {
	client.Client
	claim *ipamv1.IPAddressClaim
	pool  *ipamv1alpha1.UnifiIPPool
}

var _ ipamutil.ClaimHandler = &UnifiClaimHandler{}

// SetupWithManager sets up the controller with the Manager.
func (a *UnifiProviderAdapter) SetupWithManager(_ context.Context, b *ctrl.Builder) error {
	b.
		For(&ipamv1.IPAddressClaim{}, builder.WithPredicates(
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				claim, ok := object.(*ipamv1.IPAddressClaim)
				if !ok {
					return false
				}
				return claim.Spec.PoolRef.Kind == "UnifiIPPool" &&
					claim.Spec.PoolRef.APIGroup != nil &&
					*claim.Spec.PoolRef.APIGroup == ipamv1alpha1.GroupVersion.Group
			}),
		)).
		WithOptions(controller.Options{
			// To avoid race conditions when allocating IP addresses
			MaxConcurrentReconciles: 1,
		}).
		Watches(
			&ipamv1alpha1.UnifiIPPool{},
			handler.EnqueueRequestsFromMapFunc(a.unifiIPPoolToIPClaims),
			builder.WithPredicates(
				predicates.ResourceTransitionedToUnpaused(),
				predicates.PoolNoLongerEmpty(),
			),
		).
		Owns(&ipamv1.IPAddress{})

	return nil
}

// unifiIPPoolToIPClaims maps UnifiIPPool events to IPAddressClaim reconcile requests.
func (a *UnifiProviderAdapter) unifiIPPoolToIPClaims(ctx context.Context, obj client.Object) []reconcile.Request {
	pool, ok := obj.(*ipamv1alpha1.UnifiIPPool)
	if !ok {
		return nil
	}

	// List all claims in the same namespace that reference this pool
	claimList := &ipamv1.IPAddressClaimList{}
	if err := a.List(ctx, claimList, client.InNamespace(pool.Namespace)); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0)
	for _, claim := range claimList.Items {
		if claim.Spec.PoolRef.Name == pool.Name &&
			claim.Spec.PoolRef.Kind == "UnifiIPPool" &&
			claim.Spec.PoolRef.APIGroup != nil &&
			*claim.Spec.PoolRef.APIGroup == ipamv1alpha1.GroupVersion.Group {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      claim.Name,
					Namespace: claim.Namespace,
				},
			})
		}
	}

	return requests
}

// ClaimHandlerFor returns a ClaimHandler for the given claim.
func (a *UnifiProviderAdapter) ClaimHandlerFor(_ client.Client, claim *ipamv1.IPAddressClaim) ipamutil.ClaimHandler {
	return &UnifiClaimHandler{
		Client: a.Client,
		claim:  claim,
	}
}

// FetchPool fetches the UnifiIPPool referenced by the claim.
func (h *UnifiClaimHandler) FetchPool(ctx context.Context) (client.Object, *ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	pool := &ipamv1alpha1.UnifiIPPool{}
	poolKey := types.NamespacedName{
		Name:      h.claim.Spec.PoolRef.Name,
		Namespace: h.claim.Namespace,
	}

	if err := h.Get(ctx, poolKey, pool); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "pool not found", "pool", poolKey)
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to fetch pool: %w", err)
	}

	h.pool = pool
	return pool, nil, nil
}

// EnsureAddress ensures that the IPAddress is allocated with a valid address.
func (h *UnifiClaimHandler) EnsureAddress(ctx context.Context, address *ipamv1.IPAddress) (*ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Get all addresses currently in use from this pool
	addressesInUse, err := poolutil.ListAddressesInUse(ctx, h.Client, h.pool.Namespace,
		h.pool.Name, "UnifiIPPool", ipamv1alpha1.GroupVersion.Group)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses in use: %w", err)
	}

	// Check if this address is already allocated
	for _, addr := range addressesInUse {
		if addr.Name == address.Name && addr.Namespace == address.Namespace {
			// Already allocated, nothing to do
			return nil, nil
		}
	}

	// Get UnifiInstance credentials
	instance := &ipamv1alpha1.UnifiInstance{}
	instanceKey := types.NamespacedName{
		Name:      h.pool.Spec.InstanceRef.Name,
		Namespace: h.pool.Spec.InstanceRef.Namespace,
	}
	if instanceKey.Namespace == "" {
		instanceKey.Namespace = h.pool.Namespace
	}

	if err := h.Get(ctx, instanceKey, instance); err != nil {
		return nil, fmt.Errorf("failed to fetch UnifiInstance: %w", err)
	}

	// Get credentials from secret
	var secret corev1.Secret
	if err := h.Client.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.CredentialsRef.Name,
		Namespace: h.pool.Namespace,
	}, &secret); err != nil {
		return nil, fmt.Errorf("failed to get credentials secret: %w", err)
	}

	// Create Unifi client
	unifiClient, err := unifi.NewClient(unifi.Config{
		Host:     instance.Spec.Host,
		APIKey:   string(secret.Data["apiKey"]),
		Site:     instance.Spec.Site,
		Insecure: instance.Spec.Insecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Unifi client: %w", err)
	}

	// Generate MAC address for this allocation
	// TODO: In production, use a more robust MAC generation or get from claim
	macAddress := generateMACAddress(h.claim.Name)

	// Use first subnet for now
	if len(h.pool.Spec.Subnets) == 0 {
		return nil, fmt.Errorf("pool has no subnets configured")
	}
	subnetSpec := &h.pool.Spec.Subnets[0]

	// Allocate IP from Unifi
	allocation, err := unifiClient.GetOrAllocateIP(
		ctx,
		h.pool.Spec.NetworkID,
		macAddress,
		h.claim.Name,
		subnetSpec,
		addressesInUse,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Update IPAddress spec
	address.Spec.Address = allocation.IPAddress
	address.Spec.Gateway = subnetSpec.Gateway
	if subnetSpec.Prefix > 0 {
		address.Spec.Prefix = subnetSpec.Prefix
	}

	logger.Info("allocated IP address",
		"claim", h.claim.Name,
		"address", allocation.IPAddress,
		"mac", macAddress)

	return nil, nil
}

// ReleaseAddress releases the IP address allocation.
func (h *UnifiClaimHandler) ReleaseAddress(ctx context.Context) (*ctrl.Result, error) {
	// The Unifi client's ReleaseIP method is a no-op for now
	// The IPAddress resource deletion already handles deallocation tracking
	return nil, nil
}

// generateMACAddress generates a simple MAC address based on the claim name.
// TODO: Use a more robust generation method in production.
func generateMACAddress(name string) string {
	return fmt.Sprintf("00:00:00:00:00:%02x", len(name)%256)
}
