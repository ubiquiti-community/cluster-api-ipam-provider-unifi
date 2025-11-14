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
	"reflect"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/pkg/ipamutil"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

func TestUnifiProviderAdapter_SetupWithManager(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		b *ctrl.Builder
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
			a := &UnifiProviderAdapter{
				Client: tt.fields.Client,
			}
			if err := a.SetupWithManager(context.Background(), tt.args.b); (err != nil) != tt.wantErr {
				t.Errorf("UnifiProviderAdapter.SetupWithManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnifiProviderAdapter_unifiIPPoolToIPClaims(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		obj client.Object
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
			a := &UnifiProviderAdapter{
				Client: tt.fields.Client,
			}
			if got := a.unifiIPPoolToIPClaims(context.Background(), tt.args.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiProviderAdapter.unifiIPPoolToIPClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiProviderAdapter_ClaimHandlerFor(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		in0   client.Client
		claim *ipamv1beta2.IPAddressClaim
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   ipamutil.ClaimHandler
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &UnifiProviderAdapter{
				Client: tt.fields.Client,
			}
			if got := a.ClaimHandlerFor(tt.args.in0, tt.args.claim); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiProviderAdapter.ClaimHandlerFor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiClaimHandler_FetchPool(t *testing.T) {
	type fields struct {
		Client client.Client
		claim  *ipamv1beta2.IPAddressClaim
		pool   *v1beta2.UnifiIPPool
	}
	type args struct{}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    client.Object
		want1   *ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &UnifiClaimHandler{
				Client: tt.fields.Client,
				claim:  tt.fields.claim,
				pool:   tt.fields.pool,
			}
			got, got1, err := h.FetchPool(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiClaimHandler.FetchPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiClaimHandler.FetchPool() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("UnifiClaimHandler.FetchPool() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestUnifiClaimHandler_EnsureAddress(t *testing.T) {
	type fields struct {
		Client client.Client
		claim  *ipamv1beta2.IPAddressClaim
		pool   *v1beta2.UnifiIPPool
	}
	type args struct {
		address *ipamv1beta2.IPAddress
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &UnifiClaimHandler{
				Client: tt.fields.Client,
				claim:  tt.fields.claim,
				pool:   tt.fields.pool,
			}
			got, err := h.EnsureAddress(context.Background(), tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiClaimHandler.EnsureAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiClaimHandler.EnsureAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiClaimHandler_ReleaseAddress(t *testing.T) {
	type fields struct {
		Client client.Client
		claim  *ipamv1beta2.IPAddressClaim
		pool   *v1beta2.UnifiIPPool
	}
	type args struct{}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &UnifiClaimHandler{
				Client: tt.fields.Client,
				claim:  tt.fields.claim,
				pool:   tt.fields.pool,
			}
			got, err := h.ReleaseAddress(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiClaimHandler.ReleaseAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiClaimHandler.ReleaseAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateMACAddress(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateMACAddress(tt.args.name); got != tt.want {
				t.Errorf("generateMACAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
