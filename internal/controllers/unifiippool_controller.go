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
	"time"

	"github.com/go-logr/logr"
	"go4.org/netipx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"

	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

const (
	// ProtectPoolFinalizer is used to prevent pool deletion while addresses are allocated.
	ProtectPoolFinalizer = "ipam.cluster.x-k8s.io/ProtectPool"
)

// UnifiIPPoolReconciler reconciles a UnifiIPPool object.
type UnifiIPPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=unifiippools/finalizers,verbs=update
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses,verbs=get;list;watch

// Reconcile updates the UnifiIPPool status with address allocation statistics.
func (r *UnifiIPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	pool := &ipamv1beta2.UnifiIPPool{}
	if err := r.Get(ctx, req.NamespacedName, pool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch UnifiIPPool")
		return ctrl.Result{}, err
	}

	if !pool.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, pool, logger)
	}

	instance, err := r.getUnifiInstance(ctx, pool, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	if instance.Status.Ready == nil || !*instance.Status.Ready {
		logger.Info("waiting for UnifiInstance to be ready", "instance", client.ObjectKeyFromObject(instance))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	poolIPSet, err := r.buildPoolIPSet(pool, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	addressesInUse, err := poolutil.ListAddressesInUse(ctx, r.Client, pool.Namespace,
		pool.Name, "UnifiIPPool", ipamv1beta2.GroupVersion.Group)
	if err != nil {
		logger.Error(err, "unable to list addresses in use")
		return ctrl.Result{}, err
	}

	return r.updatePoolStatus(ctx, pool, poolIPSet, addressesInUse, logger)
}

func (r *UnifiIPPoolReconciler) handleDeletion(ctx context.Context, pool *ipamv1beta2.UnifiIPPool, logger logr.Logger) (ctrl.Result, error) {
	addressesInUse, err := poolutil.ListAddressesInUse(ctx, r.Client, pool.Namespace,
		pool.Name, "UnifiIPPool", ipamv1beta2.GroupVersion.Group)
	if err != nil {
		logger.Error(err, "unable to list addresses in use")
		return ctrl.Result{}, err
	}

	if len(addressesInUse) == 0 {
		if controllerutil.RemoveFinalizer(pool, ProtectPoolFinalizer) {
			if err := r.Update(ctx, pool); err != nil {
				logger.Error(err, "unable to remove finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		logger.Info("pool has addresses in use, waiting for cleanup", "count", len(addressesInUse))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func (r *UnifiIPPoolReconciler) getUnifiInstance(ctx context.Context, pool *ipamv1beta2.UnifiIPPool, logger logr.Logger) (*ipamv1beta2.UnifiInstance, error) {
	instance := &ipamv1beta2.UnifiInstance{}
	instanceKey := types.NamespacedName{
		Name:      pool.Spec.InstanceRef.Name,
		Namespace: pool.Spec.InstanceRef.Namespace,
	}
	if instanceKey.Namespace == "" {
		instanceKey.Namespace = pool.Namespace
	}

	if err := r.Get(ctx, instanceKey, instance); err != nil {
		logger.Error(err, "unable to fetch UnifiInstance", "instance", instanceKey)
		return nil, err
	}

	return instance, nil
}

func (r *UnifiIPPoolReconciler) buildPoolIPSet(pool *ipamv1beta2.UnifiIPPool, logger logr.Logger) (*netipx.IPSet, error) {
	if len(pool.Spec.Subnets) == 0 {
		err := fmt.Errorf("pool has no subnets configured")
		logger.Error(err, "invalid pool configuration")
		return nil, err
	}

	poolIPSet, err := poolutil.PoolSpecToIPSet(&pool.Spec.Subnets[0])
	if err != nil {
		logger.Error(err, "unable to convert pool spec to IPSet")
		return nil, err
	}

	return poolIPSet, nil
}

func (r *UnifiIPPoolReconciler) updatePoolStatus(ctx context.Context, pool *ipamv1beta2.UnifiIPPool, poolIPSet *netipx.IPSet, addressesInUse []ipamv1.IPAddress, logger logr.Logger) (ctrl.Result, error) {
	pool.Status.Addresses = poolutil.ComputePoolStatus(poolIPSet, addressesInUse, pool.Namespace)

	if len(addressesInUse) > 0 {
		if controllerutil.AddFinalizer(pool, ProtectPoolFinalizer) {
			if err := r.Update(ctx, pool); err != nil {
				logger.Error(err, "unable to add finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	now := metav1.Now()
	pool.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, pool); err != nil {
		logger.Error(err, "unable to update UnifiIPPool status")
		return ctrl.Result{}, err
	}

	logger.Info("successfully reconciled UnifiIPPool",
		"pool", client.ObjectKeyFromObject(pool),
		"total", pool.Status.Addresses.Total,
		"used", pool.Status.Addresses.Used,
		"free", pool.Status.Addresses.Free)

	return ctrl.Result{}, nil
}

// ipAddressToUnifiIPPool maps IPAddress events to UnifiIPPool reconcile requests.
func (r *UnifiIPPoolReconciler) ipAddressToUnifiIPPool(_ context.Context, obj client.Object) []ctrl.Request {
	address, ok := obj.(*ipamv1.IPAddress)
	if !ok {
		return nil
	}

	// Only reconcile if the address references a UnifiIPPool.
	if address.Spec.PoolRef.Kind != "UnifiIPPool" ||
		address.Spec.PoolRef.APIGroup == nil ||
		*address.Spec.PoolRef.APIGroup != ipamv1beta2.GroupVersion.Group {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      address.Spec.PoolRef.Name,
				Namespace: address.Namespace,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnifiIPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1beta2.UnifiIPPool{}).
		Watches(
			&ipamv1.IPAddress{},
			handler.EnqueueRequestsFromMapFunc(r.ipAddressToUnifiIPPool),
		).
		Complete(r)
}
