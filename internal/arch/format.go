package arch

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
)

// Format returns the model as the fmt command writes it: JSON indented by
// two spaces, with every set of names and patterns sorted and without
// duplicates. Every list of objects, such as the layers, the forbidden
// imports or a fitness rule kind, keeps its order, so a rule keeps its
// index in names such as "isolations[2]", and so do the lists whose order
// means something. The result decodes to a model that hashes as m does, and
// formatting it again changes nothing. m is not modified.
//
// Requires: m was decoded by DecodeModel, so it is valid.
func (m *Model) Format() []byte {
	src, _ := json.Marshal(m) // a Model always encodes
	var c Model
	_ = json.Unmarshal(src, &c) // and decodes what it encoded
	sortStringSets(reflect.ValueOf(&c).Elem())
	var buf bytes.Buffer
	_ = WriteJSON(&buf, &c)
	return buf.Bytes()
}

// sortStringSets sorts and compacts every []string reachable from v, except
// a list whose JSON key [ordered] names.
func sortStringSets(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			sortStringSets(v.Elem())
		}
	case reflect.Struct:
		sortFields(v)
	case reflect.Slice:
		for i := range v.Len() {
			sortStringSets(v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			// A map's values are not addressable: sort a copy and store it.
			cp := reflect.New(v.Type().Elem()).Elem()
			cp.Set(v.MapIndex(k))
			sortStringSets(cp)
			v.SetMapIndex(k, cp)
		}
	default:
	}
}

// sortFields sorts a struct's []string fields, but those whose JSON key
// [ordered] names, and the sets within its other fields.
func sortFields(v reflect.Value) {
	for i := range v.NumField() {
		f, fv := v.Type().Field(i), v.Field(i)
		if !f.IsExported() {
			continue
		}
		if fv.Kind() != reflect.Slice || fv.Type().Elem().Kind() != reflect.String {
			sortStringSets(fv)
			continue
		}
		if key, _, _ := strings.Cut(f.Tag.Get("json"), ","); !ordered(key) && fv.Len() > 1 {
			fv.Set(reflect.ValueOf(compactSorted(fv)).Convert(fv.Type()))
		}
	}
}

// compactSorted returns a []string slice's elements sorted, without
// duplicates.
func compactSorted(v reflect.Value) []string {
	out := make([]string, v.Len())
	for i := range out {
		out[i] = v.Index(i).String()
	}
	slices.Sort(out)
	return slices.Compact(out)
}
