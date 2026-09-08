package cidrgen

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// LoadClassifications parses a YAML document mapping classification names to
// prefix lengths and returns it as a map suitable for the classifications
// argument of [New]:
//
//	classifications:
//	  datanode: 28
//	  computenode: 27
//
// Unknown top-level keys, a missing "classifications" key, a non-integer value,
// or a prefix length outside 0..32 are errors.
func LoadClassifications(r io.Reader) (map[string]int, error) {
	var doc struct {
		Classifications map[string]int `yaml:"classifications"`
	}

	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("cidrgen: parsing classifications: empty document")
		}
		return nil, fmt.Errorf("cidrgen: parsing classifications: %w", err)
	}
	if doc.Classifications == nil {
		return nil, fmt.Errorf("cidrgen: parsing classifications: missing \"classifications\" key")
	}
	for name, bits := range doc.Classifications {
		if bits < 0 || bits > 32 {
			return nil, fmt.Errorf("cidrgen: classification %q: prefix length %d out of range 0..32", name, bits)
		}
	}
	return doc.Classifications, nil
}
