// Package state manages the loop state file with YAML-like frontmatter.
package state

import (
	"strings"
)

// OrderedMap preserves insertion order of key-value pairs.
// This ensures round-trip fidelity when reading and writing frontmatter.
type OrderedMap struct {
	keys   []string
	values map[string]string
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: make(map[string]string)}
}

// Set adds or updates a key-value pair.
// New keys are appended; existing keys keep their position.
func (m *OrderedMap) Set(key, value string) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get retrieves the value for a key and whether it exists.
func (m *OrderedMap) Get(key string) (string, bool) {
	v, ok := m.values[key]
	return v, ok
}

// GetOr retrieves the value for a key, or fallback if not set.
func (m *OrderedMap) GetOr(key, fallback string) string {
	if v, ok := m.values[key]; ok {
		return v
	}
	return fallback
}

// Delete removes a key.
func (m *OrderedMap) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// Keys returns all keys in insertion order.
func (m *OrderedMap) Keys() []string {
	out := make([]string, len(m.keys))
	copy(out, m.keys)
	return out
}

// Len returns the number of entries.
func (m *OrderedMap) Len() int {
	return len(m.keys)
}

// Frontmatter represents a parsed frontmatter block with body.
type Frontmatter struct {
	Fields *OrderedMap
	Body   string
}

// ParseFrontmatter parses content with YAML-like frontmatter between --- delimiters.
// Returns the parsed frontmatter and any body content after the closing ---.
func ParseFrontmatter(content string) *Frontmatter {
	fm := &Frontmatter{Fields: NewOrderedMap()}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		fm.Body = content
		return fm
	}

	// Find closing ---
	closingIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closingIdx = i
			break
		}
	}

	if closingIdx == -1 {
		// No closing delimiter — treat entire content as body
		fm.Body = content
		return fm
	}

	// Parse key: value pairs between delimiters
	for i := 1; i < closingIdx; i++ {
		line := lines[i]
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		if key != "" {
			fm.Fields.Set(key, value)
		}
	}

	// Everything after closing --- is body
	if closingIdx+1 < len(lines) {
		fm.Body = strings.Join(lines[closingIdx+1:], "\n")
	}

	return fm
}

// Render produces the full file content: frontmatter + body.
func (fm *Frontmatter) Render() string {
	var b strings.Builder

	b.WriteString("---\n")
	for _, key := range fm.Fields.Keys() {
		v, _ := fm.Fields.Get(key)
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	b.WriteString("---\n")

	if fm.Body != "" {
		b.WriteString(fm.Body)
		// Ensure trailing newline
		if !strings.HasSuffix(fm.Body, "\n") {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderFrontmatterOnly produces just the frontmatter block (no body).
func (fm *Frontmatter) RenderFrontmatterOnly() string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range fm.Fields.Keys() {
		v, _ := fm.Fields.Get(key)
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	return b.String()
}
