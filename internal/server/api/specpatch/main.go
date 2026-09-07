package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type override struct {
	Pointer string `json:"pointer"`
	GoType  string `json:"x-go-type"`
	Why     string `json:"why"`
}

type target struct {
	offset int64
	empty  bool
	found  bool
}

func main() {
	spec := flag.String("spec", "", "vendored upstream spec")
	overrides := flag.String("overrides", "", "x-go-type overrides")
	extend := flag.String("extend", "", "spec whose paths and schemas are merged in")
	out := flag.String("out", "", "patched spec")
	flag.Parse()

	if *spec == "" || *overrides == "" || *out == "" {
		log.Fatal("specpatch: -spec, -overrides and -out are required")
	}

	patched, err := patch(*spec, *overrides, *extend)
	if err != nil {
		log.Fatalf("specpatch: %v", err)
	}

	if err := os.WriteFile(*out, patched, 0o644); err != nil {
		log.Fatalf("specpatch: %v", err)
	}
}

type members struct {
	Paths      map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
}

func readMembers(path string) (*members, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	read := &members{}
	if err := json.Unmarshal(raw, read); err != nil {
		return nil, err
	}

	return read, nil
}

func rendered(existing, adding map[string]json.RawMessage, what string, empty bool) (string, error) {
	names := make([]string, 0, len(adding))
	for name := range adding {
		if _, taken := existing[name]; taken {
			return "", fmt.Errorf("%s %q is already declared upstream; rename it", what, name)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%q: %s", name, adding[name]))
	}

	joined := strings.Join(parts, ", ")
	if empty {
		return joined, nil
	}

	return joined + ", ", nil
}

func patch(specPath, overridesPath, extendPath string) ([]byte, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}

	list, err := readOverrides(overridesPath)
	if err != nil {
		return nil, err
	}

	targets := make(map[string]*target, len(list))
	for _, o := range list {
		targets[o.Pointer] = &target{}
	}

	upstream, err := readMembers(specPath)
	if err != nil {
		return nil, err
	}

	extension := &members{}
	if extendPath != "" {
		if extension, err = readMembers(extendPath); err != nil {
			return nil, err
		}
	}

	if len(extension.Paths) > 0 {
		targets["/paths"] = &target{}
	}
	if len(extension.Components.Schemas) > 0 {
		targets["/components/schemas"] = &target{}
	}

	if err := scan(json.NewDecoder(strings.NewReader(string(raw))), "", targets); err != nil {
		return nil, err
	}

	edits := make([]struct {
		at   int64
		text string
	}, 0, len(list)+2)

	merges := []struct {
		pointer  string
		what     string
		existing map[string]json.RawMessage
		adding   map[string]json.RawMessage
	}{
		{"/paths", "path", upstream.Paths, extension.Paths},
		{"/components/schemas", "schema", upstream.Components.Schemas, extension.Components.Schemas},
	}

	for _, merge := range merges {
		found, wanted := targets[merge.pointer]
		if !wanted {
			continue
		}
		if !found.found {
			return nil, fmt.Errorf("%s does not resolve in %s", merge.pointer, specPath)
		}

		text, err := rendered(merge.existing, merge.adding, merge.what, found.empty)
		if err != nil {
			return nil, err
		}

		edits = append(edits, struct {
			at   int64
			text string
		}{at: found.offset, text: text})
	}

	for _, o := range list {
		found := targets[o.Pointer]
		if !found.found {
			return nil, fmt.Errorf("override %q does not resolve in %s; re-apply it by hand or drop it (%s)", o.Pointer, specPath, o.Why)
		}

		text := fmt.Sprintf("%q: %q, ", "x-go-type", o.GoType)
		if found.empty {
			text = fmt.Sprintf("%q: %q", "x-go-type", o.GoType)
		}

		edits = append(edits, struct {
			at   int64
			text string
		}{at: found.offset, text: text})
	}

	sort.Slice(edits, func(i, j int) bool { return edits[i].at > edits[j].at })

	for _, edit := range edits {
		raw = append(raw[:edit.at], append([]byte(edit.text), raw[edit.at:]...)...)
	}

	return raw, nil
}

func readOverrides(path string) ([]override, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var list []override
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, o := range list {
		switch {
		case o.Pointer == "":
			return nil, fmt.Errorf("override with no pointer")
		case o.GoType == "":
			return nil, fmt.Errorf("override %q has no x-go-type", o.Pointer)
		case o.Why == "":
			return nil, fmt.Errorf("override %q has no why", o.Pointer)
		case seen[o.Pointer]:
			return nil, fmt.Errorf("override %q listed twice", o.Pointer)
		}
		seen[o.Pointer] = true
	}

	return list, nil
}

func scan(dec *json.Decoder, path string, targets map[string]*target) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		if found, wanted := targets[path]; wanted {
			if found.found {
				return fmt.Errorf("pointer %q resolves more than once", path)
			}
			found.found = true
			found.offset = dec.InputOffset()
			found.empty = !dec.More()
		}

		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}

			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key at %q is not a string", path)
			}

			if name == "x-go-type" {
				return fmt.Errorf("%q already carries an x-go-type; the vendored spec should stay pristine", path)
			}

			if err := scan(dec, path+"/"+escape(name), targets); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			if err := scan(dec, path+"/"+strconv.Itoa(i), targets); err != nil {
				return err
			}
		}
	}

	_, err = dec.Token()

	return err
}

func escape(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}
