package apitest

import (
	"reflect"
	"slices"
	"testing"
)

const depth = 3

func Reads[R, M any](t *testing.T, merge func(*M, *R), ignored map[string]string) {
	t.Helper()

	request := reflect.TypeFor[R]()
	var untouched M
	for _, field := range reflect.VisibleFields(request) {
		var merged M
		only := reflect.New(request).Elem()
		only.FieldByIndex(field.Index).Set(filled(field.Type, depth))
		merge(&merged, only.Addr().Interface().(*R))

		read := !reflect.DeepEqual(merged, untouched)
		check(t, request.Name()+"."+field.Name, read, ignored[field.Name], "read")
	}
	stale(t, request, ignored)
}

func Writes[M, R any](t *testing.T, dto func(*M) R, ignored map[string]string) {
	t.Helper()

	model := filled(reflect.TypeFor[M](), depth).Interface().(M)
	written := reflect.ValueOf(dto(&model))
	for _, field := range reflect.VisibleFields(written.Type()) {
		check(t, written.Type().Name()+"."+field.Name, !written.FieldByIndex(field.Index).IsZero(), ignored[field.Name], "written")
	}
	stale(t, written.Type(), ignored)
}

func check(t *testing.T, name string, done bool, reason, verb string) {
	t.Helper()

	switch {
	case !done && reason == "":
		t.Errorf("%s is never %s: translate it, or list it as ignored with the reason", name, verb)
	case done && reason != "":
		t.Errorf("%s is listed as ignored (%s) but is %s", name, reason, verb)
	}
}

func stale(t *testing.T, typ reflect.Type, ignored map[string]string) {
	t.Helper()

	names := []string{}
	for _, field := range reflect.VisibleFields(typ) {
		names = append(names, field.Name)
	}
	for name := range ignored {
		if !slices.Contains(names, name) {
			t.Errorf("%s.%s is listed as ignored but no longer exists", typ.Name(), name)
		}
	}
}

func filled(typ reflect.Type, remaining int) reflect.Value {
	value := reflect.New(typ).Elem()
	if remaining == 0 {
		return value
	}

	switch typ.Kind() {
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(7)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(7.5)
	case reflect.String:
		value.SetString("filled")
	case reflect.Pointer:
		value.Set(filled(typ.Elem(), remaining-1).Addr())
	case reflect.Slice:
		value.Set(reflect.Append(value, filled(typ.Elem(), remaining-1)))
	case reflect.Array:
		value.Index(0).Set(filled(typ.Elem(), remaining-1))
	case reflect.Struct:
		for _, field := range reflect.VisibleFields(typ) {
			if field.IsExported() && len(field.Index) == 1 {
				value.FieldByIndex(field.Index).Set(filled(field.Type, remaining-1))
			}
		}
	}

	return value
}
