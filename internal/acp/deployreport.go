package acp

import (
	"fmt"
	"os"

	"github.com/artipop/xciii/internal/dokku"
)

// What a finished deploy session says on the card. "The agent stopped talking"
// is not "the branch is live", so the answer comes from the record the dokku
// MCP server left behind rather than from the session's status.

// applyDeployOutcome reads the recorded deploy result and says so on the card
// when the agent's own summary would have been misleading.
func (m *Manager) applyDeployOutcome(s *Session) {
	if s.Deploy == nil {
		return
	}
	if s.Artifacts == "" {
		// Nothing was recorded, so the session status is all there is.
		return
	}
	res, err := dokku.ReadOutcome(s.Artifacts)
	switch {
	case os.IsNotExist(err):
		m.comment(s, "Деплой не подтверждён: инструмент `deploy_branch` ни разу не отработал, так что ветка не опубликована.")
	case err != nil:
		m.log.Warn("acp: cannot read deploy outcome", "session", s.ID, "err", err)
	case !res.OK:
		m.comment(s, fmt.Sprintf("Деплой ветки `%s` не удался: %s", res.Branch, res.Error))
	}
}
