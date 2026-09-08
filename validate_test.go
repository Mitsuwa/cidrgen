package cidrgen

import (
	"errors"
	"net/netip"
	"testing"
)

func mustPrefixes(t *testing.T, ss ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		p, err := parsePrefix(s)
		if err != nil {
			t.Fatalf("parsePrefix(%q): %v", s, err)
		}
		out[i] = p
	}
	return out
}

func TestValidateAllocated(t *testing.T) {
	parent := mustPrefixes(t, "10.0.0.0/16")[0]

	tests := []struct {
		name      string
		allocated []string
		wantErr   error
		wantOrder []string // expected sorted order on success
	}{
		{
			name:      "empty",
			allocated: nil,
			wantOrder: nil,
		},
		{
			name:      "disjoint, returned sorted",
			allocated: []string{"10.0.2.0/24", "10.0.0.0/24", "10.0.1.0/24"},
			wantOrder: []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"},
		},
		{
			name:      "adjacent blocks do not overlap",
			allocated: []string{"10.0.0.0/25", "10.0.0.128/25"},
			wantOrder: []string{"10.0.0.0/25", "10.0.0.128/25"},
		},
		{
			name:      "identical entries overlap",
			allocated: []string{"10.0.0.0/24", "10.0.0.0/24"},
			wantErr:   ErrOverlappingInput,
		},
		{
			name:      "nested entries overlap",
			allocated: []string{"10.0.0.0/24", "10.0.0.64/26"},
			wantErr:   ErrOverlappingInput,
		},
		{
			name:      "partial overlap",
			allocated: []string{"10.0.0.0/23", "10.0.1.0/24"},
			wantErr:   ErrOverlappingInput,
		},
		{
			name:      "entry outside parent",
			allocated: []string{"10.0.0.0/24", "192.168.1.0/24"},
			wantErr:   ErrOutOfParent,
		},
		{
			name:      "entry larger than parent",
			allocated: []string{"10.0.0.0/15"},
			wantErr:   ErrOutOfParent,
		},
		{
			name:      "parent-sized entry is inside parent",
			allocated: []string{"10.0.0.0/16"},
			wantOrder: []string{"10.0.0.0/16"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAllocated(parent, mustPrefixes(t, tt.allocated...))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.wantOrder) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.wantOrder))
			}
			for i, want := range tt.wantOrder {
				if got[i].String() != want {
					t.Fatalf("entry %d = %s, want %s", i, got[i], want)
				}
			}
		})
	}
}
