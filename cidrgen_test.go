package cidrgen

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("valid parent, nil classifications", func(t *testing.T) {
		g, err := New("10.0.0.0/16", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g == nil {
			t.Fatal("New returned nil Generator")
		}
	})

	t.Run("parent host bits are canonicalized", func(t *testing.T) {
		g, err := New("10.0.0.9/16", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := g.Generate(Request{Netmask: 24})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if got.String() != "10.0.0.0/24" {
			t.Fatalf("Generate = %s, want 10.0.0.0/24 (parent not canonicalized)", got)
		}
	})

	t.Run("classification map is copied", func(t *testing.T) {
		classes := map[string]int{"datanode": 28}
		g, err := New("10.0.0.0/24", classes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		classes["datanode"] = 30 // mutate after New

		got, err := g.Generate(Request{Classification: "datanode"})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if got.String() != "10.0.0.0/28" {
			t.Fatalf("Generate = %s, want 10.0.0.0/28 (map not copied)", got)
		}
	})

	tests := []struct {
		name   string
		parent string
	}{
		{name: "unparseable parent", parent: "nope"},
		{name: "missing prefix length", parent: "10.0.0.0"},
		{name: "prefix too long", parent: "10.0.0.0/33"},
		{name: "ipv6 parent", parent: "2001:db8::/32"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.parent, nil)
			if !errors.Is(err, ErrInvalidPrefix) {
				t.Fatalf("error = %v, want ErrInvalidPrefix", err)
			}
			if g != nil {
				t.Fatalf("New returned non-nil Generator on error: %v", g)
			}
		})
	}
}

func TestResolveBits(t *testing.T) {
	classes := map[string]int{"datanode": 28, "computenode": 27}

	tests := []struct {
		name    string
		parent  string
		classes map[string]int
		req     Request
		want    int
		wantErr error
	}{
		{
			name:   "netmask only",
			parent: "10.0.0.0/16", req: Request{Netmask: 26}, want: 26,
		},
		{
			name:   "classification only",
			parent: "10.0.0.0/16", classes: classes,
			req: Request{Classification: "datanode"}, want: 28,
		},
		{
			name:   "netmask overrides classification",
			parent: "10.0.0.0/16", classes: classes,
			req: Request{Netmask: 26, Classification: "datanode"}, want: 26,
		},
		{
			name:   "neither specified",
			parent: "10.0.0.0/16", req: Request{}, wantErr: ErrNoSizeSpecified,
		},
		{
			name:   "unknown classification",
			parent: "10.0.0.0/16", classes: classes,
			req: Request{Classification: "edgenode"}, wantErr: ErrUnknownClassification,
		},
		{
			name:   "classification without a map",
			parent: "10.0.0.0/16",
			req:    Request{Classification: "datanode"}, wantErr: ErrUnknownClassification,
		},
		{
			name:   "requested prefix equals parent",
			parent: "10.0.0.0/16", req: Request{Netmask: 16}, wantErr: ErrInvalidPrefix,
		},
		{
			name:   "requested prefix shorter than parent",
			parent: "10.0.0.0/16", req: Request{Netmask: 8}, wantErr: ErrInvalidPrefix,
		},
		{
			name:   "requested prefix above 32",
			parent: "10.0.0.0/16", req: Request{Netmask: 33}, wantErr: ErrInvalidPrefix,
		},
		{
			name:   "negative netmask",
			parent: "10.0.0.0/16", req: Request{Netmask: -1}, wantErr: ErrInvalidPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.parent, tt.classes)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := g.resolveBits(tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveBits = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	classes := map[string]int{"datanode": 28, "computenode": 27}

	tests := []struct {
		name    string
		parent  string
		classes map[string]int
		req     Request
		want    string
		wantErr error
	}{
		{
			name:   "fills the gap between two allocations",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"},
				Netmask:   24,
			},
			want: "10.0.1.0/24",
		},
		{
			name:   "empty pool returns the lowest block",
			parent: "10.0.0.0/16",
			req:    Request{Netmask: 24},
			want:   "10.0.0.0/24",
		},
		{
			name:   "re-aligns past a smaller allocation",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.0.0/26"},
				Netmask:   24,
			},
			want: "10.0.1.0/24",
		},
		{
			name:   "smaller request slots into a sub-block gap",
			parent: "10.0.0.0/24",
			req: Request{
				Allocated: []string{"10.0.0.0/26", "10.0.0.128/26"},
				Netmask:   26,
			},
			want: "10.0.0.64/26",
		},
		{
			name:   "exact fit in the last slot",
			parent: "10.0.0.0/24",
			req: Request{
				Allocated: []string{"10.0.0.0/25"},
				Netmask:   25,
			},
			want: "10.0.0.128/25",
		},
		{
			name:   "unsorted allocated input is handled",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.3.0/24", "10.0.0.0/24", "10.0.1.0/24"},
				Netmask:   24,
			},
			want: "10.0.2.0/24",
		},
		{
			name:   "request larger than an existing allocation",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.0.0/24"},
				Netmask:   23,
			},
			want: "10.0.2.0/23",
		},
		{
			name:   "fits between two /27 allocations",
			parent: "10.0.0.0/24",
			req: Request{
				Allocated: []string{"10.0.0.0/27", "10.0.0.64/27"},
				Netmask:   27,
			},
			want: "10.0.0.32/27",
		},
		{
			name:   "consecutive single-host allocations",
			parent: "10.0.0.0/30",
			req: Request{
				Allocated: []string{"10.0.0.0/32", "10.0.0.1/32"},
				Netmask:   32,
			},
			want: "10.0.0.2/32",
		},
		{
			name:   "top of the address space",
			parent: "255.255.255.0/24",
			req: Request{
				Allocated: []string{"255.255.255.0/25"},
				Netmask:   25,
			},
			want: "255.255.255.128/25",
		},
		{
			name:    "classification block re-aligns past a smaller allocation",
			parent:  "10.0.0.0/24",
			classes: classes,
			req: Request{
				Allocated:      []string{"10.0.0.0/28"},
				Classification: "computenode",
			},
			want: "10.0.0.32/27",
		},
		{
			name:    "classification-sized block",
			parent:  "10.0.0.0/24",
			classes: classes,
			req: Request{
				Allocated:      []string{"10.0.0.0/28"},
				Classification: "datanode",
			},
			want: "10.0.0.16/28",
		},
		{
			name:   "host bits in inputs are tolerated",
			parent: "10.0.0.9/16",
			req: Request{
				Allocated: []string{"10.0.0.5/24"},
				Netmask:   24,
			},
			want: "10.0.1.0/24",
		},
		{
			name:   "unparseable allocated entry",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.0.0/24", "bad/24"},
				Netmask:   24,
			},
			wantErr: ErrInvalidPrefix,
		},
		{
			name:   "overlapping allocated entries",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"10.0.0.0/23", "10.0.1.0/24"},
				Netmask:   24,
			},
			wantErr: ErrOverlappingInput,
		},
		{
			name:   "allocated entry outside parent",
			parent: "10.0.0.0/16",
			req: Request{
				Allocated: []string{"172.16.0.0/24"},
				Netmask:   24,
			},
			wantErr: ErrOutOfParent,
		},
		{
			name:   "pool exhausted",
			parent: "10.0.0.0/24",
			req: Request{
				Allocated: []string{"10.0.0.0/25", "10.0.0.128/25"},
				Netmask:   25,
			},
			wantErr: ErrNoSpace,
		},
		{
			name:    "no size specified",
			parent:  "10.0.0.0/16",
			req:     Request{},
			wantErr: ErrNoSizeSpecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.parent, tt.classes)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := g.Generate(tt.req)
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
				t.Fatalf("Generate = %s, want %s", got, tt.want)
			}
			assertFits(t, g.parent, mustPrefixes(t, tt.req.Allocated...), got)
		})
	}
}

