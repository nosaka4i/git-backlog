package cmd

import (
	"fmt"

	"github.com/nosaka4i/git-backlog/internal/gitx"
	"github.com/nosaka4i/git-backlog/internal/store"
)

// resolveAgentIdentity reads the backlog.agent.name/backlog.agent.email
// git config for --as-agent. Returns nil (use the ambient git identity)
// when asAgent is false. See docs/design/git-backlog.md's "Agent identity"
// section for why this exists and how to configure it.
func resolveAgentIdentity(asAgent bool) (*store.Identity, error) {
	if !asAgent {
		return nil, nil
	}
	name, ok := gitx.Config("backlog.agent.name")
	if !ok || name == "" {
		return nil, fmt.Errorf("--as-agent requires backlog.agent.name to be configured; run: git config backlog.agent.name \"<name>\"")
	}
	email, ok := gitx.Config("backlog.agent.email")
	if !ok || email == "" {
		return nil, fmt.Errorf("--as-agent requires backlog.agent.email to be configured; run: git config backlog.agent.email \"<email>\"")
	}
	return &store.Identity{Name: name, Email: email}, nil
}

const asAgentFlagUsage = "record this operation as the agent identity from backlog.agent.name/backlog.agent.email, instead of the ambient git identity"
