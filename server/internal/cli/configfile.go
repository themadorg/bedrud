package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"bedrud/config"

	"gopkg.in/yaml.v3"
)

// setConfigValue writes a single dotted key into the YAML file at path and
// returns the canonical key it wrote.
//
// The file is edited as a YAML node tree rather than re-serialised from a
// decoded map, so every untouched key keeps its original spelling, ordering
// and comments. That matters because config.Load unmarshals into camelCase
// yaml tags and is case sensitive: a whole-file rewrite that lowercases keys
// (as viper's WriteConfigAs does) produces a file bedrud can no longer read.
func setConfigValue(path, key, raw string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse config %s: %w", path, err)
	}
	root, err := documentRoot(&doc)
	if err != nil {
		return "", err
	}

	segments := strings.Split(key, ".")
	for _, seg := range segments {
		if seg == "" {
			return "", fmt.Errorf("invalid config key: %q", key)
		}
	}

	resolved, ok := resolveConfigKey(reflect.TypeOf(config.Config{}), segments)
	if !ok {
		return "", fmt.Errorf("unknown config key: %s", key)
	}

	value, err := scalarNodeFor(raw, resolved.leaf)
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.Join(resolved.names, "."), err)
	}
	if err := assignNode(root, resolved.names, value); err != nil {
		return "", err
	}

	out, err := encodeDocument(&doc)
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	var probe config.Config
	if err := yaml.Unmarshal(out, &probe); err != nil {
		return "", fmt.Errorf("refusing to write a config bedrud could not load back: %w", err)
	}
	if err := writeConfigFile(path, out); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return strings.Join(resolved.names, "."), nil
}

// configKey is a dotted key resolved against the Config struct: names holds the
// canonical spelling of each segment, leaf the Go type the value lands in
// (nil when the key sits inside a free-form map and has no static type).
type configKey struct {
	names []string
	leaf  reflect.Type
}

// resolveConfigKey walks the Config struct by yaml tag, matching each segment
// case insensitively so "auth.jwtsecret" resolves to the canonical
// "auth.jwtSecret" instead of writing a key the loader would ignore.
func resolveConfigKey(t reflect.Type, segments []string) (configKey, bool) {
	resolved := configKey{names: make([]string, 0, len(segments))}
	current := t
	for i, seg := range segments {
		for current != nil && current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		if current == nil {
			return configKey{}, false
		}
		if current.Kind() == reflect.Map {
			// Free-form section (e.g. email.subjectLines): the remaining
			// segments are user-defined keys, kept exactly as typed.
			resolved.names = append(resolved.names, segments[i:]...)
			if len(segments)-i == 1 {
				resolved.leaf = current.Elem()
			}
			return resolved, true
		}
		if current.Kind() != reflect.Struct {
			return configKey{}, false
		}
		field, name, ok := fieldByYAMLName(current, seg)
		if !ok {
			return configKey{}, false
		}
		resolved.names = append(resolved.names, name)
		current = field.Type
	}
	resolved.leaf = current
	return resolved, true
}

// fieldByYAMLName finds a struct field by its yaml tag name, ignoring case,
// and returns the tag's canonical spelling.
func fieldByYAMLName(t reflect.Type, name string) (reflect.StructField, string, bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = strings.ToLower(field.Name)
		}
		if strings.EqualFold(tag, name) {
			return field, tag, true
		}
	}
	return reflect.StructField{}, "", false
}

// scalarNodeFor turns the CLI string into a YAML node typed for the target
// field, so numbers stay numbers, bools stay bools, and a string field that
// happens to hold digits (server.port) stays quoted.
func scalarNodeFor(raw string, t reflect.Type) (*yaml.Node, error) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		// No static type: only bools are unambiguous enough to coerce.
		switch strings.ToLower(raw) {
		case "true", "false":
			return boolNode(strings.ToLower(raw) == "true"), nil
		}
		return stringNode(raw), nil
	}

	switch t.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("expected a boolean, got %q", raw)
		}
		return boolNode(b), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected an integer, got %q", raw)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(n, 10)}, nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("expected a number, got %q", raw)
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(f, 'g', -1, 64)}, nil
	case reflect.String:
		return stringNode(raw), nil
	case reflect.Slice, reflect.Array:
		return sequenceNode(raw, t.Elem())
	default:
		return nil, fmt.Errorf("is a config section, not a value")
	}
}

