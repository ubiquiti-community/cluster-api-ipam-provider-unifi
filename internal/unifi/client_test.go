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

package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

func TestNewApiClient(t *testing.T) {
	type args struct {
		cfg Config
	}
	tests := []struct {
		name    string
		args    args
		want    *ApiClient
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewApiClient(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewApiClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewApiClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApiClient_ValidateCredentials(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct{}
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
			c := &ApiClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			if err := c.ValidateCredentials(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("ApiClient.ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApiClient_GetNetwork(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		networkID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *unifi.Network
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ApiClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			got, err := c.GetNetwork(context.Background(), tt.args.networkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApiClient.GetNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApiClient.GetNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApiClient_GetOrAllocateIP tests GetOrAllocateIP function.
// TODO: Update test to match new signature with pool and claim parameters
/*
func TestApiClient_GetOrAllocateIP(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		pool           *v1beta2.UnifiIPPool
		claim          *ipamv1beta2.IPAddressClaim
		networkID      string
		macAddress     string
		hostname       string
		addressesInUse []ipamv1beta2.IPAddress
	}
	tests := []struct {
		name    string
		fields  args    want    *IPAllocation
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			got, err := c.GetOrAllocateIP(context.Background(), tt.args.pool, tt.args.claim, tt.args.networkID, tt.args.macAddress, tt.args.hostname, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApiClient.GetOrAllocateIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApiClient.GetOrAllocateIP() = %v, want %v", got, tt.want)
			}
		})
	}
*/

// TestApiClient_allocateNextIP tests the allocateNextIP function.
// TODO: Update test to match new signature with context, pool, claim, and 4 return values (ip, prefix, gateway, error)
/*
func TestApiClient_allocateNextIP(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		ctx            context.Context
		pool           *v1beta2.UnifiIPPool
		claim          *ipamv1beta2.IPAddressClaim
		network        *unifi.Network
		addressesInUse []ipamv1beta2.IPAddress
	}
	tests := []struct {
		name        string
		fields      args        wantIP      string
		wantPrefix  int32
		wantGateway string
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			gotIP, gotPrefix, gotGateway, err := c.allocateNextIP(tt.args.ctx, tt.args.pool, tt.args.claim, tt.args.network, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApiClient.allocateNextIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIP != tt.wantIP {
				t.Errorf("ApiClient.allocateNextIP() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("ApiClient.allocateNextIP() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotGateway != tt.wantGateway {
				t.Errorf("ApiClient.allocateNextIP() gotGateway = %v, want %v", gotGateway, tt.wantGateway)
			}
		})
	}
*/

func TestDerefString(t *testing.T) {
	value := "10.0.0.1"
	empty := ""

	tests := []struct {
		name string
		in   *string
		want string
	}{
		{name: "nil pointer is unset", in: nil, want: ""},
		{name: "pointer to empty string is unset", in: &empty, want: ""},
		{name: "pointer to value yields the value", in: &value, want: "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DerefString(tt.in); got != tt.want {
				t.Errorf("DerefString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCollectDNSServers pins the unset semantics the pointer migration has to
// preserve: go-unifi v1.34.0 models an unconfigured DHCP DNS slot as either a
// nil *string or a pointer to "", and both must be skipped exactly the way the
// pre-migration `!= ""` check skipped a plain empty string.
func TestCollectDNSServers(t *testing.T) {
	tests := []struct {
		name    string
		network *unifi.Network
		want    []string
	}{
		{
			name:    "all slots nil",
			network: &unifi.Network{},
			want:    []string{},
		},
		{
			name: "all slots point at empty strings",
			network: &unifi.Network{
				DHCPDDNS1: unifi.Ptr(""),
				DHCPDDNS2: unifi.Ptr(""),
				DHCPDDNS3: unifi.Ptr(""),
				DHCPDDNS4: unifi.Ptr(""),
			},
			want: []string{},
		},
		{
			name: "empty and nil slots are skipped, order preserved",
			network: &unifi.Network{
				DHCPDDNS1: unifi.Ptr(""),
				DHCPDDNS2: unifi.Ptr("1.1.1.1"),
				DHCPDDNS3: nil,
				DHCPDDNS4: unifi.Ptr("9.9.9.9"),
			},
			want: []string{"1.1.1.1", "9.9.9.9"},
		},
		{
			name: "all slots configured",
			network: &unifi.Network{
				DHCPDDNS1: unifi.Ptr("1.1.1.1"),
				DHCPDDNS2: unifi.Ptr("1.0.0.1"),
				DHCPDDNS3: unifi.Ptr("8.8.8.8"),
				DHCPDDNS4: unifi.Ptr("8.8.4.4"),
			},
			want: []string{"1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collectDNSServers(tt.network); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectDNSServers() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewApiClient_DefaultsSite covers the one piece of behavior NewApiClient owns
// itself; everything else it does is handed to unifi.New.
func TestNewApiClient_DefaultsSite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
	}))
	defer srv.Close()

	tests := []struct {
		name     string
		site     string
		wantSite string
	}{
		{name: "empty site falls back to default", site: "", wantSite: "default"},
		{name: "explicit site is kept", site: "office", wantSite: "office"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewApiClient(Config{Host: srv.URL, APIKey: "test-key", Site: tt.site})
			if err != nil {
				t.Fatalf("NewApiClient() unexpected error = %v", err)
			}
			if got.site != tt.wantSite {
				t.Errorf("NewApiClient() site = %q, want %q", got.site, tt.wantSite)
			}
			if got.api == nil {
				t.Error("NewApiClient() did not build a go-unifi ApiClient")
			}
		})
	}
}

// TestNewApiClient_RejectsBadBaseURL checks that a construction failure from
// go-unifi is surfaced rather than swallowed.
func TestNewApiClient_RejectsBadBaseURL(t *testing.T) {
	if _, err := NewApiClient(Config{Host: "http://unifi.example.com/api", APIKey: "test-key"}); err == nil {
		t.Error("NewApiClient() with a base URL ending in /api: got nil error, want an error")
	}
}

func TestApiClient_ReleaseIP(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		networkID  string
		ipAddress  string
		macAddress string
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
			c := &ApiClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			if err := c.ReleaseIP(context.Background(), tt.args.networkID, tt.args.ipAddress, tt.args.macAddress); (err != nil) != tt.wantErr {
				t.Errorf("ApiClient.ReleaseIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
