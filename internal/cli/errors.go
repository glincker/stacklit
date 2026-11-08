package cli

import (
	"errors"
	"io/fs"

	"github.com/glincker/stacklit/internal/config"
)

// Exit codes. Scripts and hooks can rely on these staying stable.
const (
	ExitOK       = 0
	ExitFailure  = 1 // unexpected internal error
	ExitUsage    = 2 // bad flags, bad config, missing path
	ExitProblems = 3 // doctor found problems
)

// ExitError carries a specific exit code out of a command.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode maps an error returned by Execute to a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	var ce *config.Error
	if errors.As(err, &ce) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return ExitUsage
	}
	return ExitFailure
}
