package skills

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RunTimeout is how long a skill command may run.
const RunTimeout = 2 * time.Minute

// MaxOutput is how much of a command's output is kept.
const MaxOutput = 256 << 10

// RunResult is what a skill command printed.
type RunResult struct {
	Dir      string
	Command  string
	Exit     int
	Stdout   string
	Stderr   string
	TimedOut bool
}

// Run runs command in the skill's folder, with the system shell, so a
// skill can start bash, python, node or adb. The command is the skill's,
// run as the person who started Pinky.
func (l *Library) Run(ctx context.Context, id, command string) (RunResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return RunResult{}, errors.New("skills: empty command")
	}
	dir, err := l.Dir(id)
	if err != nil {
		return RunResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, RunTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &limitWriter{w: &out, n: MaxOutput}
	cmd.Stderr = &limitWriter{w: &errb, n: MaxOutput}
	err = cmd.Run()
	res := RunResult{Dir: dir, Command: command, Stdout: out.String(), Stderr: errb.String()}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		res.Exit = -1
		return res, fmt.Errorf("skills: command timed out after %s", RunTimeout)
	}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.Exit = ee.ExitCode()
			return res, nil
		}
		return res, err
	}
	return res, nil
}

type limitWriter struct {
	w io.Writer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	take := p
	if len(take) > l.n {
		take = take[:l.n]
	}
	n, err := l.w.Write(take)
	l.n -= n
	if err != nil {
		return n, err
	}
	return len(p), nil
}
