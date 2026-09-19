package service

import (
	"fmt"
	"strings"
)

func RenderPrometheusStatus(status map[string]any) (string, error) {
	repo, ok := status["repo_revision"].(string)
	if !ok {
		return "", fmt.Errorf("status repo_revision must be a string")
	}
	index, ok := status["index_revision"].(string)
	if !ok {
		return "", fmt.Errorf("status index_revision must be a string")
	}
	stale, ok := status["index_stale"].(bool)
	if !ok {
		return "", fmt.Errorf("status index_stale must be a boolean")
	}
	visible, ok := status["visible_concepts"].(int)
	if !ok {
		return "", fmt.Errorf("status visible_concepts must be an integer")
	}
	backlog, ok := status["proposal_backlog"].(int)
	if !ok {
		return "", fmt.Errorf("status proposal_backlog must be an integer")
	}
	closed, ok := status["closed"].(bool)
	if !ok {
		return "", fmt.Errorf("status closed must be a boolean")
	}
	boolean := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	lines := []string{"# HELP memento_service_up Memento process health.", "# TYPE memento_service_up gauge", "memento_service_up 1", "# HELP memento_control_db_open Whether the control database connection is open.", "# TYPE memento_control_db_open gauge", fmt.Sprintf("memento_control_db_open %d", boolean(!closed)), "# HELP memento_index_stale Whether the derived index is stale relative to repo head.", "# TYPE memento_index_stale gauge", fmt.Sprintf("memento_index_stale %d", boolean(stale)), "# HELP memento_visible_concepts Authorized concept count for the local operator view.", "# TYPE memento_visible_concepts gauge", fmt.Sprintf("memento_visible_concepts %d", visible), "# HELP memento_proposal_backlog Submitted or approved proposals awaiting action.", "# TYPE memento_proposal_backlog gauge", fmt.Sprintf("memento_proposal_backlog %d", backlog), "# HELP memento_repo_revision_info Repo and index revision labels.", "# TYPE memento_repo_revision_info gauge", fmt.Sprintf("memento_repo_revision_info{repo_revision=%q,index_revision=%q} 1", repo, index), ""}
	return strings.Join(lines, "\n"), nil
}