// sequenceNode builds a flow sequence from a comma-separated CLI value so list
// fields (server.trustedProxies) do not end up holding a single string.
func sequenceNode(raw string, elem reflect.Type) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	if strings.TrimSpace(raw) == "" {
		return node, nil
	}
	for _, part := range strings.Split(raw, ",") {
		item, err := scalarNodeFor(strings.TrimSpace(part), elem)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, item)
	}
	return node, nil
}

func boolNode(b bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}
}

// stringNode quotes values that YAML would otherwise resolve to another type,
// which would make the loader fail on a string field.
func stringNode(s string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: s}
	if needsQuoting(s) {
		node.Tag = "!!str"
		node.Style = yaml.DoubleQuotedStyle
	}
	return node
}

func needsQuoting(s string) bool {
	if s == "" || strings.TrimSpace(s) != s {
		return true
	}
	var probe any
	if err := yaml.Unmarshal([]byte(s), &probe); err != nil {
		return true
	}
	str, ok := probe.(string)
	return !ok || str != s
}

// documentRoot returns the mapping node the keys live in, creating one for an
// empty or comment-only file.
func documentRoot(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == 0 || len(doc.Content) == 0 {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{root}
		return root, nil
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && (root.Tag == "!!null" || root.Value == "") {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
		root.Value = ""
		return root, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root is not a YAML mapping")
	}
	return root, nil
}

func assignNode(root *yaml.Node, names []string, value *yaml.Node) error {
	node := root
	for i, name := range names[:len(names)-1] {
		child, err := childMapping(node, name)
		if err != nil {
			return fmt.Errorf("%s: %w", strings.Join(names[:i+1], "."), err)
		}
		node = child
	}
	setMapEntry(node, names[len(names)-1], value)
	return nil
}

func childMapping(parent *yaml.Node, name string) (*yaml.Node, error) {
	if key, value, ok := findMapEntry(parent, name); ok {
		switch {
		case value.Kind == yaml.MappingNode:
			key.Value = name
			return value, nil
		case value.Kind == yaml.ScalarNode && (value.Tag == "!!null" || value.Value == ""):
			key.Value = name
			value.Kind = yaml.MappingNode
			value.Tag = "!!map"
			value.Value = ""
			return value, nil
		default:
			return nil, fmt.Errorf("holds a value, not a section")
		}
	}
	child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		child,
	)
	return child, nil
}

// setMapEntry replaces the value for name, keeping the comments attached to the
// old value and repairing the key's spelling when the file holds a variant of
// it (a config already mangled by an earlier release, for instance).
func setMapEntry(parent *yaml.Node, name string, value *yaml.Node) {
	if key, old, ok := findMapEntry(parent, name); ok {
		key.Value = name
		value.HeadComment = old.HeadComment
		value.LineComment = old.LineComment
		value.FootComment = old.FootComment
		*old = *value
		return
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		value,
	)
}

// findMapEntry looks up a key, preferring an exact match and falling back to a
// case-insensitive one.
func findMapEntry(parent *yaml.Node, name string) (key, value *yaml.Node, found bool) {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == name {
			return parent.Content[i], parent.Content[i+1], true
		}
	}
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if strings.EqualFold(parent.Content[i].Value, name) {
			return parent.Content[i], parent.Content[i+1], true
		}
	}
	return nil, nil, false
}

func encodeDocument(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeConfigFile replaces the config atomically, keeping the file's mode and
// (on Unix) its owner so a root-run "config set" cannot lock the service out
// of its own config.
func writeConfigFile(path string, data []byte) error {
	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".bedrud-config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := copyFileOwner(path, tmpName); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
