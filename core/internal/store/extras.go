package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Extras carries JSON keys that appear in a persisted document but are not
// claimed by the owning Go struct's typed fields. The backend never parses,
// validates, or generates these — it round-trips them verbatim so other
// clients (notably the renderer for UI-only state like `avatar`) can write
// their own keys into shared schemas without the backend silently dropping
// them on the next save.
//
// Usage rules:
//   - Declare a `Extras Extras `json:"-"`` field on any struct that
//     persists via JSON and is shared with another writer.
//   - Implement MarshalJSON / UnmarshalJSON on that struct using
//     MarshalWithExtras / UnmarshalWithExtras.
//   - Never read from Extras inside core logic. Treat the keys as opaque.
//
// See also: project CLAUDE.md "Schema 设计准则" for the broader principle.
type Extras map[string]json.RawMessage

// UnmarshalWithExtras decodes raw into v (using v's normal JSON tags),
// then captures any top-level keys not consumed by typed fields into
// `*extras`. Existing entries in `*extras` are replaced. Pass
// `extras = nil` to skip extras capture.
//
// Returns an error if raw is not a JSON object — the Extras pattern only
// makes sense for object-shaped documents.
func UnmarshalWithExtras(raw []byte, v any, extras *Extras) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return err
	}
	if extras == nil {
		return nil
	}

	var all map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := json.Unmarshal(raw, &all); err != nil {
		return fmt.Errorf("extras: raw is not a JSON object: %w", err)
	}

	known, err := jsonTagSet(v)
	if err != nil {
		return err
	}
	for k := range known {
		delete(all, k)
	}
	if len(all) == 0 {
		*extras = nil
		return nil
	}
	*extras = all
	return nil
}

// MarshalWithExtras marshals v using its normal JSON tags, then merges
// `extras` keys that aren't already present in the result. Typed fields
// always win — the backend's contract is the source of truth for any key
// it claims.
//
// The output is sorted by key so persisted files don't churn on every
// save just because Go's map iteration order changed.
func MarshalWithExtras(v any, extras Extras) ([]byte, error) {
	base, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(extras) == 0 {
		return base, nil
	}
	var combined map[string]json.RawMessage
	if err := json.Unmarshal(base, &combined); err != nil {
		return nil, fmt.Errorf("extras: marshal output is not an object: %w", err)
	}
	for k, val := range extras {
		if _, exists := combined[k]; exists {
			continue // typed field wins
		}
		combined[k] = val
	}
	return marshalSorted(combined)
}

// jsonTagSet returns the set of top-level JSON keys claimed by v's typed
// fields. Embedded structs and `json:",inline"` are not supported because
// no schema in this codebase needs them; revisit if that changes.
func jsonTagSet(v any) (map[string]struct{}, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("extras: nil pointer passed to UnmarshalWithExtras")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("extras: v must be a struct or pointer to one, got %s", rv.Kind())
	}
	rt := rv.Type()
	out := make(map[string]struct{}, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" {
			name = f.Name // matches encoding/json default
		}
		out[name] = struct{}{}
	}
	return out, nil
}

// marshalSorted writes a map[string]json.RawMessage with keys in
// ascending order so output is byte-stable across runs.
func marshalSorted(m map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(m[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
