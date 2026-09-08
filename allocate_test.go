package cidrgen

import (
	"errors"
	"net/netip"
	"testing"
)

// assertFits verifies the invariant every allocation must satisfy: got is inside
// parent and overlaps no entry in allocated.
func assertFits(t *testing.T, parent netip.Prefix, allocated []netip.Prefix, got netip.Prefix) {
	t.Helper()
	pStart, pEnd := prefixRange(parent)
	gStart, gEnd := prefixRange(got)
	if gStart < pStart || gEnd > pEnd {
		t.Fatalf("%s is not within parent %s", got, parent)
	}
	if gStart%(gEnd-gStart+1) != 0 {
		t.Fatalf("%s is not aligned to its own size", got)
	}
	for _, a := range allocated {
		aStart, aEnd := prefixRange(a)
		if gStart <= aEnd && aStart <= gEnd {
			t.Fatalf("%s overlaps allocated %s", got, a)
		}
	}
}

func TestFirstFit(t *testing.T) {
	tests := []struct {
		name      string
		parent    string
		allocated []string
		bits      int
		want      string
		wantErr   error
	}{
		{
			name:   "empty pool takes the lowest block",
			parent: "10.0.0.0/16", allocated: nil, bits: 24,
			want: "10.0.0.0/24",
		},
		{
			name:   "fills the first gap",
			parent: "10.0.0.0/16", allocated: []string{"10.0.0.0/24", "10.0.2.0/24"}, bits: 24,
			want: "10.0.1.0/24",
		},
		{
			name:   "skips a run of allocations",
			parent: "10.0.0.0/16", allocated: []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"}, bits: 24,
			want: "10.0.3.0/24",
		},
		{
			name:   "re-aligns past a smaller allocation",
			parent: "10.0.0.0/16", allocated: []string{"10.0.0.0/26"}, bits: 24,
			want: "10.0.1.0/24",
		},
		{
			name:   "smaller request slots into a sub-block gap",
			parent: "10.0.0.0/16", allocated: []string{"10.0.0.0/26", "10.0.0.128/26"}, bits: 26,
			want: "10.0.0.64/26",
		},
		{
			name:   "exact fit in the last slot",
			parent: "10.0.0.0/24", allocated: []string{"10.0.0.0/25"}, bits: 25,
			want: "10.0.0.128/25",
		},
		{
			name:   "full pool returns ErrNoSpace",
			parent: "10.0.0.0/24", allocated: []string{"10.0.0.0/25", "10.0.0.128/25"}, bits: 25,
			wantErr: ErrNoSpace,
		},
		{
			name:   "free space exists but no aligned gap large enough",
			parent: "10.0.0.0/24", allocated: []string{"10.0.0.0/26", "10.0.0.192/26"}, bits: 25,
			wantErr: ErrNoSpace,
		},
		{
			name:   "top of the address space",
			parent: "255.255.255.0/24", allocated: []string{"255.255.255.0/25"}, bits: 25,
			want: "255.255.255.128/25",
		},
		{
			name:   "whole space, single /1",
			parent: "0.0.0.0/0", allocated: []string{"0.0.0.0/1"}, bits: 1,
			want: "128.0.0.0/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := mustPrefixes(t, tt.parent)[0]
			alloc := mustPrefixes(t, tt.allocated...)
			sorted, err := validateAllocated(parent, alloc)
			if err != nil {
				t.Fatalf("validateAllocated: %v", err)
			}
			got, err := firstFit(parent, sorted, tt.bits)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("firstFit = %s, want %s", got, tt.want)
			}
			assertFits(t, parent, alloc, got)
		})
	}
}

// TestFirstFitSequential carves a /16 into /24s one at a time, feeding each
// result back in, and checks the invariant holds for all 256 allocations.
func TestFirstFitSequential(t *testing.T) {
	parent := mustPrefixes(t, "10.0.0.0/16")[0]
	var allocated []netip.Prefix
	seen := map[string]bool{}

	for i := 0; i < 256; i++ {
		sorted, err := validateAllocated(parent, allocated)
		if err != nil {
			t.Fatalf("iteration %d: validateAllocated: %v", i, err)
		}
		got, err := firstFit(parent, sorted, 24)
		if err != nil {
			t.Fatalf("iteration %d: firstFit: %v", i, err)
		}
		assertFits(t, parent, allocated, got)
		if seen[got.String()] {
			t.Fatalf("iteration %d: %s allocated twice", i, got)
		}
		seen[got.String()] = true
		allocated = append(allocated, got)
	}

	if _, err := firstFit(parent, allocated, 24); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("257th allocation: error = %v, want ErrNoSpace", err)
	}
}
