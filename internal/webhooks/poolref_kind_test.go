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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

// legacyPoolKind is the kind this provider served before the metal3-style
// rename. Nothing may match on it any more.
const legacyPoolKind = "UnifiIPPool"

func poolRefScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := v1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta2 to scheme: %v", err)
	}
	if err := ipamv1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add cluster-api ipam v1beta2 to scheme: %v", err)
	}
	return scheme
}

func addressForPool(kind string) *ipamv1beta2.IPAddress {
	return &ipamv1beta2.IPAddress{
		ObjectMeta: metav1.ObjectMeta{Name: "allocated", Namespace: "test-ns"},
		Spec: ipamv1beta2.IPAddressSpec{
			Address:  "192.168.1.10",
			ClaimRef: ipamv1beta2.IPAddressClaimReference{Name: "claim"},
			PoolRef: ipamv1beta2.IPPoolReference{
				Name:     "test-pool",
				Kind:     kind,
				APIGroup: v1beta2.GroupVersion.Group,
			},
		},
	}
}

// TestIPPoolWebhook_ValidateDelete_MatchesServedKind pins the third
// string-literal cluster: the delete webhook counts allocated addresses by
// poolRef.kind, so it must see the served kind and ignore the pre-rename one.
func TestIPPoolWebhook_ValidateDelete_MatchesServedKind(t *testing.T) {
	for _, tt := range []struct {
		name        string
		addressKind string
		wantErr     bool
	}{
		{name: "served kind blocks deletion", addressKind: v1beta2.IPPoolKind, wantErr: true},
		{name: "legacy kind does not", addressKind: legacyPoolKind, wantErr: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := poolRefScheme(t)
			c := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(addressForPool(tt.addressKind)).Build()

			w := &IPPool{Client: c}
			pool := &v1beta2.IPPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "test-ns"},
			}

			_, err := w.ValidateDelete(context.Background(), pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDelete() error = %v, want error = %v (addresses of kind %q)",
					err, tt.wantErr, tt.addressKind)
			}
		})
	}
}

// TestNoLegacyKindLiteralsRemain is the belt to the behavioral braces: it fails
// if any Go source under internal/ or cmd/ resurrects a hard-coded pre-rename
// kind name. Kind comparisons must go through v1beta2.IPPoolKind /
// v1beta2.InstanceKind, which the compiler can then keep honest.
func TestNoLegacyKindLiteralsRemain(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve repository root: %v", err)
	}

	for _, dir := range []string{"internal", "cmd"} {
		if err := filepath.WalkDir(filepath.Join(root, dir), goSourceChecker(t, root)); err != nil {
			t.Fatalf("failed to walk %s: %v", dir, err)
		}
	}
}

// goSourceChecker returns a WalkDirFunc that reports any Go source still
// carrying a pre-rename kind literal. The poolref_kind pin tests name those
// literals on purpose and are skipped.
func goSourceChecker(t *testing.T, root string) fs.WalkDirFunc {
	t.Helper()

	const selfBase = "poolref_kind_test.go"
	legacy := []string{`"UnifiIPPool"`, `"UnifiInstance"`, `"unifiippools"`, `"unifiinstances"`}

	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || filepath.Base(path) == selfBase {
			return nil
		}
		src, err := os.ReadFile(path) // #nosec G304 - paths come from walking the repo tree
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, lit := range legacy {
			if strings.Contains(string(src), lit) {
				t.Errorf("%s still contains the pre-rename literal %s; use the v1beta2 kind consts instead", rel, lit)
			}
		}
		return nil
	}
}
