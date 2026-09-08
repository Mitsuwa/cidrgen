package cidrgen

import (
	"os"
	"strings"
	"testing"
)

func TestLoadClassificationsFile(t *testing.T) {
	f, err := os.Open("testdata/classifications.yaml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	got, err := LoadClassifications(f)
	if err != nil {
		t.Fatalf("LoadClassifications: %v", err)
	}
	want := map[string]int{"datanode": 28, "computenode": 27, "edgenode": 30}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("got[%q] = %d, want %d", k, got[k], v)
		}
	}
}

func TestLoadClassificationsErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "empty document", in: ""},
		{name: "missing classifications key", in: "other:\n  datanode: 28\n"},
		{name: "unknown top-level key", in: "classifications:\n  datanode: 28\nextra: 1\n"},
		{name: "non-integer value", in: "classifications:\n  datanode: small\n"},
		{name: "prefix length too large", in: "classifications:\n  datanode: 40\n"},
		{name: "negative prefix length", in: "classifications:\n  datanode: -1\n"},
		{name: "malformed yaml", in: "classifications: :\n  - ]["},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadClassifications(strings.NewReader(tt.in)); err == nil {
				t.Fatalf("expected error for %q", tt.in)
			}
		})
	}
}

func TestLoadClassificationsRoundTrip(t *testing.T) {
	in := "classifications:\n  datanode: 28\n  computenode: 27\n"
	m, err := LoadClassifications(strings.NewReader(in))
	if err != nil {
		t.Fatalf("LoadClassifications: %v", err)
	}
	g, err := New("10.0.0.0/24", m)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := g.Generate(Request{Classification: "datanode"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.String() != "10.0.0.0/28" {
		t.Fatalf("Generate = %s, want 10.0.0.0/28", got)
	}
}
