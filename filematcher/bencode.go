package filematcher

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"unicode/utf8"
)

// decodeBencode parses a bencoded value into Go types:
//   - dict  → map[string]any
//   - list  → []any
//   - int   → int64
//   - str   → string when valid UTF-8, otherwise []byte
func decodeBencode(data []byte) (any, error) {
	v, n, err := parseBencode(data)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, fmt.Errorf("trailing data after bencode value")
	}
	return v, nil
}

func decodeBencodeDict(data []byte) (map[string]any, error) {
	v, err := decodeBencode(data)
	if err != nil {
		return nil, err
	}
	d, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top-level value is not a dict")
	}
	return d, nil
}

func parseBencode(data []byte) (any, int, error) {
	if len(data) == 0 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	switch data[0] {
	case 'i':
		end := bytes.IndexByte(data, 'e')
		if end < 0 {
			return nil, 0, fmt.Errorf("unterminated integer")
		}
		n, err := strconv.ParseInt(string(data[1:end]), 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %w", err)
		}
		return n, end + 1, nil
	case 'l':
		list := []any{}
		pos := 1
		for {
			if pos >= len(data) {
				return nil, 0, fmt.Errorf("unterminated list")
			}
			if data[pos] == 'e' {
				return list, pos + 1, nil
			}
			v, n, err := parseBencode(data[pos:])
			if err != nil {
				return nil, 0, err
			}
			list = append(list, v)
			pos += n
		}
	case 'd':
		dict := make(map[string]any)
		pos := 1
		var lastKey string
		first := true
		for {
			if pos >= len(data) {
				return nil, 0, fmt.Errorf("unterminated dict")
			}
			if data[pos] == 'e' {
				return dict, pos + 1, nil
			}
			keyVal, n, err := parseBencode(data[pos:])
			if err != nil {
				return nil, 0, err
			}
			pos += n
			key, ok := keyVal.(string)
			if !ok {
				if b, ok := keyVal.([]byte); ok {
					key = string(b)
				} else {
					return nil, 0, fmt.Errorf("dict key must be a string")
				}
			}
			if !first && key < lastKey {
				// Tolerant on decode; BEP 3 requires sorted keys on encode.
			}
			first = false
			lastKey = key
			val, n, err := parseBencode(data[pos:])
			if err != nil {
				return nil, 0, err
			}
			pos += n
			dict[key] = val
		}
	default:
		colon := bytes.IndexByte(data, ':')
		if colon < 0 {
			return nil, 0, fmt.Errorf("invalid string length")
		}
		length, err := strconv.Atoi(string(data[:colon]))
		if err != nil || length < 0 {
			return nil, 0, fmt.Errorf("invalid string length")
		}
		start := colon + 1
		end := start + length
		if end > len(data) {
			return nil, 0, io.ErrUnexpectedEOF
		}
		raw := data[start:end]
		if utf8.Valid(raw) {
			return string(raw), end, nil
		}
		out := make([]byte, length)
		copy(out, raw)
		return out, end, nil
	}
}

// encodeBencode encodes v using BEP 3 rules (dict keys sorted as raw strings).
func encodeBencode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeBencode(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeBencode(w *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		return fmt.Errorf("cannot encode nil")
	case string:
		writeString(w, x)
		return nil
	case []byte:
		writeBytes(w, x)
		return nil
	case int:
		fmt.Fprintf(w, "i%de", x)
		return nil
	case int64:
		fmt.Fprintf(w, "i%de", x)
		return nil
	case int32:
		fmt.Fprintf(w, "i%de", x)
		return nil
	case bool:
		if x {
			w.WriteString("i1e")
		} else {
			w.WriteString("i0e")
		}
		return nil
	case []any:
		w.WriteByte('l')
		for _, item := range x {
			if err := writeBencode(w, item); err != nil {
				return err
			}
		}
		w.WriteByte('e')
		return nil
	case []string:
		w.WriteByte('l')
		for _, item := range x {
			writeString(w, item)
		}
		w.WriteByte('e')
		return nil
	case map[string]any:
		w.WriteByte('d')
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeString(w, k)
			if err := writeBencode(w, x[k]); err != nil {
				return err
			}
		}
		w.WriteByte('e')
		return nil
	default:
		return fmt.Errorf("unsupported bencode type %T", v)
	}
}

func writeString(w *bytes.Buffer, s string) {
	fmt.Fprintf(w, "%d:", len(s))
	w.WriteString(s)
}

func writeBytes(w *bytes.Buffer, b []byte) {
	fmt.Fprintf(w, "%d:", len(b))
	w.Write(b)
}

// deepEqualBencode compares two decoded bencode trees.
func deepEqualBencode(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !deepEqualBencode(v, bv[k]) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqualBencode(av[i], bv[i]) {
				return false
			}
		}
		return true
	case string:
		switch bv := b.(type) {
		case string:
			return av == bv
		case []byte:
			return av == string(bv)
		default:
			return false
		}
	case []byte:
		switch bv := b.(type) {
		case []byte:
			return bytes.Equal(av, bv)
		case string:
			return string(av) == bv
		default:
			return false
		}
	case int64:
		switch bv := b.(type) {
		case int64:
			return av == bv
		case int:
			return av == int64(bv)
		default:
			return false
		}
	case int:
		return deepEqualBencode(int64(av), b)
	default:
		return a == b
	}
}

func asString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	default:
		return "", false
	}
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	default:
		return 0, false
	}
}
