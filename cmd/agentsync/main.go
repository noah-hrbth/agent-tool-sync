package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
	"github.com/noah-hrbth/agentsync/internal/tui"
)

var version = "dev"

var (
	workspace  string
	globalFlag bool
	fromFlag   string
	forceFlag  bool
)

var rootCmd = &cobra.Command{
	Use:   "agentsync",
	Short: "Sync agent configs across Claude Code, Cursor, OpenCode, and more",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, scope, err := resolveBase()
		if err != nil {
			return err
		}
		if err := requireInitialized(ws, scope); err != nil {
			if errors.Is(err, errNotInitialized) {
				return runRootFlow(ws, scope, defaultWizardRunner, cmd.OutOrStdout(), isStdinTty())
			}
			return err
		}
		return tui.Run(ws, scope, tools.All())
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold .agentsync/ in the workspace",
	RunE:  runInit,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "One-way sync: canonical → all enabled tool folders",
	RunE:  runSync,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report sync status for all enabled tools",
	RunE:  runStatus,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print agentsync version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func main() {
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "", "path to workspace (default: current directory)")
	rootCmd.PersistentFlags().BoolVarP(&globalFlag, "global", "g", false, "operate at user scope (canonical at ~/.agentsync, syncs to user-level tool dirs)")
	initCmd.Flags().StringVar(&fromFlag, "from", "", "import existing config from a tool (e.g. claude, cursor) instead of starting fresh")
	initCmd.Flags().BoolVar(&forceFlag, "force", false, "wipe an existing .agentsync/ without prompting before re-initializing")
	rootCmd.AddCommand(initCmd, syncCmd, statusCmd, versionCmd)

	// report runtime errors ourselves: the wizard already renders its own
	// failure inline, and usage text is noise on a runtime (non-arg) error
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		// the wizard already showed its failure inline; don't reprint it
		if !errors.Is(err, errWizardReported) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

// resolveBase returns the base directory and scope for the current invocation.
// --global and --workspace are mutually exclusive. Default is project scope at cwd.
func resolveBase() (string, tools.Scope, error) {
	if globalFlag && workspace != "" {
		return "", tools.ScopeProject, fmt.Errorf("--global and --workspace are mutually exclusive")
	}

	ws := workspace
	scope := tools.ScopeProject
	switch {
	case globalFlag:
		scope = tools.ScopeUser
		home, err := os.UserHomeDir()
		if err != nil {
			return "", scope, err
		}
		ws = home
	case workspace == "":
		cwd, err := os.Getwd()
		if err != nil {
			return "", scope, err
		}
		ws = cwd
	}

	// resolve the workspace root once so safepath can forbid symlink crossings
	// strictly below it without re-resolving on every write
	resolved, err := filepath.EvalSymlinks(ws)
	if err != nil {
		return "", scope, fmt.Errorf("resolve workspace %q: %w", ws, err)
	}
	return resolved, scope, nil
}

// requireInitialized verifies that ws holds an initialized scope: .agentsync/
// must exist and be a directory. Returns a scope-aware "run 'agentsync init'"
// error when missing, or a repair hint when .agentsync is a plain file.
func requireInitialized(ws string, scope tools.Scope) error {
	info, err := os.Stat(filepath.Join(ws, ".agentsync"))
	if err != nil {
		return notInitializedError(scope)
	}
	if !info.IsDir() {
		return agentsyncNotDirError(ws)
	}
	return nil
}

// agentsyncNotDirError reports the repairable state where .agentsync exists but
// is a plain file rather than the expected directory.
func agentsyncNotDirError(ws string) error {
	return fmt.Errorf(".agentsync at %s is not a directory — remove it, then run 'agentsync init'", ws)
}

// errNotInitialized marks the missing-.agentsync/ case so callers can
// distinguish it from repairable states like .agentsync being a plain file
var errNotInitialized = errors.New("not initialized")

// notInitializedError builds the scope-aware not-initialized error.
func notInitializedError(scope tools.Scope) error {
	initCmd := "agentsync init"
	if scope == tools.ScopeUser {
		initCmd = "agentsync init --global"
	}
	return fmt.Errorf("%w — run '%s'", errNotInitialized, initCmd)
}

func loadState(ws string) (*canonical.Canonical, *config.Config, error) {
	c, err := canonical.Load(ws)
	if err != nil {
		return nil, nil, fmt.Errorf("load canonical: %w", err)
	}
	cfg, err := config.Load(ws, tools.Names())
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	return c, cfg, nil
}

func runInit(cmd *cobra.Command, _ []string) error {
	ws, scope, err := resolveBase()
	if err != nil {
		return err
	}
	return runInitFlow(ws, scope, fromFlag, forceFlag, defaultWizardRunner, cmd.InOrStdin(), cmd.OutOrStdout(), isStdinTty())
}

func runSync(cmd *cobra.Command, _ []string) error {
	ws, scope, err := resolveBase()
	if err != nil {
		return err
	}
	if err := requireInitialized(ws, scope); err != nil {
		return err
	}
	c, cfg, err := loadState(ws)
	if err != nil {
		return err
	}

	adapters := tools.All()
	results, err := syncer.Status(ws, c, adapters, cfg, scope)
	if err != nil {
		return fmt.Errorf("status check: %w", err)
	}

	var divergent []syncer.FileResult
	for _, r := range results {
		if r.Status == syncer.StatusDivergent {
			divergent = append(divergent, r)
		}
	}
	if len(divergent) > 0 {
		fmt.Fprintf(os.Stderr, "Divergent files detected (externally edited):\n")
		for _, r := range divergent {
			fmt.Fprintf(os.Stderr, "  ▲ %s  [%s]\n", r.Path, r.ToolName)
		}
		fmt.Fprintf(os.Stderr, "\nRun 'agentsync' (TUI) to resolve, or re-run with --force to overwrite.\n")
		return fmt.Errorf("unresolved divergences")
	}

	changed, err := applyGitignoreFlowCLI(ws, cfg, adapters, scope, cmd.InOrStdin(), cmd.OutOrStdout(), isStdinTty())
	if err != nil {
		return fmt.Errorf("gitignore: %w", err)
	}
	if changed {
		if err := config.Save(ws, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	result, err := syncer.RunSync(ws, c, adapters, cfg, scope, syncer.SyncOptions{})
	if err != nil {
		return err
	}

	for _, f := range result.Written {
		fmt.Printf("  ✓ %s\n", f.Path)
	}
	for _, f := range result.Skipped {
		fmt.Printf("  – %s (skipped)\n", f.Path)
	}
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  ✗ %v\n", e)
		}
		return fmt.Errorf("%d error(s) during sync", len(result.Errors))
	}
	fmt.Printf("\nSynced %d file(s).\n", len(result.Written))
	return nil
}

func runStatus(cmd *cobra.Command, _ []string) error {
	ws, scope, err := resolveBase()
	if err != nil {
		return err
	}
	if err := requireInitialized(ws, scope); err != nil {
		return err
	}
	c, cfg, err := loadState(ws)
	if err != nil {
		return err
	}

	results, err := syncer.Status(ws, c, tools.All(), cfg, scope)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No tools enabled or no files to sync.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tTOOL\tPATH")
	for _, r := range results {
		icon := statusIcon(r.Status)
		fmt.Fprintf(w, "%s\t%s\t%s\n", icon, r.ToolName, r.Path)
	}
	return w.Flush()
}

func statusIcon(s syncer.FileStatus) string {
	switch s {
	case syncer.StatusSynced:
		return "●"
	case syncer.StatusDivergent:
		return "▲"
	case syncer.StatusMissing:
		return "○"
	default:
		return "+"
	}
}
