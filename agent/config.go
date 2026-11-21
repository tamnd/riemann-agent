// Package agent drafts the text of the written proof with a model (spec 09). Each role reads the records and the content repository, sends one prompt, and writes a draft with the status drafted. A person must check each draft before its status goes up.
//
// The budget in agent.yaml limits the spend for each day and for each run. The ledger agent/ledger.jsonl records each call. With no agent.yaml, the budget is zero and no call goes out.
package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/tamnd/riemann-agent/llm"
)

// Roles are the roles of spec 09.1.
var Roles = []string{"namer", "stater", "prover", "chapterer", "linker", "pathfinder"}

// Config is agent.yaml of the content repository.
type Config struct {
	// Models gives the model of each role. A role that is not in the map uses the default of spec 09.4.
	Models map[string]string `yaml:"models"`
	// Effort gives the effort of each role: low, medium, high, xhigh or max.
	Effort map[string]string `yaml:"effort"`
	// MaxTokens gives the largest answer of each role, in tokens.
	MaxTokens map[string]int64 `yaml:"max_tokens"`
	Budget    Budget           `yaml:"budget"`
}

// Budget is the largest spend in dollars. Zero means that no call goes out.
type Budget struct {
	Day float64 `yaml:"day"` // for one day, in UTC, over all runs
	Run float64 `yaml:"run"` // for one run of rh draft
}

// large are the roles that use the large model by default: the roles that write proofs and chapters.
var large = map[string]bool{"prover": true, "chapterer": true, "stater": true}

// Model returns the model of a role.
func (c *Config) Model(role string) string {
	if m := c.Models[role]; m != "" {
		return m
	}
	if large[role] {
		return llm.ModelLarge
	}
	return llm.ModelSmall
}

// Tokens returns the largest answer of a role.
func (c *Config) Tokens(role string) int64 {
	if n := c.MaxTokens[role]; n > 0 {
		return n
	}
	if large[role] {
		return 16000
	}
	return 2000
}

// Think reports whether the role thinks before it answers. The large roles do.
func (c *Config) Think(role string) bool { return large[role] }

// LoadConfig reads agent.yaml in the root of the content repository. A missing file gives the defaults with a zero budget.
func LoadConfig(root string) (*Config, error) {
	c := &Config{}
	data, err := os.ReadFile(filepath.Join(root, "agent.yaml")) // #nosec G304 -- a file of the content repository
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("agent.yaml: %w", err)
	}
	for r := range c.Models {
		if !isRole(r) {
			return nil, fmt.Errorf("agent.yaml: %q is not a role", r)
		}
	}
	if c.Budget.Day < 0 || c.Budget.Run < 0 {
		return nil, errors.New("agent.yaml: a budget cannot be negative")
	}
	return c, nil
}

func isRole(r string) bool {
	for _, x := range Roles {
		if x == r {
			return true
		}
	}
	return false
}
