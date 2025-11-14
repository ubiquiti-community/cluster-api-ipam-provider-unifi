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

package ipamutil

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	clusterutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

const (
	// ReleaseAddressFinalizer is used to release an IP address before cleaning up the claim.
	ReleaseAddressFinalizer = "ipam.cluster.x-k8s.io/ReleaseAddress"

	// ProtectAddressFinalizer is used to prevent deletion of an IPAddress object while its claim is not deleted.
	ProtectAddressFinalizer = "ipam.cluster.x-k8s.io/ProtectAddress"
)

// ClaimReconciler reconciles an IPAddressClaim object using a ProviderAdapter.
type ClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	WatchFilterValue string

	Adapter ProviderAdapter
}

// ProviderAdapter is an interface that must be implemented by the IPAM provider.
type ProviderAdapter interface {
	// SetupWithManager will be called during the setup of the controller for the ClaimReconciler to allow the provider
	// implementation to extend the controller configuration.
	SetupWithManager(context.Context, *builder.Builder) error
	// ClaimHandlerFor is called during reconciliation to get a ClaimHandler for the reconciled [ipamv1.IPAddressClaim].
	ClaimHandlerFor(client.Client, *ipamv1.IPAddressClaim) ClaimHandler
}

// ClaimHandler knows how to allocate and release IP addresses for a specific provider.
type ClaimHandler interface {
	// FetchPool is called to fetch the pool referenced by the claim.
	FetchPool(ctx context.Context) (client.Object, *ctrl.Result, error)
	// EnsureAddress is called to make sure that the IPAddress.Spec is correct and the address is allocated.
	EnsureAddress(ctx context.Context, address *ipamv1.IPAddress) (*ctrl.Result, error)
	// ReleaseAddress is called to release the ip address that was allocated for the claim.
	ReleaseAddress(ctx context.Context) (*ctrl.Result, error)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClaimReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if r.Adapter == nil {
		return fmt.Errorf("error setting the manager: Adapter is nil")
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &ipamv1.IPAddressClaim{}, "clusterName", indexClusterName); err != nil {
		return fmt.Errorf("failed to register indexer for IPAddressClaim: %w", err)
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1.IPAddressClaim{}, builder.WithPredicates(
			predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetLogger(), r.WatchFilterValue),
		)).
		Watches(
			&clusterv1.Cluster{},
			ctrlhandler.EnqueueRequestsFromMapFunc(r.clusterToIPClaims),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldCluster, oldOK := e.ObjectOld.(*clusterv1.Cluster)
					newCluster, newOK := e.ObjectNew.(*clusterv1.Cluster)
					if !oldOK || !newOK {
						return false
					}
					return annotations.IsPaused(oldCluster, oldCluster) && !annotations.IsPaused(newCluster, newCluster)
				},
			}),
		)

	if err := r.Adapter.SetupWithManager(ctx, b); err != nil {
		return fmt.Errorf("failed to setup adapter: %w", err)
	}

	return b.Complete(r)
}

// Reconcile is called by the controller to reconcile a claim.
func (r *ClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	claim := &ipamv1.IPAddressClaim{}
	if err := r.Get(ctx, req.NamespacedName, claim); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if res, err := r.checkClusterPaused(ctx, claim); err != nil || res != nil {
		return unwrapResult(res), err
	}

	patchHelper, err := patch.NewHelper(claim, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patchHelper.Patch(ctx, claim); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if controllerutil.AddFinalizer(claim, ReleaseAddressFinalizer) {
		return ctrl.Result{}, nil
	}

	handler := r.Adapter.ClaimHandlerFor(r.Client, claim)

	if !claim.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, claim, handler)
	}

	return r.reconcileNormal(ctx, claim, handler)
}

