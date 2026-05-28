package sandbox

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	app "github.com/BruceLoveDecimal/AgentHub/internal/application/sandboxes"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

type LocalExecutor struct {
	Root string
}

func (e LocalExecutor) Execute(ctx context.Context, req sandbox.CommandRequest) (app.CommandResult, error) {
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	if e.Root != "" {
		if req.CWD != "" {
			cmd.Dir = filepath.Join(e.Root, filepath.Clean(req.CWD))
		} else {
			cmd.Dir = e.Root
		}
	}
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdout, err := cmd.Output()
	stderr := ""
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			stderr = err.Error()
		}
	}
	if ctx.Err() != nil {
		stderr = ctx.Err().Error()
		if exitCode == 0 {
			exitCode = 124
		}
	}
	return app.CommandResult{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   stderr + exitCodeSuffix(exitCode),
	}, nil
}

func exitCodeSuffix(code int) string {
	if code == 0 {
		return ""
	}
	return "\nexit_code=" + strconv.Itoa(code)
}
