package steps

import (
	"bytes"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Marshal returns the step file: the front matter in the order of the step files, with the lists in flow style, then the body.
func (s *Step) Marshal() ([]byte, error) {
	m := &yaml.Node{Kind: yaml.MappingNode}
	str := func(k, v string) {
		if v != "" {
			m.Content = append(m.Content, key(k), &yaml.Node{Kind: yaml.ScalarNode, Value: v})
		}
	}
	list := func(k string, vs []string) {
		if len(vs) == 0 {
			return
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, v := range vs {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
		}
		m.Content = append(m.Content, key(k), seq)
	}
	str("id", s.ID)
	str("title", s.Title)
	str("decl", s.Decl)
	str("chapter", s.Chapter)
	str("status", s.Status)
	str("drafted_by", s.DraftedBy)
	str("prompt", s.Prompt)
	list("uses", s.Uses)
	list("concepts", s.Concepts)
	list("routine", s.Routine)
	list("open", s.Open)
	if len(s.Reviews) > 0 {
		var rv yaml.Node
		if err := rv.Encode(s.Reviews); err != nil {
			return nil, err
		}
		m.Content = append(m.Content, key("reviews"), &rv)
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	out := []byte("---\n")
	out = append(out, b.Bytes()...)
	out = append(out, "---\n"...)
	out = append(out, strings.TrimLeft(s.Body, "\n")...)
	return out, nil
}

func key(k string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: k} }
