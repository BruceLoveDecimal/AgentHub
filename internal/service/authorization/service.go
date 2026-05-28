package authorization

import (
	"errors"
	"time"

	"github.com/BruceLoveDecimal/AgentHub/internal/domain/audit"
	"github.com/BruceLoveDecimal/AgentHub/internal/domain/capability"
)

type Request struct {
	ActorChain audit.ActorChain
	Principal  capability.Principal
	Capability capability.Name
	Resource   capability.Resource
	At         time.Time
}

type Decision struct {
	Allowed          bool
	RequiresApproval bool
	Reason           string
	MatchedGrantID   string
}

type Service struct{}

func (s Service) Authorize(req Request, grants []capability.Grant) Decision {
	if req.At.IsZero() {
		req.At = time.Now().UTC()
	}
	if err := req.Validate(); err != nil {
		return deny("invalid_request: " + err.Error())
	}

	var allow *capability.Grant
	for i := range grants {
		grant := &grants[i]
		if !grant.Matches(req.Principal, req.Capability, req.Resource, req.At) {
			continue
		}
		if grant.Effect == capability.EffectDeny {
			return Decision{
				Allowed:        false,
				Reason:         "explicit_deny",
				MatchedGrantID: grant.ID,
			}
		}
		if grant.Effect == capability.EffectAllow && allow == nil {
			allow = grant
		}
	}

	if allow == nil {
		return deny("no_matching_grant")
	}
	if allow.Conditions.RequiresApproval {
		return Decision{
			Allowed:          false,
			RequiresApproval: true,
			Reason:           "approval_required",
			MatchedGrantID:   allow.ID,
		}
	}
	return Decision{
		Allowed:        true,
		Reason:         "allowed",
		MatchedGrantID: allow.ID,
	}
}

func (r Request) Validate() error {
	if err := r.ActorChain.Validate(); err != nil {
		return err
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	if r.Capability == "" {
		return errors.New("capability is required")
	}
	if err := r.Resource.Validate(); err != nil {
		return err
	}
	if r.Principal.Kind == capability.PrincipalAgent && r.ActorChain.AgentID() != r.Principal.ID {
		return errors.New("agent principal must match actor chain")
	}
	return nil
}

func deny(reason string) Decision {
	return Decision{
		Allowed: false,
		Reason:  reason,
	}
}
