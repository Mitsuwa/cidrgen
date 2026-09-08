package cidrgen

import (
	"errors"
	"testing"
)

func TestParsePrefix(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // canonical form; empty means expect error
		wantErr error
	}{
		{name: "canonical", in: "10.0.0.0/24", want: "10.0.0.0/24"},
		{name: "host bits set are masked", in: "10.0.0.5/24", want: "10.0.0.0/24"},
		{name: "full mask", in: "192.168.1.7/32", want: "192.168.1.7/32"},
		{name: "zero mask", in: "8.8.8.8/0", want: "0.0.0.0/0"},
		{name: "not a cidr", in: "10.0.0.0", wantErr: ErrInvalidPrefix},
		{name: "garbage", in: "not-an-ip/24", wantErr: ErrInvalidPrefix},
		{name: "prefix too long", in: "10.0.0.0/33", wantErr: ErrInvalidPrefix},
		{name: "ipv6 rejected", in: "2001:db8::/32", wantErr: ErrInvalidPrefix},
		{name: "ipv4-mapped ipv6 rejected", in: "::ffff:10.0.0.0/120", wantErr: ErrInvalidPrefix},
		{name: "empty", in: "", wantErr: ErrInvalidPrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrefix(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parsePrefix(%q) error = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePrefix(%q) unexpected error: %v", tt.in, err)
			}
			if got.String() != tt.want {
				t.Fatalf("parsePrefix(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestU32RoundTrip(t *testing.T) {
	for _, s := range []string{"0.0.0.0", "10.0.0.1", "192.168.255.254", "255.255.255.255"} {
		p, err := parsePrefix(s + "/32")
		if err != nil {
			t.Fatalf("parsePrefix(%q): %v", s, err)
		}
		if got := u32ToAddr(addrToU32(p.Addr())); got != p.Addr() {
			t.Fatalf("round trip %s = %s", s, got)
		}
	}
}

func TestPrefixRange(t *testing.T) {
	tests := []struct {
		in         string
		start, end uint64
	}{
		{"10.0.0.0/24", 0x0A000000, 0x0A0000FF},
		{"10.0.0.0/32", 0x0A000000, 0x0A000000},
		{"0.0.0.0/0", 0, 0xFFFFFFFF},
		{"255.255.255.128/25", 0xFFFFFF80, 0xFFFFFFFF},
	}
	for _, tt := range tests {
		p, err := parsePrefix(tt.in)
		if err != nil {
			t.Fatalf("parsePrefix(%q): %v", tt.in, err)
		}
		start, end := prefixRange(p)
		if start != tt.start || end != tt.end {
			t.Fatalf("prefixRange(%q) = (%#x, %#x), want (%#x, %#x)", tt.in, start, end, tt.start, tt.end)
		}
	}
}
