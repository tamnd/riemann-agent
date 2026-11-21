package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/riemann-agent/llm"
)

// LedgerFile is the path of the ledger in the content repository.
const LedgerFile = "agent/ledger.jsonl"

// Entry is one call of a model.
type Entry struct {
	Time   time.Time `json:"time"`
	Role   string    `json:"role"`
	Target string    `json:"target"` // the Lean name or the id that the call drafted
	Model  string    `json:"model"`
	Prompt string    `json:"prompt"` // <role>/<version>
	Usage  llm.Usage `json:"usage"`
	Cost   float64   `json:"cost"`
	File   string    `json:"file,omitempty"`  // the file that the draft wrote
	Error  string    `json:"error,omitempty"` // the error of the call, when it failed after the model answered
}

// Ledger is the record of the calls.
type Ledger struct {
	path    string
	Entries []Entry
}

// OpenLedger reads the ledger of the content repository. A missing file gives an empty ledger.
func OpenLedger(root string) (*Ledger, error) {
	l := &Ledger{path: filepath.Join(root, LedgerFile)}
	f, err := os.Open(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", LedgerFile, n, err)
		}
		l.Entries = append(l.Entries, e)
	}
	return l, sc.Err()
}

// Spent returns the spend of the UTC day of t.
func (l *Ledger) Spent(t time.Time) float64 {
	day := t.UTC().Format(time.DateOnly)
	sum := 0.0
	for _, e := range l.Entries {
		if e.Time.UTC().Format(time.DateOnly) == day {
			sum += e.Cost
		}
	}
	return sum
}

// Add appends an entry to the file and to the ledger.
func (l *Ledger) Add(e Entry) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- a file of the content repository
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	l.Entries = append(l.Entries, e)
	return f.Close()
}

// ErrBudget is the error for a call that the budget does not allow.
var ErrBudget = errors.New("the budget does not allow the call")

// Allow checks that a call with the estimate est fits the budget, after the spend run of this run.
func (c *Config) Allow(l *Ledger, now time.Time, run, est float64) error {
	day := l.Spent(now)
	switch {
	case c.Budget.Day == 0 || c.Budget.Run == 0:
		return fmt.Errorf("%w: agent.yaml sets no budget for the day and the run", ErrBudget)
	case day+est > c.Budget.Day:
		return fmt.Errorf("%w: the day has spent $%.2f of $%.2f, and the call can cost $%.2f", ErrBudget, day, c.Budget.Day, est)
	case run+est > c.Budget.Run:
		return fmt.Errorf("%w: the run has spent $%.2f of $%.2f, and the call can cost $%.2f", ErrBudget, run, c.Budget.Run, est)
	}
	return nil
}