// TestGenerateSequential carves a /16 into 256 /24s one at a time through a
// single Generator, feeding each result back into Allocated, and checks the
// invariant holds every iteration.
func TestGenerateSequential(t *testing.T) {
	g, err := New("10.0.0.0/16", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var allocated []string
	seen := map[string]bool{}
	for i := 0; i < 256; i++ {
		got, err := g.Generate(Request{Allocated: allocated, Netmask: 24})
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		assertFits(t, g.parent, mustPrefixes(t, allocated...), got)
		if seen[got.String()] {
			t.Fatalf("iteration %d: %s allocated twice", i, got)
		}
		seen[got.String()] = true
		allocated = append(allocated, got.String())
	}

	if _, err := g.Generate(Request{Allocated: allocated, Netmask: 24}); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("257th allocation: error = %v, want ErrNoSpace", err)
	}
}

// TestGeneratorConcurrent runs many Generate calls against one Generator in
// parallel, each with its own Request. Results are checked on the test goroutine
// after the workers finish (t.Fatalf must not be called from a spawned
// goroutine). Meaningful under -race.
func TestGeneratorConcurrent(t *testing.T) {
	g, err := New("10.0.0.0/16", map[string]int{"datanode": 28})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	reqs := []Request{
		{Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"}, Netmask: 24},
		{Allocated: []string{"10.0.0.0/28"}, Classification: "datanode"},
		{Allocated: nil, Netmask: 20},
		{Allocated: []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"}, Netmask: 24},
	}
	wants := []string{"10.0.1.0/24", "10.0.0.16/28", "10.0.0.0/20", "10.0.3.0/24"}

	const n = 100
	type result struct {
		idx int
		got netip.Prefix
		err error
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			idx := i % len(reqs)
			got, err := g.Generate(reqs[idx])
			results[i] = result{idx: idx, got: got, err: err}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err != nil {
			t.Fatalf("call %d (req %d): %v", i, r.idx, r.err)
		}
		if r.got.String() != wants[r.idx] {
			t.Fatalf("call %d (req %d): Generate = %s, want %s", i, r.idx, r.got, wants[r.idx])
		}
		assertFits(t, g.parent, mustPrefixes(t, reqs[r.idx].Allocated...), r.got)
	}
}
