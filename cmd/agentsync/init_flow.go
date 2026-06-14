package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/safepath"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
	"github.com/noah-hrbth/agentsync/internal/wizard"
)

// wizardRunner abstracts wizard.Run so init/root flows can be tested with a
// stub instead of a live Bubble Tea program.
type wizardRunner func(ws string, scope tools.Scope, cfg *config.Config, options []wizard.SourceOption) (wizard.Outcome, error)

// defaultWizardRunner is the production wizardRunner; the only call site of
// wizard.Run in the cmd layer.
var defaultWizardRunner wizardRunner = wizard.Run

// runInitFlow drives `agentsync init`: --from validation first (an invalid
// import source must never cost the existing canonical), then the re-init
// guard, then detection-based tool enablement, then one of three paths —
// headless import (--from), the interactive wizard (tty with import options),
// or a fresh scaffold.
func runInitFlow(ws string, scope tools.Scope, fromKey string, force bool, wiz wizardRunner, in io.Reader, out io.Writer, isTty bool) error {
	var fromTool tools.Tool
	if fromKey != "" {
		t, err := resolveImportTool(fromKey, scope)
		if err != nil {
			return err
		}
		if !tools.DetectAtScope(ws, t, scope).Found {
			return fmt.Errorf("nothing to import: %s not found at %s scope", fromKey, scope)
		}
		fromTool = t
	}

	proceed, err := reinitGuard(ws, force, in, out, isTty)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	if fromKey != "" {
		cfg := config.WithEnabled(tools.Names(), wizard.DetectedNames(ws, scope))
		return initFromTool(ws, scope, fromTool, cfg, out)
	}

	detectedNames, options := wizard.BuildOptions(ws, scope)
	cfg := config.WithEnabled(tools.Names(), detectedNames)

	if isTty && len(options) > 0 {
		outcome, err := wiz(ws, scope, cfg, options)
		if err != nil {
			return err
		}
		return handleWizardOutcome(outcome, out)
	}

	return scaffoldFresh(ws, scope, cfg, out)
}

// runRootFlow handles a bare `agentsync` invocation on an uninitialized scope:
// non-tty errors with the init hint; a tty runs the wizard when import options
// exist, otherwise scaffolds fresh. Returns without launching the TUI.
func runRootFlow(ws string, scope tools.Scope, wiz wizardRunner, out io.Writer, isTty bool) error {
	if !isTty {
		return notInitializedError(scope)
	}

	detectedNames, options := wizard.BuildOptions(ws, scope)
	cfg := config.WithEnabled(tools.Names(), detectedNames)

	if len(options) > 0 {
		outcome, err := wiz(ws, scope, cfg, options)
		if err != nil {
			return err
		}
		return handleWizardOutcome(outcome, out)
	}

	return scaffoldFresh(ws, scope, cfg, out)
}

// initFromTool imports headlessly from the already-validated tool: scaffold,
// bulk import, plain-text summary plus a scope-appropriate run hint.
func initFromTool(ws string, scope tools.Scope, t tools.Tool, cfg *config.Config, out io.Writer) error {
	if err := syncer.Scaffold(ws, scope, cfg); err != nil {
		return err
	}
	summary, err := syncer.ImportFromTool(ws, t, scope)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Initialized .agentsync/ in %s (scope: %s)\n", ws, scope)
	fmt.Fprintf(out, "%s: %s\n", t.Meta.Name, syncer.FormatImportSummary(summary))
	fmt.Fprintf(out, "Run '%s' to review and sync.\n", scope.Command())
	return nil
}

// resolveImportTool maps a --from key to its registered tool, rejecting
// unknown keys (listing the import-eligible keys at scope) and tools with no
// importable concepts at scope.
func resolveImportTool(fromKey string, scope tools.Scope) (tools.Tool, error) {
	for _, t := range tools.All() {
		if t.Meta.Key != fromKey {
			continue
		}
		if !tools.ImportEligible(t, scope) {
			return tools.Tool{}, fmt.Errorf("cannot import from %s: no importable concepts at %s scope", fromKey, scope)
		}
		return t, nil
	}
	return tools.Tool{}, fmt.Errorf("unknown tool %q — importable tools at %s scope: %s", fromKey, scope, strings.Join(eligibleToolKeys(scope), ", "))
}

// eligibleToolKeys lists the keys of all import-eligible tools at scope, in
// registry order.
func eligibleToolKeys(scope tools.Scope) []string {
	var keys []string
	for _, t := range tools.All() {
		if tools.ImportEligible(t, scope) {
			keys = append(keys, t.Meta.Key)
		}
	}
	return keys
}

// errWizardReported wraps a wizard failure that the wizard already rendered
// inline, so main() can exit non-zero without reprinting the message.
var errWizardReported = errors.New("wizard failed")

// handleWizardOutcome maps a wizard Outcome to the flow result: aborts are
// clean exits with a short note, wizard-level failures propagate (wrapped so
// main() skips the duplicate print), and success prints nothing extra (the
// wizard already rendered its done view).
func handleWizardOutcome(outcome wizard.Outcome, out io.Writer) error {
	if outcome.Aborted {
		fmt.Fprintln(out, "Init aborted — nothing was created.")
		return nil
	}
	if outcome.Err != nil {
		return fmt.Errorf("%w: %w", errWizardReported, outcome.Err)
	}
	return nil
}

// scaffoldFresh creates the .agentsync/ skeleton with the given config and
// prints the scope-conditional success messages.
func scaffoldFresh(ws string, scope tools.Scope, cfg *config.Config, out io.Writer) error {
	if err := syncer.Scaffold(ws, scope, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "Initialized .agentsync/ in %s (scope: %s)\n", ws, scope)
	if scope == tools.ScopeUser {
		fmt.Fprintln(out, "Edit ~/.agentsync/AGENTS.md, then run 'agentsync sync --global' or launch the TUI with 'agentsync --global'.")
	} else {
		fmt.Fprintln(out, "Edit .agentsync/AGENTS.md, then run 'agentsync sync' or launch the TUI with 'agentsync'.")
	}
	return nil
}

// reinitGuard gates init on an already-initialized scope. When .agentsync/
// does not exist it proceeds silently; a plain file at that path is a
// repairable error, never wiped. When the dir exists: --force wipes without
// prompting; a tty asks for confirmation (default No) and wipes on yes; a
// non-tty without --force errors. Returns (proceed, err) — proceed=false with
// nil err is a clean user abort.
func reinitGuard(ws string, force bool, in io.Reader, out io.Writer, isTty bool) (bool, error) {
	target := filepath.Join(ws, ".agentsync")
	info, err := os.Stat(target)
	if err != nil {
		return true, nil
	}
	if !info.IsDir() {
		return false, agentsyncNotDirError(ws)
	}

	if force {
		return true, safepath.RemoveAll(ws, ".agentsync")
	}

	if !isTty {
		return false, fmt.Errorf(".agentsync/ already exists at %s — re-run with --force to wipe and re-initialize", target)
	}

	fmt.Fprintf(out, "⚠ .agentsync/ already exists at %s — re-initializing will DELETE its contents (rules, skills, agents, commands, AGENTS.md, config, snapshot). Continue? [y/N] ", target)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out, "Aborted — .agentsync/ left untouched.")
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, safepath.RemoveAll(ws, ".agentsync")
	default:
		fmt.Fprintln(out, "Aborted — .agentsync/ left untouched.")
		return false, nil
	}
}
