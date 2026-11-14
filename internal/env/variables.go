package env

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type Variable struct {
	Name   string
	Value  string
	Set    bool
	Secret bool
}

func (c Config) Variables() []Variable {
	variables := described(reflect.ValueOf(c))

	names := make([]string, 0, len(c.SourceAPIKeys))
	for name := range c.SourceAPIKeys {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		variables = append(variables, Variable{Name: name, Set: true, Secret: true})
	}

	return variables
}

func described(value reflect.Value) []Variable {
	variables := []Variable{}
	for i := range value.NumField() {
		field := value.Type().Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")

		switch {
		case name == "-":
		case name == "" && field.Type.Kind() == reflect.Struct:
			variables = append(variables, described(value.Field(i))...)
		case field.Tag.Get("secret") == "true":
			variables = append(variables, Variable{Name: name, Set: set(value.Field(i)), Secret: true})
		default:
			variables = append(variables, Variable{Name: name, Value: written(value.Field(i)), Set: set(value.Field(i))})
		}
	}

	return variables
}

func set(value reflect.Value) bool {
	if value.Kind() == reflect.Slice {
		return value.Len() > 0
	}

	return !value.IsZero()
}

func written(value reflect.Value) string {
	if !set(value) {
		return ""
	}
	if entries, ok := value.Interface().([]string); ok {
		return strings.Join(entries, ", ")
	}

	return fmt.Sprint(value.Interface())
}
