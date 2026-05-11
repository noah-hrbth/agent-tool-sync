package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/gitignore"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// isStdinTty reports whether stdin is a terminal. Used to decide whether the
// CLI may prompt the user interactively.
func isStdinTty() bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi == nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// maxInvalidPromptRetries caps the y/n re-prompt loop so a stream of unexpected
// input can't lock the CLI into an infinite read.
const maxInvalidPromptRetries = 3

// applyGitignoreFlowCLI runs the CLI-side gitignore prompt + apply logic.
//
// Behavior:
//   - At user scope: no-op.
//   - Already prompted, Manage=true: refresh the managed block.
//   - Already prompted, Manage=false: no-op.
//   - Not prompted, no tty: log a one-line hint, leave Prompted=false.
//   - Not prompted, tty: prompt y/n (up to maxInvalidPromptRetries invalid retries).
//     y applies, n removes any existing block. Either persists Manage + Prompted.
//
// Returns true when cfg.Gitignore was mutated and needs persisting.
func applyGitignoreFlowCLI(
	workspace string,
	cfg *config.Config,
	adapters []tools.Adapter,
	scope tools.Scope,
	in io.Reader,
	out io.Writer,
	isTty bool,
) (bool, error) {
	if scope != tools.ScopeProject {
		return false, nil
	}

	if cfg.Gitignore.Prompted {
		if cfg.Gitignore.Manage {
			return false, gitignore.Update(workspace, gitignore.Compute(adapters))
		}
		return false, nil
	}

	if !isTty {
		fmt.Fprintln(out, "Tip: agentsync can manage a .gitignore block for derived tool dirs. Run interactively or edit .agentsync/config.yaml (gitignore.manage / gitignore.prompted) to configure.")
		return false, nil
	}

	entries := gitignore.Compute(adapters)
	answer, ok := promptYesNo(in, out, entries)
	if !ok {
		fmt.Fprintln(out, "No valid response; leaving .gitignore unchanged. Re-run sync to choose.")
		return false, nil
	}

	if answer {
		if err := gitignore.Update(workspace, entries); err != nil {
			return false, err
		}
		cfg.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
		fmt.Fprintln(out, "Updated .gitignore with agentsync-managed block.")
		return true, nil
	}

	if err := gitignore.Remove(workspace); err != nil {
		return false, err
	}
	cfg.Gitignore = config.GitignoreConfig{Manage: false, Prompted: true}
	fmt.Fprintln(out, "Skipped .gitignore management; removed any existing managed block.")
	return true, nil
}

// promptYesNo asks the user whether to add the managed gitignore block. Returns
// (answer, true) on a valid y/n response, (false, false) when the user supplies
// maxInvalidPromptRetries unrecognized answers in a row (or input ends early).
func promptYesNo(in io.Reader, out io.Writer, entries []string) (bool, bool) {
	fmt.Fprintf(out, "agentsync can add %d derived tool entries to .gitignore (%s).\n", len(entries), summarize(entries))
	fmt.Fprintln(out, "This is reversible — re-run sync after editing .agentsync/config.yaml to change later.")
	reader := bufio.NewReader(in)
	for i := 0; i < maxInvalidPromptRetries; i++ {
		fmt.Fprint(out, "Add agentsync block to .gitignore? [y/n]: ")
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return false, false
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, true
		case "n", "no":
			return false, true
		}
	}
	return false, false
}

// summarize returns a short comma-separated preview of the first few entries.
func summarize(entries []string) string {
	const max = 3
	if len(entries) <= max {
		return strings.Join(entries, ", ")
	}
	return strings.Join(entries[:max], ", ") + fmt.Sprintf(", +%d more", len(entries)-max)
}