func (r *ClaimReconciler) checkClusterPaused(ctx context.Context, claim *ipamv1.IPAddressClaim) (*ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	_, hasClusterLabel := claim.GetLabels()[clusterv1.ClusterNameLabel]
	if !hasClusterLabel {
		return nil, nil
	}

	cluster, err := clusterutil.GetClusterFromMetadata(ctx, r.Client, claim.ObjectMeta)
	if apierrors.IsNotFound(err) {
		// Cluster not found, continue anyway (claim might not be cluster-scoped).
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if cluster == nil || !annotations.IsPaused(cluster, cluster) {
		return nil, nil
	}

	if !claim.DeletionTimestamp.IsZero() {
		// Allow deletion even if cluster is paused.
		return nil, nil
	}

	log.Info("IPAddressClaim linked to a cluster that is paused, skipping reconciliation")
	res := ctrl.Result{}
	return &res, nil
}

func (r *ClaimReconciler) reconcileNormal(ctx context.Context, claim *ipamv1.IPAddressClaim, handler ClaimHandler) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	pool, res, err := handler.FetchPool(ctx)
	if err != nil || res != nil {
		return r.handlePoolFetchError(ctx, claim, handler, err, res)
	}

	if pool == nil {
		err := fmt.Errorf("pool is nil")
		log.Error(err, "pool error")
		return ctrl.Result{}, errors.Wrap(err, "reconciliation failed")
	}

	if annotations.HasPaused(pool) {
		log.Info("IPAddressClaim references Pool which is paused, skipping reconciliation.", "IPAddressClaim", claim.GetName(), "Pool", pool.GetName())
		return ctrl.Result{}, nil
	}

	address := NewIPAddress(claim, pool)

	operationResult, err := r.createOrPatchAddress(ctx, &address, claim, pool, handler)
	if err != nil {
		return ctrl.Result{}, err
	}

	if operationResult != controllerutil.OperationResultNone {
		log.Info("IPAddress successfully created or patched", "operation", operationResult)
	}

	if err := r.waitForAddressInCache(ctx, &address); err != nil {
		log.Info("Address is not yet visible in cache, requeueing", "error", err)
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, err
	}

	if address.DeletionTimestamp != nil {
		log.Info("Address is marked for deletion, but deletion is prevented until the claim is deleted as well", "address", address.Name)
	}

	claim.Status.AddressRef = corev1.LocalObjectReference{Name: address.Name}
	return ctrl.Result{}, nil
}

func (r *ClaimReconciler) handlePoolFetchError(ctx context.Context, claim *ipamv1.IPAddressClaim, handler ClaimHandler, err error, res *ctrl.Result) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if !apierrors.IsNotFound(err) {
		return unwrapResult(res), errors.Wrap(err, "failed to fetch pool")
	}

	err = fmt.Errorf("pool not found: %w", err)
	log.Error(err, "the referenced pool could not be found")

	if !claim.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, claim, handler)
	}

	return ctrl.Result{}, nil
}

func (r *ClaimReconciler) createOrPatchAddress(ctx context.Context, address *ipamv1.IPAddress, claim *ipamv1.IPAddressClaim, pool client.Object, handler ClaimHandler) (controllerutil.OperationResult, error) {
	var res *ctrl.Result

	operationResult, err := controllerutil.CreateOrPatch(ctx, r.Client, address, func() error {
		var err error
		if res, err = handler.EnsureAddress(ctx, address); err != nil {
			return err
		}

		if err = ensureIPAddressOwnerReferences(r.Scheme, address, claim, pool); err != nil {
			return errors.Wrap(err, "failed to ensure owner references on address")
		}

		r.copyClusterLabelToAddress(address, claim)
		_ = controllerutil.AddFinalizer(address, ProtectAddressFinalizer)

		return nil
	})

	if res != nil {
		return operationResult, errors.New("handler returned result during address creation")
	}
	if err != nil {
		return operationResult, errors.Wrap(err, "failed to create or patch address")
	}

	return operationResult, nil
}

func (r *ClaimReconciler) copyClusterLabelToAddress(address *ipamv1.IPAddress, claim *ipamv1.IPAddressClaim) {
	val, ok := claim.Labels[clusterv1.ClusterNameLabel]
	if !ok {
		return
	}

	if address.Labels == nil {
		address.Labels = make(map[string]string)
	}
	address.Labels[clusterv1.ClusterNameLabel] = val
}

func (r *ClaimReconciler) waitForAddressInCache(ctx context.Context, address *ipamv1.IPAddress) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		key := client.ObjectKeyFromObject(address)
		if err := r.Get(ctx, key, &ipamv1.IPAddress{}); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return true, nil
	})
}

