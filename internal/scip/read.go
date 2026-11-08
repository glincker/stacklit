// Package scip reads the parts of a SCIP index that stacklit needs (documents,
// occurrences, symbol names) with a small hand-written protobuf decoder, so no
// protobuf dependency is added. Unknown fields are skipped.
package scip

import (
	"errors"
	"fmt"
	"os"
)

// Role bits on an occurrence, from the SCIP schema.
const (
	RoleDefinition = 0x1
)

// Occurrence is one symbol mention. Lines are zero-based like the format.
type Occurrence struct {
	Line    int
	Symbol  string
	Roles   int32
	IsLocal bool
}

// Document is one indexed source file.
type Document struct {
	Path        string
	Occurrences []Occurrence
}

// Index is the decoded subset of a SCIP index.
type Index struct {
	Documents []Document
}

var errTruncated = errors.New("scip: truncated or malformed protobuf")

type buf struct {
	b []byte
	i int
}

func (r *buf) done() bool { return r.i >= len(r.b) }

func (r *buf) varint() (uint64, error) {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if r.i >= len(r.b) {
			return 0, errTruncated
		}
		c := r.b[r.i]
		r.i++
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v, nil
		}
	}
	return 0, errTruncated
}

func (r *buf) bytes() ([]byte, error) {
	n, err := r.varint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(r.b)-r.i) {
		return nil, errTruncated
	}
	out := r.b[r.i : r.i+int(n)]
	r.i += int(n)
	return out, nil
}

// field reads one tag and returns the field number and wire type.
func (r *buf) field() (num int, wire int, err error) {
	t, err := r.varint()
	if err != nil {
		return 0, 0, err
	}
	return int(t >> 3), int(t & 7), nil
}

// skip discards a value of the given wire type.
func (r *buf) skip(wire int) error {
	switch wire {
	case 0:
		_, err := r.varint()
		return err
	case 1:
		r.i += 8
	case 2:
		_, err := r.bytes()
		return err
	case 5:
		r.i += 4
	default:
		return fmt.Errorf("scip: unsupported wire type %d", wire)
	}
	if r.i > len(r.b) {
		return errTruncated
	}
	return nil
}

// Parse decodes a SCIP index from its serialized bytes.
func Parse(data []byte) (*Index, error) {
	r := &buf{b: data}
	idx := &Index{}
	for !r.done() {
		num, wire, err := r.field()
		if err != nil {
			return nil, err
		}
		if num == 2 && wire == 2 {
			b, err := r.bytes()
			if err != nil {
				return nil, err
			}
			doc, err := parseDocument(b)
			if err != nil {
				return nil, err
			}
			idx.Documents = append(idx.Documents, doc)
			continue
		}
		if err := r.skip(wire); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func parseDocument(b []byte) (Document, error) {
	r := &buf{b: b}
	var d Document
	for !r.done() {
		num, wire, err := r.field()
		if err != nil {
			return d, err
		}
		switch {
		case num == 1 && wire == 2:
			p, err := r.bytes()
			if err != nil {
				return d, err
			}
			d.Path = string(p)
		case num == 2 && wire == 2:
			ob, err := r.bytes()
			if err != nil {
				return d, err
			}
			o, err := parseOccurrence(ob)
			if err != nil {
				return d, err
			}
			d.Occurrences = append(d.Occurrences, o)
		default:
			if err := r.skip(wire); err != nil {
				return d, err
			}
		}
	}
	return d, nil
}

func parseOccurrence(b []byte) (Occurrence, error) {
	r := &buf{b: b}
	o := Occurrence{Line: -1}
	for !r.done() {
		num, wire, err := r.field()
		if err != nil {
			return o, err
		}
		switch {
		case num == 1 && wire == 2: // packed range: only the start line matters
			p, err := r.bytes()
			if err != nil {
				return o, err
			}
			if o.Line < 0 && len(p) > 0 {
				pr := &buf{b: p}
				v, err := pr.varint()
				if err != nil {
					return o, err
				}
				o.Line = int(v)
			}
		case num == 1 && wire == 0: // unpacked range element
			v, err := r.varint()
			if err != nil {
				return o, err
			}
			if o.Line < 0 {
				o.Line = int(v)
			}
		case num == 2 && wire == 2:
			s, err := r.bytes()
			if err != nil {
				return o, err
			}
			o.Symbol = string(s)
		case num == 3 && wire == 0:
			v, err := r.varint()
			if err != nil {
				return o, err
			}
			o.Roles = int32(v)
		default:
			if err := r.skip(wire); err != nil {
				return o, err
			}
		}
	}
	o.IsLocal = len(o.Symbol) > 6 && o.Symbol[:6] == "local "
	return o, nil
}

// maxBytes bounds the index file size; override with STACKLIT_SCIP_MAX_BYTES.
func maxBytes() int64 {
	var n int64
	if _, err := fmt.Sscan(os.Getenv("STACKLIT_SCIP_MAX_BYTES"), &n); err == nil && n > 0 {
		return n
	}
	return 512 << 20
}

// Load reads and decodes a .scip file.
func Load(path string) (*Index, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > maxBytes() {
		return nil, fmt.Errorf("scip: %s is %d bytes, over STACKLIT_SCIP_MAX_BYTES", path, st.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}
