package sandbox

import (
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/sandbox"
)

type Service struct{}

func (Service) ValidateCommand(policy sandbox.Policy, instance sandbox.Instance, req sandbox.CommandRequest) error {
	if err := instance.CanRunCommand(); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}
	return policy.Allows(req)
}
