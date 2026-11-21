// Package prompts holds the prompts of the roles of rh draft (spec 09). Each file is one role. The front matter gives the role and the version, the text before the line "=== prompt ===" is the system prompt, and the text after it is a Go template for the prompt of one target.
//
// Change the version of a file when its text changes, because each draft records the version of its prompt.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"go.yaml.in/yaml/v3"
)

// Files holds the prompt files.
//
//go:embed *.md
var Files embed.FS

// sep is the line between the system prompt and the template.
const sep = "\n=== prompt ===\n"

// Prompt is one prompt file.
type Prompt struct {
	Role    string `yaml:"role"`
	Version int    `yaml:"version"`
	System  string `yaml:"-"`

	tmpl *template.Template
}

// ID returns <role>/<version>, the value of the field prompt of a draft.
func (p *Prompt) ID() string { return fmt.Sprintf("%s/%d", p.Role, p.Version) }

// Load reads the prompt of a role.
func Load(role string) (*Prompt, error) {
	data, err := Files.ReadFile(role + ".md")
	if err != nil {
		return nil, fmt.Errorf("prompts: no prompt for the role %q", role)
	}
	return parse(role, data)
}

func parse(role string, data []byte) (*Prompt, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, fmt.Errorf("prompts: %s.md has no front matter", role)
	}
	front, rest, ok := strings.Cut(string(data[4:]), "\n---\n")
	if !ok {
		return nil, fmt.Errorf("prompts: %s.md has no end of the front matter", role)
	}
	p := &Prompt{}
	if err := yaml.Unmarshal([]byte(front), p); err != nil {
		return nil, fmt.Errorf("prompts: %s.md: %w", role, err)
	}
	if p.Role != role || p.Version < 1 {
		return nil, fmt.Errorf("prompts: %s.md must have role %s and a version of 1 or more", role, role)
	}
	system, body, ok := strings.Cut(rest, sep)
	if !ok {
		return nil, fmt.Errorf("prompts: %s.md has no line === prompt ===", role)
	}
	p.System = strings.TrimSpace(system)
	t, err := template.New(role).Option("missingkey=error").Parse(strings.TrimSpace(body) + "\n")
	if err != nil {
		return nil, fmt.Errorf("prompts: %s.md: %w", role, err)
	}
	p.tmpl = t
	return p, nil
}

// Render fills the template with data and returns the prompt of one target.
func (p *Prompt) Render(data any) (string, error) {
	var b strings.Builder
	if err := p.tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("prompts: %s: %w", p.ID(), err)
	}
	return b.String(), nil
}
