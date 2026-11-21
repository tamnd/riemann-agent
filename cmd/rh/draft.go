package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/riemann-agent/agent"
	"github.com/tamnd/riemann-agent/index"
	"github.com/tamnd/riemann-agent/llm"
	"github.com/tamnd/riemann-agent/steps"
)

func runDraft(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rh draft", flag.ContinueOnError)
	fs.SetOutput(stderr)
	content := fs.String("content", "../riemann", "the root of the content repository")
	indexDir := fs.String("index", "", "the records of rh index (required)")
	src := fs.String("src", "../riemann-lean/lean/src", "the root of the Lean source")
	spineDir := fs.String("spine", "", "the output of rh spine, for the order of the queue")
	queue := fs.Int("queue", 0, "draft the next N names of the queue, in the order of the spine")
	dry := fs.Bool("n", false, "print the prompts and the cost estimates, and call no model")
	pr := fs.Bool("pr", false, "commit the drafts on a new branch of the content repository and open a pull request with the label agent-draft")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: rh draft [flags] <Lean name or step id>...")
		fmt.Fprintln(stderr, "       rh draft [flags] -queue N")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "rh draft writes each step with the prover and the status drafted. The model and the budget come from agent.yaml of the content repository. With no budget, no call goes out. The key is in ANTHROPIC_API_KEY.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *indexDir == "" || (*queue == 0) == (fs.NArg() == 0) {
		return fmt.Errorf("%w: draft needs -index, and either names or -queue N", errUsage)
	}
	env, err := loadAgentEnv(*content, *indexDir, *src, *spineDir)
	if err != nil {
		return err
	}
	names := fs.Args()
	if *queue > 0 {
		names = env.Queue(*queue)
		if len(names) == 0 {
			fmt.Fprintln(stdout, "the queue is empty: each name in an open list has a step")
			return nil
		}
	}
	byID := steps.ByID(env.Steps)
	for i, n := range names {
		if s, ok := byID[n]; ok {
			names[i] = s.Decl
		}
	}
	cfg, err := agent.LoadConfig(*content)
	if err != nil {
		return err
	}
	ledger, err := agent.OpenLedger(*content)
	if err != nil {
		return err
	}
	d := &agent.Drafter{Root: *content, Env: env, Config: cfg, Ledger: ledger, Out: stdout, Now: time.Now}
	if !*dry {
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return errors.New("ANTHROPIC_API_KEY is not set; use -n for a dry run")
		}
		d.Client = llm.NewClaude(key)
	}
	ctx := context.Background()
	var done []*agent.Result
	total := 0.0
	for _, n := range names {
		res, err := d.Prove(ctx, n)
		if err != nil {
			if len(done) > 0 && *pr {
				fmt.Fprintf(stderr, "rh draft: %s: %v\n", n, err)
				break
			}
			return fmt.Errorf("%s: %w", n, err)
		}
		if res.Dry {
			total += res.Estimate
			continue
		}
		total += res.Cost
		done = append(done, res)
		fmt.Fprintf(stdout, "%s: wrote %s for $%.4f", n, res.Step.File, res.Cost)
		if len(res.Parents) > 0 {
			fmt.Fprintf(stdout, ", cited by %s", strings.Join(res.Parents, ", "))
		}
		fmt.Fprintln(stdout)
	}
	if *dry {
		fmt.Fprintf(stdout, "%d prompts, at most $%.4f in all\n", len(names), total)
		return nil
	}
	fmt.Fprintf(stdout, "%d steps drafted for $%.4f\n", len(done), total)
	if *pr && len(done) > 0 {
		return openDraftPR(*content, done, stdout)
	}
	return nil
}

// loadAgentEnv reads what the roles need.
func loadAgentEnv(content, indexDir, src, spineDir string) (*agent.Env, error) {
	ix, err := index.Load(indexDir)
	if err != nil {
		return nil, err
	}
	env := &agent.Env{Index: ix, Graph: index.NewGraph(ix), Src: src}
	if env.Steps, err = steps.Load(content); err != nil {
		return nil, err
	}
	if env.Glossary, err = steps.LoadGlossary(content); err != nil {
		return nil, err
	}
	if env.Lit, err = steps.LoadLiterature(content); err != nil {
		return nil, err
	}
	if spineDir != "" {
		if env.Spine, err = index.ReadNodes(filepath.Join(spineDir, "spine.jsonl.gz")); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// openDraftPR commits the drafts on a new branch and opens a pull request with the review list of spec 09.2.
func openDraftPR(content string, done []*agent.Result, stdout io.Writer) error {
	branch := "draft/" + time.Now().UTC().Format("20060102-150405") + "-" + done[0].Step.ID
	files := []string{agent.LedgerFile}
	var list strings.Builder
	cost := 0.0
	for _, r := range done {
		files = append(files, r.Step.File)
		for _, p := range r.Parents {
			files = append(files, "proof/steps/"+p+".md")
		}
		cost += r.Cost
		fmt.Fprintf(&list, "- [ ] [%s](../blob/%s/%s), the step of `%s`\n", r.Step.ID, branch, r.Step.File, r.Name)
	}
	title := fmt.Sprintf("Draft %d steps with the prover", len(done))
	if len(done) == 1 {
		title = "Draft the step " + done[0].Step.ID + " with the prover"
	}
	body := fmt.Sprintf("rh draft wrote these steps with the prover (%s). Each step has the status drafted. A person must read each one before its status goes up. The calls cost $%.4f, and agent/ledger.jsonl records them.\n\nRead each step against its Lean text, and tick it when it is right:\n\n%s\nFor each step, check that the statement has each hypothesis, that the proof follows the Lean proof, that the names in routine are really routine, and that the text says nothing that the code does not show.\n", done[0].Step.Prompt, cost, list.String())
	run := func(name string, args ...string) error {
		cmd := exec.Command(name, args...) // #nosec G204 -- fixed commands with arguments from rh
		cmd.Dir = content
		cmd.Stdout, cmd.Stderr = stdout, stdout
		return cmd.Run()
	}
	for _, c := range [][]string{
		{"git", "checkout", "-b", branch},
		append([]string{"git", "add", "--"}, files...),
		{"git", "commit", "-m", title},
		{"git", "push", "-u", "origin", branch},
		{"gh", "pr", "create", "--title", title, "--label", "agent-draft", "--body", body},
	} {
		if err := run(c[0], c[1:]...); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(c[:2], " "), err)
		}
	}
	return nil
}
