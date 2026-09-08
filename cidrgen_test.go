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

// step is one Generate call within a TestGenerateMixedSizeSequence row. netmask
// or classification names the requested size (mirroring Request); want is the
// canonical CIDR the call must return, or wantErr the sentinel it must match.
type step struct {
	netmask        int
	classification string
	want           string
	wantErr        error
}

// TestGenerateMixedSizeSequence drives one Generator through an ordered mix of
// differently sized requests, feeding each result back into Request.Allocated,
// and checks the exact block returned at every step. Each success step also
// asserts the assertFits invariant, that no address is handed out twice across
// the row, and — for classification steps — that the block is the mapped size.
//
// A row's seed entries stay in Request.Allocated for every step. They are
// written with host bits set (e.g. "10.0.0.5/25"), so the rows that use them
// also verify the Generator canonicalizes each Allocated entry with Masked()
// before the overlap scan: the expected sequences are those of the masked
// forms, and a Generator that scanned the raw addresses would return different
// blocks (or ErrNoSpace).
//
// The expected sequences are traced through firstFit (first-fit, lowest
// address, re-aligned up to the block size past each allocation); a wrong value
// here is a table bug, not an algorithm bug.
func TestGenerateMixedSizeSequence(t *testing.T) {
	classes := map[string]int{"region": 16, "zone": 24, "host": 32}

	tests := []struct {
		name    string
		parent  string
		classes map[string]int
		seed    []string // host-bit-set entries kept in Allocated for every step
		steps   []step
	}{
		{
			name:   "netmask mix under a /8 re-aligns a later /16 past an early /32",
			parent: "10.0.0.0/8",
			steps: []step{
				{netmask: 32, want: "10.0.0.0/32"},
				{netmask: 16, want: "10.1.0.0/16"},
				{netmask: 24, want: "10.0.1.0/24"},
				{netmask: 32, want: "10.0.0.1/32"},
				{netmask: 16, want: "10.2.0.0/16"},
			},
		},
		{
			name:    "classification mix under a /8",
			parent:  "100.0.0.0/8",
			classes: classes,
			steps: []step{
				{classification: "host", want: "100.0.0.0/32"},
				{classification: "region", want: "100.1.0.0/16"},
				{classification: "zone", want: "100.0.1.0/24"},
				{classification: "host", want: "100.0.0.1/32"},
				{classification: "region", want: "100.2.0.0/16"},
			},
		},
		{
			name:    "netmask and classification steps interleaved through one Generator",
			parent:  "10.0.0.0/8",
			classes: classes,
			steps: []step{
				{classification: "zone", want: "10.0.0.0/24"},
				{netmask: 32, want: "10.0.1.0/32"},
				{classification: "region", want: "10.1.0.0/16"},
				{netmask: 24, want: "10.0.2.0/24"},
				{classification: "host", want: "10.0.1.1/32"},
			},
		},
		{
			name:   "/24 and /32 packing under a tight /16 parent",
			parent: "10.0.0.0/16",
			steps: []step{
				{netmask: 24, want: "10.0.0.0/24"},
				{netmask: 32, want: "10.0.1.0/32"},
				{netmask: 24, want: "10.0.2.0/24"},
				{netmask: 32, want: "10.0.1.1/32"},
				{netmask: 24, want: "10.0.3.0/24"},
			},
		},
		{
			name:   "sequence fills a /24 exactly then a /32 no longer fits",
			parent: "10.0.0.0/24",
			steps: []step{
				{netmask: 25, want: "10.0.0.0/25"},
				{netmask: 26, want: "10.0.0.128/26"},
				{netmask: 26, want: "10.0.0.192/26"},
				{netmask: 32, wantErr: ErrNoSpace},
			},
		},
		{
			name:   "whole address space: /1, /32, /2",
			parent: "0.0.0.0/0",
			steps: []step{
				{netmask: 1, want: "0.0.0.0/1"},
				{netmask: 32, want: "128.0.0.0/32"},
				{netmask: 2, want: "192.0.0.0/2"},
			},
		},
		{
			name:   "whole address space: /4 and /10 interleaved",
			parent: "0.0.0.0/0",
			steps: []step{
				{netmask: 4, want: "0.0.0.0/4"},
				{netmask: 10, want: "16.0.0.0/10"},
				{netmask: 4, want: "32.0.0.0/4"},
				{netmask: 10, want: "16.64.0.0/10"},
			},
		},
		{
			name:   "whole address space: /10, /4, /2, /32",
			parent: "0.0.0.0/0",
			steps: []step{
				{netmask: 10, want: "0.0.0.0/10"},
				{netmask: 4, want: "16.0.0.0/4"},
				{netmask: 2, want: "64.0.0.0/2"},
				{netmask: 32, want: "0.64.0.0/32"},
			},
		},
		{
			// The seed masks to 10.0.0.0/25 = [0,127]. A Generator that scanned
			// the raw 10.0.0.5/25 would treat it as [5,132] and answer
			// 10.0.0.192/26 on the first step.
			name:   "host-bit seed masked to the block it occupies, then exhaustion",
			parent: "10.0.0.0/24",
			seed:   []string{"10.0.0.5/25"},
			steps: []step{
				{netmask: 26, want: "10.0.0.128/26"},
				{netmask: 26, want: "10.0.0.192/26"},
				{netmask: 25, wantErr: ErrNoSpace},
			},
		},
		{
			// Host bits set in the parent and in both seeds. Masked forms:
			// parent 10.0.0.0/8, seeds 10.0.0.0/24 and 10.1.0.0/16.
			name:   "host bits in the parent and the seeds under a /8",
			parent: "10.1.2.3/8",
			seed:   []string{"10.0.0.200/24", "10.1.50.99/16"},
			steps: []step{
				{netmask: 24, want: "10.0.1.0/24"},
				{netmask: 16, want: "10.2.0.0/16"},
				{netmask: 32, want: "10.0.2.0/32"},
			},
		},
		{
			// Host-bit seed (masks to 100.0.0.0/24) with classification steps.
			name:    "host-bit seed with classification steps",
			parent:  "100.0.0.0/8",
			classes: classes,
			seed:    []string{"100.0.0.77/24"},
			steps: []step{
				{classification: "zone", want: "100.0.1.0/24"},
				{classification: "host", want: "100.0.2.0/32"},
				{classification: "region", want: "100.1.0.0/16"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.parent, tt.classes)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			allocated := append([]string(nil), tt.seed...)
			seen := map[string]bool{}
			for i, s := range tt.steps {
				got, err := g.Generate(Request{
					Allocated:      allocated,
					Netmask:        s.netmask,
					Classification: s.classification,
				})
				if s.wantErr != nil {
					if !errors.Is(err, s.wantErr) {
						t.Fatalf("step %d: error = %v, want %v", i, err, s.wantErr)
					}
					break // an error step is always last in a row
				}
				if err != nil {
					t.Fatalf("step %d: unexpected error: %v", i, err)
				}
				if got.String() != s.want {
					t.Fatalf("step %d: Generate = %s, want %s", i, got, s.want)
				}
				assertFits(t, g.parent, mustPrefixes(t, allocated...), got)
				if s.classification != "" && got.Bits() != tt.classes[s.classification] {
					t.Fatalf("step %d: got /%d, want /%d for %q",
						i, got.Bits(), tt.classes[s.classification], s.classification)
				}
				if seen[got.String()] {
					t.Fatalf("step %d: %s allocated twice", i, got)
				}
				seen[got.String()] = true
				allocated = append(allocated, got.String())
			}
		})
	}
}
