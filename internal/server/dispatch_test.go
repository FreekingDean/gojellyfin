package server

import (
	"reflect"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

var (
	operations    = reflect.TypeFor[api.StrictServerInterface]()
	unimplemented = reflect.TypeFor[api.Unimplemented]()
)

func TestServerDispatchesEveryOperationToOneService(t *testing.T) {
	server := reflect.TypeFor[Server]()
	owners := map[string]string{}
	wrapped := false

	for index := range server.NumField() {
		field := server.Field(index)
		if !field.Anonymous {
			continue
		}

		if field.Type == unimplemented {
			t.Errorf("Server embeds api.Unimplemented directly, which makes every stub as shallow as the services; it goes inside nestedUnimplemented")
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			wrapped = wrapped || embeds(field.Type, unimplemented)
			continue
		}

		service := field.Type.Elem()
		if embeds(service, unimplemented) {
			t.Errorf("%s embeds api.Unimplemented, so its stubs tie with every other service's methods", service)
		}

		for index := range field.Type.NumMethod() {
			method := field.Type.Method(index)
			if _, ok := operations.MethodByName(method.Name); !ok {
				continue
			}
			if owner, taken := owners[method.Name]; taken {
				t.Errorf("%s is declared by both %s and %s, so neither serves it", method.Name, owner, service)
				continue
			}
			owners[method.Name] = service.String()
		}
	}

	if !wrapped {
		t.Errorf("Server does not embed api.Unimplemented one level down, so unimplemented operations have nothing to fall through to")
	}
	if len(owners) == 0 {
		t.Fatal("no service declared an operation, so this guard checked nothing")
	}
}

func embeds(container, target reflect.Type) bool {
	for index := range container.NumField() {
		field := container.Field(index)
		if field.Anonymous && field.Type == target {
			return true
		}
	}

	return false
}
