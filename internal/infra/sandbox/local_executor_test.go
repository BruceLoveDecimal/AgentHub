package sandbox

import (
	"context"
	"testing"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

func TestLocalExecutorRunsCommand(t *testing.T) {
	result, err := LocalExecutor{}.Execute(context.Background(), sandbox.CommandRequest{
		Command: "printf",
		Args:    []string{"hello"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.ExitCode != 0 || result.Stdout != "hello" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
