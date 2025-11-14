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
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

func TestClaimReconciler_SetupWithManager(t *testing.T) {
	type fields struct {
		Client           client.Client
		Scheme           *runtime.Scheme
		WatchFilterValue string
		Adapter          ProviderAdapter
	}
	type args struct {
		mgr ctrl.Manager
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClaimReconciler{
				Client:           tt.fields.Client,
				Scheme:           tt.fields.Scheme,
				WatchFilterValue: tt.fields.WatchFilterValue,
				Adapter:          tt.fields.Adapter,
			}
			if err := r.SetupWithManager(context.Background(), tt.args.mgr); (err != nil) != tt.wantErr {
				t.Errorf("ClaimReconciler.SetupWithManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClaimReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client           client.Client
		Scheme           *runtime.Scheme
		WatchFilterValue string
		Adapter          ProviderAdapter
	}
	type args struct {
		req ctrl.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClaimReconciler{
				Client:           tt.fields.Client,
				Scheme:           tt.fields.Scheme,
				WatchFilterValue: tt.fields.WatchFilterValue,
				Adapter:          tt.fields.Adapter,
			}
			got, err := r.Reconcile(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClaimReconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClaimReconciler.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimReconciler_reconcileDelete(t *testing.T) {
	type fields struct {
		Client           client.Client
		Scheme           *runtime.Scheme
		WatchFilterValue string
		Adapter          ProviderAdapter
	}
	type args struct {
		claim   *ipamv1.IPAddressClaim
		handler ClaimHandler
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClaimReconciler{
				Client:           tt.fields.Client,
				Scheme:           tt.fields.Scheme,
				WatchFilterValue: tt.fields.WatchFilterValue,
				Adapter:          tt.fields.Adapter,
			}
			got, err := r.reconcileDelete(context.Background(), tt.args.claim, tt.args.handler)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClaimReconciler.reconcileDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClaimReconciler.reconcileDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimReconciler_clusterToIPClaims(t *testing.T) {
	type fields struct {
		Client           client.Client
		Scheme           *runtime.Scheme
		WatchFilterValue string
		Adapter          ProviderAdapter
	}
	type args struct {
		o client.Object
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []reconcile.Request
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClaimReconciler{
				Client:           tt.fields.Client,
				Scheme:           tt.fields.Scheme,
				WatchFilterValue: tt.fields.WatchFilterValue,
				Adapter:          tt.fields.Adapter,
			}
			if got := r.clusterToIPClaims(context.Background(), tt.args.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClaimReconciler.clusterToIPClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_indexClusterName(t *testing.T) {
	type args struct {
		object client.Object
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indexClusterName(tt.args.object); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("indexClusterName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unwrapResult(t *testing.T) {
	type args struct {
		res *ctrl.Result
	}
	tests := []struct {
		name string
		args args
		want ctrl.Result
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unwrapResult(tt.args.res); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unwrapResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewIPAddress(t *testing.T) {
	type args struct {
		claim *ipamv1.IPAddressClaim
		pool  client.Object
	}
	tests := []struct {
		name string
		args args
		want ipamv1.IPAddress
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewIPAddress(tt.args.claim, tt.args.pool); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIPAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ensureIPAddressOwnerReferences(t *testing.T) {
	type args struct {
		scheme  *runtime.Scheme
		address *ipamv1.IPAddress
		claim   *ipamv1.IPAddressClaim
		pool    client.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ensureIPAddressOwnerReferences(tt.args.scheme, tt.args.address, tt.args.claim, tt.args.pool); (err != nil) != tt.wantErr {
				t.Errorf("ensureIPAddressOwnerReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
