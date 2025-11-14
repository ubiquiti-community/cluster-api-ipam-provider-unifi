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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/poolutil"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/unifi"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

const (
	// ProtectPoolFinalizer is added to UnifiIPPool resources that have addresses in use.
	ProtectPoolFinalizer = "ipam.cluster.x-k8s.io/ProtectPool"

	// DefaultSyncInterval is how often to sync with Unifi controller.
	DefaultSyncInterval = 10 * time.Minute

	// Condition types for UnifiIPPool status.
	ConditionNetworkSynced = "NetworkSynced"
	ConditionReady         = "Ready"
	ConditionHealthy       = "Healthy"
	ConditionExhausted     = "Exhausted"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//nolint:cyclop // Reconciliation logic naturally has higher complexity
func (r *UnifiIPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	pool := &v1beta2.UnifiIPPool{}
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

	// Discover Unifi network if needed (when NetworkID not configured)
	if pool.Spec.NetworkID == "" && pool.Status.DiscoveredNetworkID == "" {
		if err := r.discoverNetwork(ctx, pool, instance, logger); err != nil {
			logger.Error(err, "failed to discover Unifi network")
			// Set condition and requeue
			r.updateNetworkDiscoveryCondition(pool, err)
			if err := r.Status().Update(ctx, pool); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	poolIPSet, err := r.buildPoolIPSet(pool, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	addressesInUse, err := poolutil.ListAddressesInUse(ctx, r.Client, pool.Namespace,
		pool.Name, "UnifiIPPool", v1beta2.GroupVersion.Group)
	if err != nil {
		logger.Error(err, "unable to list addresses in use")
		return ctrl.Result{}, err
	}

	// Perform periodic sync with Unifi to detect configuration drift
	if err := r.syncWithUnifi(ctx, pool, instance, logger); err != nil {
		logger.Error(err, "failed to sync with Unifi network")
		// Don't fail reconciliation on sync errors, but log and continue
		// The sync will be retried on the next reconciliation
	}

	if err := r.updatePoolStatus(ctx, pool, poolIPSet, addressesInUse, logger); err != nil {
		return ctrl.Result{}, err
	}

	// Update all conditions
	r.updateReadyCondition(pool, instance)
	r.updateHealthyCondition(pool)
	r.updateExhaustedCondition(pool)

	// Update status with all conditions
	if err := r.Status().Update(ctx, pool); err != nil {
		logger.Error(err, "unable to update pool conditions")
		return ctrl.Result{}, err
	}

	// Schedule next sync using RequeueAfter
	nextSync := r.calculateNextSyncInterval(pool)

	logger.V(1).Info("scheduling next Unifi sync", "after", nextSync)
	return ctrl.Result{RequeueAfter: nextSync}, nil
}

func (r *UnifiIPPoolReconciler) handleDeletion(ctx context.Context, pool *v1beta2.UnifiIPPool, logger logr.Logger) (ctrl.Result, error) {
	addressesInUse, err := poolutil.ListAddressesInUse(ctx, r.Client, pool.Namespace,
		pool.Name, "UnifiIPPool", v1beta2.GroupVersion.Group)
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

func (r *UnifiIPPoolReconciler) getUnifiInstance(ctx context.Context, pool *v1beta2.UnifiIPPool, logger logr.Logger) (*v1beta2.UnifiInstance, error) {
	instance := &v1beta2.UnifiInstance{}
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

func (r *UnifiIPPoolReconciler) buildPoolIPSet(pool *v1beta2.UnifiIPPool, logger logr.Logger) (*netipx.IPSet, error) {
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

func (r *UnifiIPPoolReconciler) updatePoolStatus(ctx context.Context, pool *v1beta2.UnifiIPPool, poolIPSet *netipx.IPSet, addressesInUse []ipamv1beta2.IPAddress, logger logr.Logger) error {
	// Compute basic address statistics
	pool.Status.Addresses = poolutil.ComputePoolStatus(poolIPSet, addressesInUse, pool.Namespace)

	// Calculate capacity metrics
	pool.Status.Capacity = r.calculateCapacityMetrics(pool.Status.Addresses)

	// Update allocation details
	pool.Status.AllocationDetails = r.buildAllocationDetails(addressesInUse, pool)

	// Update Status.Allocations map (claim name → IP address)
	// This enables IP reuse workflows by providing visibility into current assignments
	pool.Status.Allocations = make(map[string]string)
	for _, addr := range addressesInUse {
		if addr.Spec.ClaimRef.Name != "" && addr.Spec.Address != "" {
			pool.Status.Allocations[addr.Spec.ClaimRef.Name] = addr.Spec.Address
		}
	}

	// Add finalizer if addresses in use
	if len(addressesInUse) > 0 {
		if controllerutil.AddFinalizer(pool, ProtectPoolFinalizer) {
			if err := r.Update(ctx, pool); err != nil {
				logger.Error(err, "unable to add finalizer")
				return err
			}
		}
	}

	now := metav1.Now()
	pool.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, pool); err != nil {
		logger.Error(err, "unable to update UnifiIPPool status")
		return err
	}

	logger.Info("successfully reconciled UnifiIPPool",
		"pool", client.ObjectKeyFromObject(pool),
		"total", pool.Status.Addresses.Total,
		"used", pool.Status.Addresses.Used,
		"free", pool.Status.Addresses.Free,
		"utilization", pool.Status.Capacity.UtilizationPercent)

	return nil
}

// calculateCapacityMetrics computes pool utilization metrics.
func (r *UnifiIPPoolReconciler) calculateCapacityMetrics(summary *v1beta2.IPAddressStatusSummary) *v1beta2.PoolCapacity {
	if summary == nil || summary.Total == nil || *summary.Total == 0 {
		return &v1beta2.PoolCapacity{}
	}

	total := *summary.Total
	used := int32(0)
	if summary.Used != nil {
		used = *summary.Used
	}

	// Calculate utilization percentage
	utilizationPercent := (used * 100) / total
	highUtilization := utilizationPercent >= 80

	return &v1beta2.PoolCapacity{
		UtilizationPercent: &utilizationPercent,
		HighUtilization:    &highUtilization,
	}
}

// buildAllocationDetails creates detailed allocation information from IPAddress list.
func (r *UnifiIPPoolReconciler) buildAllocationDetails(addressesInUse []ipamv1beta2.IPAddress, _ *v1beta2.UnifiIPPool) *v1beta2.AllocationDetails {
	if len(addressesInUse) == 0 {
		return &v1beta2.AllocationDetails{
			AllocatedIPs: []v1beta2.AllocatedIP{},
		}
	}

	details := &v1beta2.AllocationDetails{
		AllocatedIPs: make([]v1beta2.AllocatedIP, 0, len(addressesInUse)),
	}

	var firstTime, lastTime *metav1.Time

	for _, addr := range addressesInUse {
		allocatedIP := v1beta2.AllocatedIP{
			Address: addr.Spec.Address,
		}

		// Extract claim name from claim ref
		if addr.Spec.ClaimRef.Name != "" {
			allocatedIP.ClaimName = addr.Spec.ClaimRef.Name
		}

		// Extract cluster name from labels
		if clusterName, ok := addr.Labels["cluster.x-k8s.io/cluster-name"]; ok {
			allocatedIP.ClusterName = clusterName
		}

		// Track allocation time
		creationTime := addr.GetCreationTimestamp()
		allocatedIP.AllocatedAt = &creationTime

		// Find first and last allocation times
		if firstTime == nil || creationTime.Before(firstTime) {
			firstTime = &creationTime
		}
		if lastTime == nil || creationTime.After(lastTime.Time) {
			lastTime = &creationTime
		}

		details.AllocatedIPs = append(details.AllocatedIPs, allocatedIP)
	}

	details.FirstAllocationTime = firstTime
	details.LastAllocationTime = lastTime

	return details
}

// ipAddressToUnifiIPPool maps IPAddress events to UnifiIPPool reconcile requests.
func (r *UnifiIPPoolReconciler) ipAddressToUnifiIPPool(_ context.Context, obj client.Object) []ctrl.Request {
	address, ok := obj.(*ipamv1beta2.IPAddress)
	if !ok {
		return nil
	}

	// Only reconcile if the address references a UnifiIPPool.
	if address.Spec.PoolRef.Kind != "UnifiIPPool" ||
		address.Spec.PoolRef.APIGroup != v1beta2.GroupVersion.Group {
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

// syncWithUnifi performs periodic synchronization with Unifi network configuration.
// This detects configuration drift and updates the pool's observed state.
//
//nolint:cyclop // Network sync logic requires multiple checks
func (r *UnifiIPPoolReconciler) syncWithUnifi(ctx context.Context, pool *v1beta2.UnifiIPPool, instance *v1beta2.UnifiInstance, logger logr.Logger) error {
	// Import unifi client package
	unifiClient, err := r.createUnifiClient(ctx, instance, pool.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create Unifi client: %w", err)
	}

	// Determine network ID to use (configured or discovered)
	networkID := pool.Spec.NetworkID
	if networkID == "" {
		networkID = pool.Status.DiscoveredNetworkID
	}
	if networkID == "" {
		return fmt.Errorf("no network ID available (neither configured nor discovered)")
	}

	// Sync network configuration from Unifi
	subnetSpec, err := unifiClient.SyncNetworkToCIDR(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to sync network config: %w", err)
	}

	// Get network details for DHCP info
	network, err := unifiClient.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network details: %w", err)
	}

	// Update network info
	pool.Status.NetworkInfo = &v1beta2.NetworkInfo{
		Name:         network.Name,
		Purpose:      network.Purpose,
		NetworkGroup: network.NetworkGroup,
	}

	// Add VLAN if configured
	if network.VLAN != 0 && network.VLAN <= 4094 { // Valid VLAN range
		vlan := int32(network.VLAN) // #nosec G115 - checked range
		pool.Status.NetworkInfo.VLAN = &vlan
	}

	// Add DHCP lease time if DHCP is enabled
	if network.DHCPDEnabled && network.DHCPDLeaseTime > 0 && network.DHCPDLeaseTime <= 2147483647 {
		leaseTime := int32(network.DHCPDLeaseTime) // #nosec G115 - checked range
		pool.Status.NetworkInfo.DHCPLeaseTime = &leaseTime
	}

	// Update observed network configuration
	pool.Status.ObservedNetworkConfiguration = &v1beta2.ObservedNetworkConfig{
		CIDR:        subnetSpec.CIDR,
		Gateway:     subnetSpec.Gateway,
		DHCPEnabled: &network.DHCPDEnabled,
	}

	if network.DHCPDEnabled && network.DHCPDStart != "" && network.DHCPDStop != "" {
		pool.Status.ObservedNetworkConfiguration.DHCPRange = &v1beta2.DHCPRangeConfig{
			Start: network.DHCPDStart,
			Stop:  network.DHCPDStop,
		}
	}

	// Detect configuration drift
	driftDetected := r.detectConfigurationDrift(pool, subnetSpec, logger)

	// Update sync condition
	r.updateSyncCondition(pool, driftDetected, err)

	// Update last sync time
	now := metav1.Now()
	pool.Status.LastSyncTime = &now

	if err := r.Status().Update(ctx, pool); err != nil {
		return fmt.Errorf("failed to update pool status: %w", err)
	}

	if driftDetected {
		logger.Info("configuration drift detected between pool and Unifi network",
			"pool_cidr", pool.Spec.Subnets[0].CIDR,
			"unifi_cidr", subnetSpec.CIDR)
	}

	return nil
}

// detectConfigurationDrift compares pool configuration with Unifi network state.
func (r *UnifiIPPoolReconciler) detectConfigurationDrift(pool *v1beta2.UnifiIPPool, unifiSpec *v1beta2.SubnetSpec, logger logr.Logger) bool {
	if len(pool.Spec.Subnets) == 0 {
		return false
	}

	poolSubnet := pool.Spec.Subnets[0]
	driftDetected := false

	// Check CIDR drift
	if poolSubnet.CIDR != unifiSpec.CIDR {
		logger.Info("CIDR drift detected", "pool", poolSubnet.CIDR, "unifi", unifiSpec.CIDR)
		driftDetected = true
	}

	// Check gateway drift
	if poolSubnet.Gateway != unifiSpec.Gateway {
		logger.Info("Gateway drift detected", "pool", poolSubnet.Gateway, "unifi", unifiSpec.Gateway)
		driftDetected = true
	}

	return driftDetected
}

// updateSyncCondition updates the NetworkSynced condition based on sync results.
func (r *UnifiIPPoolReconciler) updateSyncCondition(pool *v1beta2.UnifiIPPool, driftDetected bool, syncErr error) {
	condition := metav1.Condition{
		Type:               ConditionNetworkSynced,
		Status:             metav1.ConditionTrue,
		Reason:             "SyncSucceeded",
		Message:            "Pool configuration is synchronized with Unifi network",
		ObservedGeneration: pool.Generation,
		LastTransitionTime: metav1.Now(),
	}

	if syncErr != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SyncFailed"
		condition.Message = fmt.Sprintf("Failed to sync with Unifi: %v", syncErr)
	} else if driftDetected {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "ConfigurationDrift"
		condition.Message = "Pool configuration differs from Unifi network configuration"
	}

	r.setCondition(pool, condition)
}

// updateReadyCondition updates the Ready condition based on pool operational state.
func (r *UnifiIPPoolReconciler) updateReadyCondition(pool *v1beta2.UnifiIPPool, instance *v1beta2.UnifiInstance) {
	condition := metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "PoolReady",
		Message:            "Pool is ready for IP allocation",
		ObservedGeneration: pool.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// Check if instance is ready
	if instance.Status.Ready == nil || !*instance.Status.Ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "InstanceNotReady"
		condition.Message = "Unifi instance is not ready"
	}

	// Check if pool has subnets configured
	if len(pool.Spec.Subnets) == 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoSubnets"
		condition.Message = "Pool has no subnets configured"
	}

	r.setCondition(pool, condition)
}

// updateHealthyCondition updates the Healthy condition based on allocation health.
func (r *UnifiIPPoolReconciler) updateHealthyCondition(pool *v1beta2.UnifiIPPool) {
	condition := metav1.Condition{
		Type:               ConditionHealthy,
		Status:             metav1.ConditionTrue,
		Reason:             "PoolHealthy",
		Message:            "Pool is operating normally",
		ObservedGeneration: pool.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// Check for drift
	for _, cond := range pool.Status.Conditions {
		if cond.Type == ConditionNetworkSynced && cond.Status == metav1.ConditionFalse {
			condition.Status = metav1.ConditionFalse
			condition.Reason = cond.Reason
			condition.Message = fmt.Sprintf("Pool unhealthy: %s", cond.Message)
			break
		}
	}

	r.setCondition(pool, condition)
}

// updateExhaustedCondition updates the Exhausted condition based on capacity.
func (r *UnifiIPPoolReconciler) updateExhaustedCondition(pool *v1beta2.UnifiIPPool) {
	condition := metav1.Condition{
		Type:               ConditionExhausted,
		Status:             metav1.ConditionFalse,
		Reason:             "CapacityAvailable",
		Message:            "Pool has available capacity",
		ObservedGeneration: pool.Generation,
		LastTransitionTime: metav1.Now(),
	}

	// Check if pool is exhausted or nearly exhausted
	if pool.Status.Capacity != nil && pool.Status.Capacity.UtilizationPercent != nil {
		utilization := *pool.Status.Capacity.UtilizationPercent
		if utilization >= 100 {
			condition.Status = metav1.ConditionTrue
			condition.Reason = "PoolExhausted"
			condition.Message = "Pool has no available capacity"
		} else if utilization >= 90 {
			condition.Status = metav1.ConditionTrue
			condition.Reason = "NearlyExhausted"
			condition.Message = fmt.Sprintf("Pool is %d%% utilized - approaching exhaustion", utilization)
		}
	}

	r.setCondition(pool, condition)
}

// setCondition updates or appends a condition to the pool status.
func (r *UnifiIPPoolReconciler) setCondition(pool *v1beta2.UnifiIPPool, condition metav1.Condition) {
	for i, existing := range pool.Status.Conditions {
		if existing.Type == condition.Type {
			pool.Status.Conditions[i] = condition
			return
		}
	}
	pool.Status.Conditions = append(pool.Status.Conditions, condition)
}

// calculateNextSyncInterval determines when the next sync should occur.
func (r *UnifiIPPoolReconciler) calculateNextSyncInterval(pool *v1beta2.UnifiIPPool) time.Duration {
	// Use default sync interval
	// Could be made configurable via pool annotations in the future
	return DefaultSyncInterval
}

// createUnifiClient creates a Unifi client from instance credentials.
func (r *UnifiIPPoolReconciler) createUnifiClient(ctx context.Context, instance *v1beta2.UnifiInstance, namespace string) (*unifi.Client, error) {
	// Get credentials secret
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.CredentialsRef.Name,
		Namespace: namespace,
	}, &secret); err != nil {
		return nil, fmt.Errorf("failed to get credentials secret: %w", err)
	}

	site := DefaultUnifiSite
	if instance.Spec.Site != nil {
		site = *instance.Spec.Site
	}
	insecure := false
	if instance.Spec.Insecure != nil {
		insecure = *instance.Spec.Insecure
	}

	return unifi.NewClient(unifi.Config{
		Host:     instance.Spec.Host,
		APIKey:   string(secret.Data["apiKey"]),
		Site:     site,
		Insecure: insecure,
	})
}

// discoverNetwork attempts to auto-discover the Unifi network that contains the configured subnets.
func (r *UnifiIPPoolReconciler) discoverNetwork(ctx context.Context, pool *v1beta2.UnifiIPPool, instance *v1beta2.UnifiInstance, logger logr.Logger) error {
	if len(pool.Spec.Subnets) == 0 {
		return fmt.Errorf("no subnets configured in pool")
	}

	unifiClient, err := r.createUnifiClient(ctx, instance, pool.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create Unifi client: %w", err)
	}

	// Try to discover network for first subnet
	// (In future, could validate all subnets are in same network)
	firstSubnet := pool.Spec.Subnets[0]
	
	// Convert subnet to CIDR string if using Start/End notation
	subnetCIDR := firstSubnet.CIDR
	if subnetCIDR == "" && firstSubnet.Start != "" {
		// For Start/End ranges, construct a CIDR from the start address and prefix
		defaultPrefix := int32(24)
		if pool.Spec.Prefix != nil {
			defaultPrefix = *pool.Spec.Prefix
		}
		prefix := poolutil.GetPrefix(firstSubnet, defaultPrefix)
		subnetCIDR = fmt.Sprintf("%s/%d", firstSubnet.Start, prefix)
	}

	if subnetCIDR == "" {
		return fmt.Errorf("cannot determine subnet CIDR for network discovery")
	}

	network, err := unifiClient.FindNetworkForSubnet(ctx, subnetCIDR)
	if err != nil {
		return fmt.Errorf("failed to find network for subnet %s: %w", subnetCIDR, err)
	}

	// Update discovered network ID in status
	pool.Status.DiscoveredNetworkID = network.ID
	logger.Info("discovered Unifi network for pool",
		"network_id", network.ID,
		"network_name", network.Name,
		"subnet", subnetCIDR)

	return nil
}

// updateNetworkDiscoveryCondition sets the NetworkDiscovery condition based on discovery result.
func (r *UnifiIPPoolReconciler) updateNetworkDiscoveryCondition(pool *v1beta2.UnifiIPPool, err error) {
	condition := metav1.Condition{
		Type:               "NetworkDiscovered",
		ObservedGeneration: pool.Generation,
		LastTransitionTime: metav1.Now(),
	}

	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "DiscoveryFailed"
		condition.Message = fmt.Sprintf("Failed to discover Unifi network: %v", err)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "NetworkFound"
		condition.Message = fmt.Sprintf("Discovered network: %s", pool.Status.DiscoveredNetworkID)
	}

	r.setCondition(pool, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnifiIPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.UnifiIPPool{}).
		Watches(
			&ipamv1beta2.IPAddress{},
			handler.EnqueueRequestsFromMapFunc(r.ipAddressToUnifiIPPool),
		).
		Complete(r)
}
