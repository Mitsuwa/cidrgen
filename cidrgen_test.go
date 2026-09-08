package cidrgen

import (
	"errors"
	"testing"
)

func TestResolveBits(t *testing.T) {
	classes := map[string]int{"datanode": 28, "computenode": 27}

	tests := []struct {
		name       string
		req        Request
		parentBits int
		want       int
		wantErr    error
	}{
		{
			name: "netmask only",
			req:  Request{Netmask: 26}, parentBits: 16, want: 26,
		},
		{
			name: "classification only",
			req:  Request{Classification: "datanode", Classifications: classes}, parentBits: 16, want: 28,
		},
		{
			name: "netmask overrides classification",
			req:  Request{Netmask: 26, Classification: "datanode", Classifications: classes}, parentBits: 16, want: 26,
		},
		{
			name: "neither specified",
			req:  Request{}, parentBits: 16, wantErr: ErrNoSizeSpecified,
		},
		{
			name: "unknown classification",
			req:  Request{Classification: "edgenode", Classifications: classes}, parentBits: 16, wantErr: ErrUnknownClassification,
		},
		{
			name: "classification without a map",
			req:  Request{Classification: "datanode"}, parentBits: 16, wantErr: ErrUnknownClassification,
		},
		{
			name: "requested prefix equals parent",
			req:  Request{Netmask: 16}, parentBits: 16, wantErr: ErrInvalidPrefix,
		},
		{
			name: "requested prefix shorter than parent",
			req:  Request{Netmask: 8}, parentBits: 16, wantErr: ErrInvalidPrefix,
		},
		{
			name: "requested prefix above 32",
			req:  Request{Netmask: 33}, parentBits: 16, wantErr: ErrInvalidPrefix,
		},
		{
			name: "negative netmask",
			req:  Request{Netmask: -1}, parentBits: 16, wantErr: ErrInvalidPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBits(tt.req, tt.parentBits)
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
	classes := map[string]int{"datanode": 28}

	tests := []struct {
		name    string
		req     Request
		want    string
		wantErr error
	}{
		{
			name: "fills the gap between two allocations",
			req: Request{
				Parent:    "10.0.0.0/16",
				Allocated: []string{"10.0.0.0/24", "10.0.2.0/24"},
				Netmask:   24,
			},
			want: "10.0.1.0/24",
		},
		{
			name: "classification-sized block",
			req: Request{
				Parent:          "10.0.0.0/24",
				Allocated:       []string{"10.0.0.0/28"},
				Classification:  "datanode",
				Classifications: classes,
			},
			want: "10.0.0.16/28",
		},
		{
			name: "host bits in inputs are tolerated",
			req: Request{
				Parent:    "10.0.0.9/16",
				Allocated: []string{"10.0.0.5/24"},
				Netmask:   24,
			},
			want: "10.0.1.0/24",
		},
		{
			name:    "unparseable parent",
			req:     Request{Parent: "nope", Netmask: 24},
			wantErr: ErrInvalidPrefix,
		},
		{
			name: "unparseable allocated entry",
			req: Request{
				Parent:    "10.0.0.0/16",
				Allocated: []string{"10.0.0.0/24", "bad/24"},
				Netmask:   24,
			},
			wantErr: ErrInvalidPrefix,
		},
		{
			name: "overlapping allocated entries",
			req: Request{
				Parent:    "10.0.0.0/16",
				Allocated: []string{"10.0.0.0/23", "10.0.1.0/24"},
				Netmask:   24,
			},
			wantErr: ErrOverlappingInput,
		},
		{
			name: "allocated entry outside parent",
			req: Request{
				Parent:    "10.0.0.0/16",
				Allocated: []string{"172.16.0.0/24"},
				Netmask:   24,
			},
			wantErr: ErrOutOfParent,
		},
		{
			name: "pool exhausted",
			req: Request{
				Parent:    "10.0.0.0/24",
				Allocated: []string{"10.0.0.0/25", "10.0.0.128/25"},
				Netmask:   25,
			},
			wantErr: ErrNoSpace,
		},
		{
			name:    "no size specified",
			req:     Request{Parent: "10.0.0.0/16"},
			wantErr: ErrNoSizeSpecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Generate(tt.req)
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
		})
	}
}