func (r *ClaimReconciler) reconcileDelete(ctx context.Context, claim *ipamv1.IPAddressClaim, handler ClaimHandler) (ctrl.Result, error) {
	if res, err := handler.ReleaseAddress(ctx); err != nil {
		return unwrapResult(res), fmt.Errorf("release address: %w", err)
	}

	if err := r.deleteIPAddress(ctx, claim); err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(claim, ReleaseAddressFinalizer)
	return ctrl.Result{}, nil
}

func (r *ClaimReconciler) deleteIPAddress(ctx context.Context, claim *ipamv1.IPAddressClaim) error {
	address := &ipamv1.IPAddress{}
	namespacedName := types.NamespacedName{
		Namespace: claim.Namespace,
		Name:      claim.Name,
	}

	err := r.Get(ctx, namespacedName, address)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to fetch address")
	}

	if address.Name == "" {
		return nil
	}

	return r.removeAddressFinalizerAndDelete(ctx, address)
}

func (r *ClaimReconciler) removeAddressFinalizerAndDelete(ctx context.Context, address *ipamv1.IPAddress) error {
	p := client.MergeFrom(address.DeepCopy())
	if controllerutil.RemoveFinalizer(address, ProtectAddressFinalizer) {
		if err := r.Patch(ctx, address, p); err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to remove address finalizer")
		}
	}

	if err := r.Delete(ctx, address); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *ClaimReconciler) clusterToIPClaims(_ context.Context, o client.Object) []reconcile.Request {
	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		return nil
	}

	claimList := &ipamv1.IPAddressClaimList{}
	if err := r.List(context.Background(), claimList,
		client.InNamespace(cluster.Namespace),
		client.MatchingFields{"clusterName": cluster.Name},
	); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, len(claimList.Items))
	for i, claim := range claimList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: claim.Namespace,
				Name:      claim.Name,
			},
		}
	}
	return requests
}

func indexClusterName(object client.Object) []string {
	claim, ok := object.(*ipamv1.IPAddressClaim)
	if !ok {
		return nil
	}
	// In v1beta1, cluster name is only available via labels.
	if clusterName, ok := claim.Labels[clusterv1.ClusterNameLabel]; ok {
		return []string{clusterName}
	}
	return nil
}

func unwrapResult(res *ctrl.Result) ctrl.Result {
	if res == nil {
		return ctrl.Result{}
	}
	return *res
}

// NewIPAddress creates a new ipamv1.IPAddress with references to a pool and claim.
func NewIPAddress(claim *ipamv1.IPAddressClaim, pool client.Object) ipamv1.IPAddress {
	gvk := pool.GetObjectKind().GroupVersionKind()
	return ipamv1.IPAddress{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      claim.Name,
			Namespace: claim.Namespace,
		},
		Spec: ipamv1.IPAddressSpec{
			ClaimRef: corev1.LocalObjectReference{
				Name: claim.Name,
			},
			PoolRef: corev1.TypedLocalObjectReference{
				APIGroup: &gvk.Group,
				Kind:     gvk.Kind,
				Name:     pool.GetName(),
			},
		},
	}
}

// ensureIPAddressOwnerReferences ensures that an IPAddress has the
// IPAddressClaim and IPPool as an OwnerReference.
func ensureIPAddressOwnerReferences(scheme *runtime.Scheme, address *ipamv1.IPAddress, claim *ipamv1.IPAddressClaim, pool client.Object) error {
	if err := controllerutil.SetControllerReference(claim, address, scheme); err != nil {
		alreadyOwnedError := &controllerutil.AlreadyOwnedError{}
		if errors.As(err, &alreadyOwnedError) {
			return errors.Wrap(err, "Failed to update address's claim owner reference")
		}
	}

	if err := controllerutil.SetOwnerReference(pool, address, scheme); err != nil {
		return errors.Wrap(err, "Failed to update address's pool owner reference")
	}

	var poolRefIdx int
	poolGVK := pool.GetObjectKind().GroupVersionKind()
	for i, ownerRef := range address.GetOwnerReferences() {
		if ownerRef.APIVersion == poolGVK.GroupVersion().String() &&
			ownerRef.Kind == poolGVK.Kind &&
			ownerRef.Name == pool.GetName() {
			poolRefIdx = i
		}
	}

	address.OwnerReferences[poolRefIdx].Controller = ptr(false)
	address.OwnerReferences[poolRefIdx].BlockOwnerDeletion = ptr(true)

	return nil
}

func ptr[T any](v T) *T {
	return &v
}
