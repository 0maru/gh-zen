package app

import (
	"fmt"
	"strings"

	"github.com/0maru/gh-zen/internal/workbench"
)

type actionsFilter struct {
	Status     string
	Conclusion string
	Branch     string
	Workflow   string
	Event      string
	Actor      string
}

func (f actionsFilter) active() bool {
	return f.Status != "" ||
		f.Conclusion != "" ||
		f.Branch != "" ||
		f.Workflow != "" ||
		f.Event != "" ||
		f.Actor != ""
}

func (f actionsFilter) describe() string {
	parts := []string{}
	if f.Status != "" {
		parts = append(parts, "status="+f.Status)
	}
	if f.Conclusion != "" {
		parts = append(parts, "conclusion="+f.Conclusion)
	}
	if f.Branch != "" {
		parts = append(parts, "branch="+f.Branch)
	}
	if f.Workflow != "" {
		parts = append(parts, "workflow="+f.Workflow)
	}
	if f.Event != "" {
		parts = append(parts, "event="+f.Event)
	}
	if f.Actor != "" {
		parts = append(parts, "actor="+f.Actor)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func filterWorkflowRuns(runs []workbench.WorkflowRunRef, filter actionsFilter) []workbench.WorkflowRunRef {
	out := make([]workbench.WorkflowRunRef, 0, len(runs))
	for _, run := range runs {
		if matchesActionsFilter(run, filter) {
			out = append(out, run)
		}
	}
	return out
}

func matchesActionsFilter(run workbench.WorkflowRunRef, filter actionsFilter) bool {
	return filterStringMatches(filter.Status, run.Status) &&
		filterStringMatches(filter.Conclusion, run.Conclusion) &&
		filterStringMatches(filter.Branch, run.Branch) &&
		filterStringMatches(filter.Workflow, run.WorkflowName) &&
		filterStringMatches(filter.Event, run.Event) &&
		filterStringMatches(filter.Actor, run.Actor)
}

func filterStringMatches(filter string, value string) bool {
	return filter == "" || strings.EqualFold(filter, value)
}

type actionsFilterField int

const (
	actionsFilterFieldStatus actionsFilterField = iota
	actionsFilterFieldConclusion
	actionsFilterFieldBranch
	actionsFilterFieldWorkflow
	actionsFilterFieldEvent
	actionsFilterFieldActor
)

func (m *model) cycleActionsFilter(field actionsFilterField) {
	runs := m.actions.runs
	switch field {
	case actionsFilterFieldStatus:
		m.actions.filter.Status = nextActionsFilterValue(m.actions.filter.Status, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.Status
		}))
	case actionsFilterFieldConclusion:
		m.actions.filter.Conclusion = nextActionsFilterValue(m.actions.filter.Conclusion, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.Conclusion
		}))
	case actionsFilterFieldBranch:
		m.actions.filter.Branch = nextActionsFilterValue(m.actions.filter.Branch, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.Branch
		}))
	case actionsFilterFieldWorkflow:
		m.actions.filter.Workflow = nextActionsFilterValue(m.actions.filter.Workflow, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.WorkflowName
		}))
	case actionsFilterFieldEvent:
		m.actions.filter.Event = nextActionsFilterValue(m.actions.filter.Event, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.Event
		}))
	case actionsFilterFieldActor:
		m.actions.filter.Actor = nextActionsFilterValue(m.actions.filter.Actor, actionFilterValues(runs, func(run workbench.WorkflowRunRef) string {
			return run.Actor
		}))
	}
	m.actions.selectedRun = 0
	m.statusMessage = fmt.Sprintf("Actions filters: %s", m.actions.filter.describe())
}

func actionFilterValues(runs []workbench.WorkflowRunRef, value func(workbench.WorkflowRunRef) string) []string {
	seen := map[string]bool{}
	values := []string{}
	for _, run := range runs {
		v := strings.TrimSpace(value(run))
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, v)
	}
	return values
}

func nextActionsFilterValue(current string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	if current == "" {
		return values[0]
	}
	for i, value := range values {
		if strings.EqualFold(value, current) {
			if i == len(values)-1 {
				return ""
			}
			return values[i+1]
		}
	}
	return values[0]
}
