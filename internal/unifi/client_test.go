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
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

func TestAPIClient_ValidateCredentials(t *testing.T) {
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
			c := &APIClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			if err := c.ValidateCredentials(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("APIClient.ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIClient_GetNetwork(t *testing.T) {
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
			c := &APIClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			got, err := c.GetNetwork(context.Background(), tt.args.networkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("APIClient.GetNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("APIClient.GetNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAPIClient_GetOrAllocateIP tests GetOrAllocateIP function.
// TODO: Update test to match new signature with pool and claim parameters
/*
func TestAPIClient_GetOrAllocateIP(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		pool           *v1beta2.IPPool
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
				t.Errorf("APIClient.GetOrAllocateIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("APIClient.GetOrAllocateIP() = %v, want %v", got, tt.want)
			}
		})
	}
*/

// TestAPIClient_allocateNextIP tests the allocateNextIP function.
// TODO: Update test to match new signature with context, pool, claim, and 4 return values (ip, prefix, gateway, error)
/*
func TestAPIClient_allocateNextIP(t *testing.T) {
	type fields struct {
		client *unifi.ApiClient
		site   string
	}
	type args struct {
		ctx            context.Context
		pool           *v1beta2.IPPool
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
				t.Errorf("APIClient.allocateNextIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIP != tt.wantIP {
				t.Errorf("APIClient.allocateNextIP() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("APIClient.allocateNextIP() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotGateway != tt.wantGateway {
				t.Errorf("APIClient.allocateNextIP() gotGateway = %v, want %v", gotGateway, tt.wantGateway)
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
				DHCPDDNS1: new(""),
				DHCPDDNS2: new(""),
				DHCPDDNS3: new(""),
				DHCPDDNS4: new(""),
			},
			want: []string{},
		},
		{
			name: "empty and nil slots are skipped, order preserved",
			network: &unifi.Network{
				DHCPDDNS1: new(""),
				DHCPDDNS2: new("1.1.1.1"),
				DHCPDDNS3: nil,
				DHCPDDNS4: new("9.9.9.9"),
			},
			want: []string{"1.1.1.1", "9.9.9.9"},
		},
		{
			name: "all slots configured",
			network: &unifi.Network{
				DHCPDDNS1: new("1.1.1.1"),
				DHCPDDNS2: new("1.0.0.1"),
				DHCPDDNS3: new("8.8.8.8"),
				DHCPDDNS4: new("8.8.4.4"),
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

// TestNewAPIClient_DefaultsSite covers the one piece of behavior NewAPIClient owns
// itself; everything else it does is handed to unifi.New.
func TestNewAPIClient_DefaultsSite(t *testing.T) {
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
			got, err := NewAPIClient(Config{Host: srv.URL, APIKey: "test-key", Site: tt.site})
			if err != nil {
				t.Fatalf("NewAPIClient() unexpected error = %v", err)
			}
			if got.site != tt.wantSite {
				t.Errorf("NewAPIClient() site = %q, want %q", got.site, tt.wantSite)
			}
			if got.api == nil {
				t.Error("NewAPIClient() did not build a go-unifi APIClient")
			}
		})
	}
}

// TestNewAPIClient_RejectsBadBaseURL checks that a construction failure from
// go-unifi is surfaced rather than swallowed.
func TestNewAPIClient_RejectsBadBaseURL(t *testing.T) {
	if _, err := NewAPIClient(Config{Host: "http://unifi.example.com/api", APIKey: "test-key"}); err == nil {
		t.Error("NewAPIClient() with a base URL ending in /api: got nil error, want an error")
	}
}

func TestAPIClient_ReleaseIP(t *testing.T) {
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
			c := &APIClient{
				api:  tt.fields.client,
				site: tt.fields.site,
			}
			if err := c.ReleaseIP(context.Background(), tt.args.networkID, tt.args.ipAddress, tt.args.macAddress); (err != nil) != tt.wantErr {
				t.Errorf("APIClient.ReleaseIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// fakeController is a stand-in UniFi controller serving the handful of REST
// endpoints GetOrAllocateIP touches, so the real go-unifi client (and its real
// error types) sit between the test and the code under test.
type fakeController struct {
	// clients is what GET rest/user returns.
	clients []unifi.Client
	// networks is what GET rest/networkconf returns.
	networks []unifi.Network
	// clientGetFailures makes the first N GETs of rest/user answer with
	// clientGetStatus, so a lookup can fail while later calls succeed.
	clientGetFailures int
	clientGetStatus   int

	clientGets int
	created    []unifi.Client
	// updated records every PUT to rest/user/{id}, with ID set from the path.
	updated []unifi.Client
}

func (f *fakeController) start(t *testing.T) *APIClient {
	t.Helper()

	const base = "/proxy/network/api/s/default/rest/"
	mux := http.NewServeMux()
	mux.HandleFunc(base+"user", f.usersHandler(t))
	mux.HandleFunc(base+"user/", f.userByIDHandler(t, base+"user/"))
	mux.HandleFunc(base+"networkconf", func(w http.ResponseWriter, _ *http.Request) {
		writeUnifiData(t, w, f.networks)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := NewAPIClient(Config{Host: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewAPIClient() unexpected error = %v", err)
	}
	return c
}

// usersHandler serves GET (list) and POST (create) on rest/user.
func (f *fakeController) usersHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			f.clientGets++
			if f.clientGets <= f.clientGetFailures {
				w.WriteHeader(f.clientGetStatus)
				return
			}
			writeUnifiData(t, w, f.clients)
		case http.MethodPost:
			var created unifi.Client
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Errorf("fake controller could not decode create body: %v", err)
			}
			f.created = append(f.created, created)
			writeUnifiData(t, w, []unifi.Client{created})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

// userByIDHandler serves GET and PUT (update) on rest/user/{id}.
func (f *fakeController) userByIDHandler(t *testing.T, prefix string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, prefix)
		switch r.Method {
		case http.MethodPut:
			var updated unifi.Client
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Errorf("fake controller could not decode update body: %v", err)
			}
			updated.ID = id
			f.updated = append(f.updated, updated)
			writeUnifiData(t, w, []unifi.Client{updated})
		case http.MethodGet:
			for _, c := range f.clients {
				if c.ID == id {
					writeUnifiData(t, w, []unifi.Client{c})
					return
				}
			}
			writeUnifiData(t, w, []unifi.Client{})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func writeUnifiData[T any](t *testing.T, w http.ResponseWriter, data []T) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		Meta struct {
			RC string `json:"rc"`
		} `json:"meta"`
		Data []T `json:"data"`
	}{
		Meta: struct {
			RC string `json:"rc"`
		}{RC: "ok"},
		Data: data,
	}); err != nil {
		t.Errorf("fake controller could not encode response: %v", err)
	}
}

func testPool() *v1beta2.IPPool {
	return &v1beta2.IPPool{
		Spec: v1beta2.IPPoolSpec{
			Subnets: []v1beta2.SubnetSpec{{CIDR: "192.168.1.0/24"}},
			Gateway: "192.168.1.1",
		},
	}
}

func testNetworks() []unifi.Network {
	return []unifi.Network{{
		ID: "net-1",
		// Purpose drives Network's custom marshaller; without a valid one the
		// fake controller cannot even encode the network.
		Purpose:  unifi.PurposeCorporate,
		IPSubnet: new("192.168.1.0/24"),
	}}
}

// TestAPIClient_GetOrAllocateIP_NotFoundAllocates covers the normal
// first-allocation case. go-unifi reports "this MAC has no assignment" by
// returning *unifi.NotFoundError from GetClientByMAC -- which is not a failure,
// it is the signal to allocate.
func TestAPIClient_GetOrAllocateIP_NotFoundAllocates(t *testing.T) {
	tests := []struct {
		name    string
		clients []unifi.Client
	}{
		// GetClientByMAC returns NotFoundError for both of these.
		{name: "no clients at all", clients: nil},
		{
			name:    "clients exist but none match the MAC",
			clients: []unifi.Client{{MAC: "02:aa:bb:cc:dd:ee", FixedIP: "192.168.1.9"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeController{clients: tt.clients, networks: testNetworks()}
			c := f.start(t)

			got, err := c.GetOrAllocateIP(context.Background(), testPool(), nil,
				"net-1", "02:11:22:33:44:55", "host-1", nil)
			if err != nil {
				t.Fatalf("GetOrAllocateIP() unexpected error = %v", err)
			}
			if got == nil {
				t.Fatal("GetOrAllocateIP() returned no allocation")
			}
			if got.IPAddress == "" {
				t.Error("GetOrAllocateIP() allocated an empty IP address")
			}
			if got.IPAddress == "192.168.1.1" {
				t.Error("GetOrAllocateIP() allocated the gateway address")
			}
			if got.MacAddress != "02:11:22:33:44:55" {
				t.Errorf("GetOrAllocateIP() MacAddress = %q, want the requested MAC", got.MacAddress)
			}
			if len(f.created) != 1 {
				t.Fatalf("expected exactly one client to be created, got %d", len(f.created))
			}
			if !f.created[0].UseFixedIP || f.created[0].FixedIP != got.IPAddress {
				t.Errorf("created client = %+v, want UseFixedIP with FixedIP %q", f.created[0], got.IPAddress)
			}
		})
	}
}

// TestAPIClient_GetOrAllocateIP_LookupErrorPropagates covers the opposite case:
// a genuine failure looking the MAC up says nothing about whether the MAC is
// assigned, so it must not be mistaken for "unassigned" and must not allocate.
// Only the first lookup fails here, so a caller that ignores the error would
// sail on and allocate successfully.
func TestAPIClient_GetOrAllocateIP_LookupErrorPropagates(t *testing.T) {
	f := &fakeController{
		networks:          testNetworks(),
		clientGetFailures: 1,
		clientGetStatus:   http.StatusUnauthorized,
	}
	c := f.start(t)

	got, err := c.GetOrAllocateIP(context.Background(), testPool(), nil,
		"net-1", "02:11:22:33:44:55", "host-1", nil)
	if err == nil {
		t.Fatalf("GetOrAllocateIP() returned no error for a failed lookup; got allocation %+v", got)
	}

	loginRequired := &unifi.LoginRequiredError{}
	if !errors.As(err, &loginRequired) {
		t.Errorf("GetOrAllocateIP() error = %v, want it to wrap *unifi.LoginRequiredError", err)
	}
	if len(f.created) != 0 {
		t.Errorf("expected no client to be created on a failed lookup, got %d", len(f.created))
	}
}

// TestAPIClient_GetOrAllocateIP_ReusesExisting guards the path that already
// worked: a MAC with an assignment gets that assignment back, with no write.
func TestAPIClient_GetOrAllocateIP_ReusesExisting(t *testing.T) {
	f := &fakeController{
		networks: testNetworks(),
		clients: []unifi.Client{{
			MAC:        "02:11:22:33:44:55",
			FixedIP:    "192.168.1.50",
			Hostname:   "existing-host",
			UseFixedIP: true,
			NetworkID:  "net-1",
		}},
	}
	c := f.start(t)

	got, err := c.GetOrAllocateIP(context.Background(), testPool(), nil,
		"net-1", "02:11:22:33:44:55", "host-1", nil)
	if err != nil {
		t.Fatalf("GetOrAllocateIP() unexpected error = %v", err)
	}
	want := &IPAllocation{
		IPAddress:  "192.168.1.50",
		MacAddress: "02:11:22:33:44:55",
		Hostname:   "existing-host",
		UseFixedIP: true,
		Prefix:     24,
		Gateway:    "192.168.1.1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetOrAllocateIP() = %+v, want %+v", got, want)
	}
	if len(f.created) != 0 {
		t.Errorf("expected no client to be created when reusing, got %d", len(f.created))
	}
}

// TestAPIClient_GetOrAllocateIP_SeesFixedIPsWithoutNetworkID reproduces the
// api.err.DuplicateFixedIP loop seen in the field. A reservation made from the
// UniFi UI carries no network_id, yet UniFi enforces it site-wide, so the
// allocator must count it as taken instead of offering it again.
func TestAPIClient_GetOrAllocateIP_SeesFixedIPsWithoutNetworkID(t *testing.T) {
	f := &fakeController{
		networks: testNetworks(),
		clients: []unifi.Client{{
			MAC:        "88:a2:9e:87:76:6c",
			FixedIP:    "192.168.1.2",
			UseFixedIP: true,
			// No NetworkID: this is what a UI-created reservation looks like.
		}},
	}
	c := f.start(t)

	got, err := c.GetOrAllocateIP(context.Background(), testPool(), nil,
		"net-1", "02:11:22:33:44:55", "host-1", nil)
	if err != nil {
		t.Fatalf("GetOrAllocateIP() unexpected error = %v", err)
	}
	if got.IPAddress != "192.168.1.3" {
		t.Errorf("GetOrAllocateIP() = %q, want 192.168.1.3 because 192.168.1.2 is reserved in UniFi", got.IPAddress)
	}
}

// TestAPIClient_GetOrAllocateIP_MovesKnownClientIntoPool covers a MAC UniFi
// already knows (a real device) whose reservation is outside the pool, or which
// has none. It must get an address from the pool written onto its existing
// record: a second record for a known MAC is exactly what UniFi rejects.
func TestAPIClient_GetOrAllocateIP_MovesKnownClientIntoPool(t *testing.T) {
	tests := []struct {
		name     string
		existing unifi.Client
	}{
		{
			name: "reservation outside the pool",
			existing: unifi.Client{
				ID: "u1", MAC: "f4:4d:30:6f:a7:93", Name: "talos-10-1-40-21",
				FixedIP: "10.1.40.21", UseFixedIP: true,
			},
		},
		{
			name:     "no reservation",
			existing: unifi.Client{ID: "u1", MAC: "f4:4d:30:6f:a7:93", Name: "talos-10-1-40-21"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeController{networks: testNetworks(), clients: []unifi.Client{tt.existing}}
			c := f.start(t)

			got, err := c.GetOrAllocateIP(context.Background(), testPool(), nil,
				"net-1", "f4:4d:30:6f:a7:93", "host-1", nil)
			if err != nil {
				t.Fatalf("GetOrAllocateIP() unexpected error = %v", err)
			}

			want := &IPAllocation{
				IPAddress:  "192.168.1.2",
				MacAddress: "f4:4d:30:6f:a7:93",
				UseFixedIP: true,
				Prefix:     24,
				Gateway:    "192.168.1.1",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("GetOrAllocateIP() = %+v, want %+v", got, want)
			}
			if len(f.created) != 0 {
				t.Errorf("expected no client to be created for a known MAC, got %d", len(f.created))
			}

			// The pool address lands on the existing record; everything else on
			// it, the user's alias included, is left as it was.
			wantRecord := tt.existing
			wantRecord.FixedIP = "192.168.1.2"
			wantRecord.UseFixedIP = true
			wantRecord.NetworkID = "net-1"
			if !reflect.DeepEqual(f.updated, []unifi.Client{wantRecord}) {
				t.Errorf("updated records = %+v, want exactly %+v", f.updated, wantRecord)
			}
		})
	}
}

func newTestClaim(name string, annotations map[string]string) *ipamv1beta2.IPAddressClaim {
	return &ipamv1beta2.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
	}
}

func TestMACForClaim_Annotated(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
		wantErr     bool
	}{
		{
			name:        "uses the provider annotation, normalized to lower case",
			annotations: map[string]string{v1beta2.MACAddressAnnotation: "F4:4D:30:6F:A7:93"},
			want:        "f4:4d:30:6f:a7:93",
		},
		{
			name:        "recognizes the CAPT annotation",
			annotations: map[string]string{"capt.tinkerbell.org/mac-address": "b8:ae:ed:76:79:61"},
			want:        "b8:ae:ed:76:79:61",
		},
		{
			name:        "rejects an annotation that is not a MAC",
			annotations: map[string]string{v1beta2.MACAddressAnnotation: "not-a-mac"},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MACForClaim(newTestClaim("c", tt.annotations))
			if (err != nil) != tt.wantErr {
				t.Fatalf("MACForClaim() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("MACForClaim() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMACForClaim_DerivesWhenUnannotated pins the fallback: a stable, valid,
// locally administered MAC that differs between claims. The two names here have
// the same length, which is exactly what made the old length-based scheme hand
// both nodes the same MAC.
func TestMACForClaim_DerivesWhenUnannotated(t *testing.T) {
	a, err := MACForClaim(newTestClaim("discovery-f4-4d-30-6f-a7-93-talos-nodes", nil))
	if err != nil {
		t.Fatalf("MACForClaim() unexpected error = %v", err)
	}
	b, _ := MACForClaim(newTestClaim("discovery-b8-ae-ed-76-79-61-talos-nodes", nil))
	again, _ := MACForClaim(newTestClaim("discovery-f4-4d-30-6f-a7-93-talos-nodes", nil))

	if a == b {
		t.Errorf("two claims of equal name length derived the same MAC %q", a)
	}
	if a != again {
		t.Errorf("MACForClaim() is not deterministic: %q then %q", a, again)
	}
	if !strings.HasPrefix(a, "02:") {
		t.Errorf("MACForClaim() = %q, want a locally administered 02: prefix", a)
	}
	if _, err := net.ParseMAC(a); err != nil {
		t.Errorf("MACForClaim() = %q is not a valid MAC: %v", a, err)
	}
}

// TestAPIClient_GetOrAllocateIP_SkipsPreAllocatedAddresses: an address reserved
// in spec.preAllocations for one claim must not be handed to another claim by
// dynamic allocation, whether or not the owning claim exists yet.
func TestAPIClient_GetOrAllocateIP_SkipsPreAllocatedAddresses(t *testing.T) {
	f := &fakeController{networks: testNetworks()}
	c := f.start(t)

	pool := testPool()
	pool.Spec.PreAllocations = map[string]string{"other-claim": "192.168.1.2"}

	got, err := c.GetOrAllocateIP(context.Background(), pool, newTestClaim("this-claim", nil),
		"net-1", "02:11:22:33:44:55", "host-1", nil)
	if err != nil {
		t.Fatalf("GetOrAllocateIP() unexpected error = %v", err)
	}
	if got.IPAddress != "192.168.1.3" {
		t.Errorf("GetOrAllocateIP() = %q, want 192.168.1.3 because 192.168.1.2 is pre-allocated to other-claim", got.IPAddress)
	}
}
