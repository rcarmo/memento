package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var graphitePrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)

type operationalMetrics struct {
	repo, index      string
	stale            bool
	visible, backlog int
	closed           bool
}

func parseOperationalMetrics(status map[string]any) (operationalMetrics, error) {
	var out operationalMetrics
	var ok bool
	if out.repo, ok = status["repo_revision"].(string); !ok {
		return out, fmt.Errorf("status repo_revision must be a string")
	}
	if out.index, ok = status["index_revision"].(string); !ok {
		return out, fmt.Errorf("status index_revision must be a string")
	}
	if out.stale, ok = status["index_stale"].(bool); !ok {
		return out, fmt.Errorf("status index_stale must be a boolean")
	}
	if out.visible, ok = status["visible_concepts"].(int); !ok {
		return out, fmt.Errorf("status visible_concepts must be an integer")
	}
	if out.backlog, ok = status["proposal_backlog"].(int); !ok {
		return out, fmt.Errorf("status proposal_backlog must be an integer")
	}
	if out.closed, ok = status["closed"].(bool); !ok {
		return out, fmt.Errorf("status closed must be a boolean")
	}
	return out, nil
}
func RenderPrometheusStatus(status map[string]any) (string, error) {
	metrics, err := parseOperationalMetrics(status)
	if err != nil {
		return "", err
	}
	repo, index, stale, visible, backlog, closed := metrics.repo, metrics.index, metrics.stale, metrics.visible, metrics.backlog, metrics.closed
	boolean := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	lines := []string{"# HELP memento_service_up Memento process health.", "# TYPE memento_service_up gauge", "memento_service_up 1", "# HELP memento_control_db_open Whether the control database connection is open.", "# TYPE memento_control_db_open gauge", fmt.Sprintf("memento_control_db_open %d", boolean(!closed)), "# HELP memento_index_stale Whether the derived index is stale relative to repo head.", "# TYPE memento_index_stale gauge", fmt.Sprintf("memento_index_stale %d", boolean(stale)), "# HELP memento_visible_concepts Authorized concept count for the local operator view.", "# TYPE memento_visible_concepts gauge", fmt.Sprintf("memento_visible_concepts %d", visible), "# HELP memento_proposal_backlog Submitted or approved proposals awaiting action.", "# TYPE memento_proposal_backlog gauge", fmt.Sprintf("memento_proposal_backlog %d", backlog), "# HELP memento_repo_revision_info Repo and index revision labels.", "# TYPE memento_repo_revision_info gauge", fmt.Sprintf("memento_repo_revision_info{repo_revision=%q,index_revision=%q} 1", repo, index), ""}
	return strings.Join(lines, "\n"), nil
}
func RenderGraphiteStatus(status map[string]any, prefix string, at time.Time) (string, error) {
	if !graphitePrefixPattern.MatchString(prefix) {
		return "", fmt.Errorf("graphite metric prefix must contain dot-separated alphanumeric, underscore, or hyphen components")
	}
	metrics, err := parseOperationalMetrics(status)
	if err != nil {
		return "", err
	}
	boolean := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	stamp := at.Unix()
	lines := []string{fmt.Sprintf("%s.service_up 1 %d", prefix, stamp), fmt.Sprintf("%s.control_db_open %d %d", prefix, boolean(!metrics.closed), stamp), fmt.Sprintf("%s.index_stale %d %d", prefix, boolean(metrics.stale), stamp), fmt.Sprintf("%s.visible_concepts %d %d", prefix, metrics.visible, stamp), fmt.Sprintf("%s.proposal_backlog %d %d", prefix, metrics.backlog, stamp), ""}
	return strings.Join(lines, "\n"), nil
}
