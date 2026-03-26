package workflow

import (
	"os"
	"strings"
	"sync"
)

type ClaudeSession struct {
	PID        int
	SessionID  string
	CWD        string
	ProjectKey string
}

type activeWorkflow struct {
	paths Paths
	state *State
}

type runtimeSessionHints struct {
	SessionID  string
	SessionPID int
}

var (
	runtimeSessionHintMu     sync.RWMutex
	runtimeSessionHintsState runtimeSessionHints
)

func SetRuntimeSessionHints(sessionID string, sessionPID int) {
	runtimeSessionHintMu.Lock()
	runtimeSessionHintsState = runtimeSessionHints{
		SessionID:  strings.TrimSpace(sessionID),
		SessionPID: sessionPID,
	}
	runtimeSessionHintMu.Unlock()
}

func runtimeSessionHintsValue() runtimeSessionHints {
	runtimeSessionHintMu.RLock()
	defer runtimeSessionHintMu.RUnlock()
	return runtimeSessionHintsState
}

func ResolveCurrentClaudeSession() (*ClaudeSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return resolveCurrentClaudeSession(home, runtimeSessionHintsValue(), []int{os.Getppid(), os.Getpid()})
}

func resolveCurrentClaudeSession(home string, hints runtimeSessionHints, fallbackPIDs []int) (*ClaudeSession, error) {
	session, hadExplicitHints, err := resolveOwningClaudeSession(home, hints, fallbackPIDs)
	if err != nil {
		return nil, err
	}
	if session == nil {
		if hadExplicitHints {
			return nil, ErrExplicitRuntimeSessionHints
		}
		return nil, nil
	}
	return &ClaudeSession{
		PID:        session.PID,
		SessionID:  strings.TrimSpace(session.SessionID),
		CWD:        strings.TrimSpace(session.CWD),
		ProjectKey: encodeClaudeProjectKey(session.CWD),
	}, nil
}

func resolveWorkflow(explicitID string, claim bool) (Paths, *State, error) {
	explicitID = strings.TrimSpace(explicitID)
	if explicitID != "" {
		p := PathsFor(explicitID)
		s, err := Load(p.WorkflowFile)
		if err != nil || s == nil {
			return p, s, err
		}
		if claim {
			session, err := ResolveCurrentClaudeSession()
			if err != nil {
				return Paths{}, nil, err
			}
			if shouldClaimWorkflow(s, session) {
				BindOwner(s, session)
				Touch(s)
				if err := s.Save(p.WorkflowFile); err != nil {
					return Paths{}, nil, err
				}
			}
		}
		return p, s, nil
	}

	active, err := discoverActiveWorkflows()
	if err != nil {
		return Paths{}, nil, err
	}
	if len(active) == 0 {
		return Paths{}, nil, nil
	}

	session, err := ResolveCurrentClaudeSession()
	if err != nil {
		return Paths{}, nil, err
	}
	if session == nil {
		if len(active) == 1 {
			return active[0].paths, active[0].state, nil
		}
		return Paths{}, nil, &ActiveConflictError{IDs: activeWorkflowIDs(active)}
	}

	owned := filterWorkflows(active, func(w activeWorkflow) bool {
		return strings.TrimSpace(w.state.OwnerSessionID) == strings.TrimSpace(session.SessionID)
	})
	if len(owned) == 1 {
		return owned[0].paths, owned[0].state, nil
	}
	if len(owned) > 1 {
		return Paths{}, nil, &ActiveConflictError{IDs: activeWorkflowIDs(owned)}
	}

	claimable := filterWorkflows(active, func(w activeWorkflow) bool {
		return shouldClaimWorkflow(w.state, session)
	})
	if claim && len(claimable) == 1 {
		BindOwner(claimable[0].state, session)
		Touch(claimable[0].state)
		if err := claimable[0].state.Save(claimable[0].paths.WorkflowFile); err != nil {
			return Paths{}, nil, err
		}
		return claimable[0].paths, claimable[0].state, nil
	}
	if len(claimable) > 1 {
		return Paths{}, nil, &ActiveConflictError{IDs: activeWorkflowIDs(claimable)}
	}

	return Paths{}, nil, nil
}

func discoverActiveWorkflows() ([]activeWorkflow, error) {
	entries, err := os.ReadDir(WorkflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var active []activeWorkflow
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths := PathsFor(entry.Name())
		s, err := Load(paths.WorkflowFile)
		if err != nil || s == nil {
			continue
		}
		if strings.TrimSpace(s.Status) != "active" {
			continue
		}
		active = append(active, activeWorkflow{paths: paths, state: s})
	}
	return active, nil
}

func filterWorkflows(in []activeWorkflow, keep func(activeWorkflow) bool) []activeWorkflow {
	var out []activeWorkflow
	for _, item := range in {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func activeWorkflowIDs(in []activeWorkflow) []string {
	ids := make([]string, 0, len(in))
	for _, item := range in {
		ids = append(ids, item.state.ID)
	}
	return ids
}

func shouldClaimWorkflow(s *State, session *ClaudeSession) bool {
	if s == nil || session == nil {
		return false
	}
	if strings.TrimSpace(s.Status) != "active" {
		return false
	}
	if strings.TrimSpace(s.OwnerSessionID) != "" {
		return false
	}
	return true
}
