package gopackages

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// Platform returns the platform the go command builds for in dir, as
// "goos/goarch", such as "linux/amd64". go env reads GOOS and GOARCH from env
// and from the go env file, as the go list behind Load does with the
// process's environment, so with a nil env, which means the process's, it
// names the build the load reads. A failure is EINVALID with go's own
// message.
func Platform(ctx context.Context, dir string, env []string) (string, error) {
	const op = "gopackages.Platform"
	cmd := exec.CommandContext(ctx, "go", "env", "GOOS", "GOARCH")
	cmd.Dir, cmd.Env = dir, env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || msg == "" {
			msg = err.Error()
		}
		return "", &arch.Error{
			Op:  op,
			Err: &arch.Error{Code: arch.EINVALID, Message: "go env: " + msg},
		}
	}
	return strings.Join(strings.Fields(string(out)), "/"), nil
}
