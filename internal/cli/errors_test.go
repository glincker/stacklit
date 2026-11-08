package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/glincker/stacklit/internal/config"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, ExitOK},
		{errors.New("boom"), ExitFailure},
		{&ExitError{Code: ExitProblems, Err: errors.New("x")}, ExitProblems},
		{&config.Error{Err: errors.New("bad")}, ExitUsage},
		{fmt.Errorf("walk: %w", fs.ErrNotExist), ExitUsage},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.want {
			t.Errorf("ExitCode(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}
