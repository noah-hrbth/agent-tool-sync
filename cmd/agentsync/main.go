package main

import (
	"fmt"
	"os"
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
)

var rootCmd = &cobra.Command{
	Use:   "agentsync",
	Short: "Sync agent configs across Claude Code, Cursor, OpenCode, and more",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, scope, err := resolveBase()
		if err != nil {
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
	rootCmd.AddCommand(initCmd, syncCmd, statusCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveBase returns the base directory and scope for the current invocation.
// --global and --workspace are mutually exclusive. Default is project scope at cwd.
func resolveBase() (string, tools.Scope, error) {
	if globalFlag && workspace != "" {
		return "", tools.ScopeProject, fmt.Errorf("--global and --workspace are mutually exclusive")
	}
	if globalFlag {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", tools.ScopeUser, err
		}
		return home, tools.ScopeUser, nil
	}
	if workspace != "" {
		return workspace, tools.ScopeProject, nil
	}
	cwd, err := os.Getwd()
	return cwd, tools.ScopeProject, err
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

	base := ws + "/.agentsync"
	for _, dir := range []string{base, base + "/skills", base + "/agents", base + "/commands", base + "/rules", base + "/.state"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	agentsPath := base + "/AGENTS.md"
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		starter := "# Project Rules\n\nAdd your AI agent instructions here.\n" +
			"This file is synced to all enabled AI tools by agentsync.\n"
		if scope == tools.ScopeUser {
			starter = "# User Rules\n\nPersonal AI agent instructions applied across all your projects.\n" +
				"This file is synced to user-level tool config dirs (~/.claude, ~/.codex, etc.) by agentsync.\n"
		}
		if err := os.WriteFile(agentsPath, []byte(starter), 0o644); err != nil {
			return err
		}
	}

	cfg := config.Default(tools.Names())
	if err := config.Save(ws, cfg); err != nil {
		return err
	}

	fmt.Printf("Initialized .agentsync/ in %s (scope: %s)\n", ws, scope)
	if scope == tools.ScopeUser {
		fmt.Println("Edit ~/.agentsync/AGENTS.md, then run 'agentsync sync --global' or launch the TUI with 'agentsync --global'.")
	} else {
		fmt.Println("Edit .agentsync/AGENTS.md, then run 'agentsync sync' or launch the TUI with 'agentsync'.")
	}
	return nil
}

func runSync(cmd *cobra.Command, _ []string) error {
	ws, scope, err := resolveBase()
	if err != nil {
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
