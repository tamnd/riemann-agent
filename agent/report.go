package agent

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/riemann-agent/steps"
)

// Edit is the number of lines that people changed in a drafted file after the draft.
type Edit struct {
	Added, Deleted int
}

// EditFunc returns the edit of people on a file of the content repository since the commit that added it.
type EditFunc func(file string) (Edit, error)

// Report is the weekly report of spec 09.4: the steps drafted, the steps accepted, the edit size of the people and the cost.
type Report struct {
	Since, Until time.Time
	Calls        int
	Failed       int
	Cost         float64
	CostByRole   map[string]float64
	Drafted      []ReportStep // the steps with a drafted_by field
	NewDrafts    int          // the drafts of the ledger in the window
}

// ReportStep is one step that a model drafted.
type ReportStep struct {
	ID, File, Status, DraftedBy string
	Accepted                    bool // a person raised the status above drafted
	Edit                        Edit
}

// MakeReport reads the ledger entries in [since, until) and the steps that a model drafted. The function edit can be nil, and then the report has no edit size.
func MakeReport(l *Ledger, ss []*steps.Step, since, until time.Time, edit EditFunc) (*Report, error) {
	r := &Report{Since: since, Until: until, CostByRole: map[string]float64{}}
	for _, e := range l.Entries {
		if e.Time.Before(since) || !e.Time.Before(until) {
			continue
		}
		r.Calls++
		r.Cost += e.Cost
		r.CostByRole[e.Role] += e.Cost
		if e.Error != "" {
			r.Failed++
		} else if e.File != "" {
			r.NewDrafts++
		}
	}
	for _, s := range ss {
		if s.DraftedBy == "" {
			continue
		}
		rs := ReportStep{ID: s.ID, File: s.File, Status: s.Status, DraftedBy: s.DraftedBy, Accepted: steps.Rank(s.Status) > steps.Rank("drafted")}
		if edit != nil {
			e, err := edit(s.File)
			if err != nil {
				return nil, fmt.Errorf("agent: the edit size of %s: %w", s.File, err)
			}
			rs.Edit = e
		}
		r.Drafted = append(r.Drafted, rs)
	}
	sort.Slice(r.Drafted, func(i, j int) bool { return r.Drafted[i].ID < r.Drafted[j].ID })
	return r, nil
}

// Accepted returns the number of drafted steps that a person accepted.
func (r *Report) Accepted() int {
	n := 0
	for _, s := range r.Drafted {
		if s.Accepted {
			n++
		}
	}
	return n
}

// Write writes the report as Markdown, for an issue comment or a file.
func (r *Report) Write(w io.Writer) {
	fmt.Fprintf(w, "# The agent report, %s to %s\n\n", r.Since.Format("2006-01-02"), r.Until.Add(-time.Second).Format("2006-01-02"))
	fmt.Fprintf(w, "In this time the agents made %d calls, and %d of them failed. They wrote %d drafts. The cost was $%.2f.", r.Calls, r.Failed, r.NewDrafts, r.Cost)
	if len(r.CostByRole) > 0 {
		var roles []string
		for k := range r.CostByRole {
			roles = append(roles, k)
		}
		sort.Strings(roles)
		var parts []string
		for _, k := range roles {
			parts = append(parts, fmt.Sprintf("%s $%.2f", k, r.CostByRole[k]))
		}
		fmt.Fprintf(w, " By role: %s.", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	if len(r.Drafted) == 0 {
		fmt.Fprintln(w, "No step of the content repository has a drafted_by field.")
		return
	}
	added, deleted := 0, 0
	for _, s := range r.Drafted {
		added += s.Edit.Added
		deleted += s.Edit.Deleted
	}
	fmt.Fprintf(w, "In all, %d steps came from a model. People accepted %d of them. Since the drafts, people added %d lines and deleted %d lines in these files.\n\n", len(r.Drafted), r.Accepted(), added, deleted)
	fmt.Fprintln(w, "| Step | Drafted by | Status | Lines added | Lines deleted |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, s := range r.Drafted {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d |\n", s.ID, s.DraftedBy, s.Status, s.Edit.Added, s.Edit.Deleted)
	}
}
