// Package hooks executes user-configured post-processing commands.
package hooks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Arsenalist/prx/internal/config"
)

// Result captures the outcome of running a single hook.
type Result struct {
	Name   string
	Output string
	Err    error
}

// Run executes a list of hooks for a given event, piping data to stdin.
// Returns results for each hook. If quiet is true, output is suppressed.
func Run(event string, hooks []config.Hook, stdin []byte, quiet bool) []Result {
	var results []Result
	for _, h := range hooks {
		r := runOne(event, h, stdin, quiet)
		results = append(results, r)
	}
	return results
}

func runOne(event string, h config.Hook, stdin []byte, quiet bool) Result {
	timeout := time.Duration(h.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", h.Command)

	// Set environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PRX_EVENT=%s", event))
	for k, v := range h.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Pipe stdin
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()

	output := strings.TrimSpace(out.String())
	if !quiet && output != "" {
		fmt.Fprintf(os.Stderr, "[hook:%s] %s\n", h.Name, output)
	}

	return Result{
		Name:   h.Name,
		Output: output,
		Err:    err,
	}
}
