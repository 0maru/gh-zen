package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type actionID string

const (
	actionMoveDown             actionID = "move_down"
	actionMoveUp               actionID = "move_up"
	actionJumpTop              actionID = "jump_top"
	actionJumpBottom           actionID = "jump_bottom"
	actionFocusNextPane        actionID = "focus_next_pane"
	actionFocusPreviousPane    actionID = "focus_previous_pane"
	actionFocusPane1           actionID = "focus_pane_1"
	actionFocusPane2           actionID = "focus_pane_2"
	actionFocusPane3           actionID = "focus_pane_3"
	actionToggleHelp           actionID = "toggle_help"
	actionRefresh              actionID = "refresh"
	actionShowActions          actionID = "show_actions"
	actionShowWorkbench        actionID = "show_workbench"
	actionOpenPullRequest      actionID = "open_pr"
	actionOpenIssue            actionID = "open_issue"
	actionOpenInBrowser        actionID = "open_in_browser"
	actionCopyURL              actionID = "copy_url"
	actionCopyIssueNumber      actionID = "copy_issue_number"
	actionCopyWorktreePath     actionID = "copy_worktree_path"
	actionOpenWorkflowRun      actionID = "open_workflow_run"
	actionCopyWorkflowRunID    actionID = "copy_workflow_run_id"
	actionFetchWorkflowRunLogs actionID = "fetch_workflow_run_logs"
	actionFilterStatus         actionID = "filter_status"
	actionFilterConclusion     actionID = "filter_conclusion"
	actionFilterBranch         actionID = "filter_branch"
	actionFilterWorkflow       actionID = "filter_workflow"
	actionFilterEvent          actionID = "filter_event"
	actionFilterActor          actionID = "filter_actor"
	actionClearFilters         actionID = "clear_filters"
	actionCycleIssueState      actionID = "cycle_issue_state"
	actionCycleIssueAssignee   actionID = "cycle_issue_assignee"
	actionCycleIssueLabel      actionID = "cycle_issue_label"
	actionCycleIssueMilestone  actionID = "cycle_issue_milestone"
	actionStartIssueSearch     actionID = "start_issue_search"
	actionClearIssueFilters    actionID = "clear_issue_filters"
	actionBackToWorkbench      actionID = "back_to_workbench"
	actionQuit                 actionID = "quit"
)

type actionBinding struct {
	id      actionID
	binding key.Binding
}

type workbenchKeyMap struct {
	MoveDown             key.Binding
	MoveUp               key.Binding
	JumpTop              key.Binding
	JumpBottom           key.Binding
	FocusNextPane        key.Binding
	FocusPreviousPane    key.Binding
	FocusPane1           key.Binding
	FocusPane2           key.Binding
	FocusPane3           key.Binding
	ToggleHelp           key.Binding
	Refresh              key.Binding
	ShowActions          key.Binding
	ShowWorkbench        key.Binding
	OpenPullRequest      key.Binding
	OpenIssue            key.Binding
	OpenInBrowser        key.Binding
	CopyURL              key.Binding
	CopyIssueNumber      key.Binding
	CopyWorktreePath     key.Binding
	OpenWorkflowRun      key.Binding
	CopyWorkflowRunID    key.Binding
	FetchWorkflowRunLogs key.Binding
	FilterStatus         key.Binding
	FilterConclusion     key.Binding
	FilterBranch         key.Binding
	FilterWorkflow       key.Binding
	FilterEvent          key.Binding
	FilterActor          key.Binding
	ClearFilters         key.Binding
	CycleIssueState      key.Binding
	CycleIssueAssignee   key.Binding
	CycleIssueLabel      key.Binding
	CycleIssueMilestone  key.Binding
	StartIssueSearch     key.Binding
	ClearIssueFilters    key.Binding
	BackToWorkbench      key.Binding
	ForceQuit            key.Binding
	Quit                 key.Binding
}

type contextualHelpKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k contextualHelpKeyMap) ShortHelp() []key.Binding {
	return k.short
}

func (k contextualHelpKeyMap) FullHelp() [][]key.Binding {
	return k.full
}

func DefaultKeyMap() workbenchKeyMap {
	return workbenchKeyMap{
		MoveDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "down"),
		),
		MoveUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "up"),
		),
		JumpTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		JumpBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		FocusNextPane: key.NewBinding(
			key.WithKeys("l", "tab"),
			key.WithHelp("l/tab", "next pane"),
		),
		FocusPreviousPane: key.NewBinding(
			key.WithKeys("h", "shift+tab"),
			key.WithHelp("h/S-tab", "prev pane"),
		),
		FocusPane1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("[1]", "pane"),
		),
		FocusPane2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("[2]", "pane"),
		),
		FocusPane3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("[3]", "pane"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ShowActions: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "actions"),
		),
		ShowWorkbench: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "workbench"),
		),
		OpenPullRequest: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "open PR"),
		),
		OpenIssue: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "issues"),
		),
		OpenInBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		CopyURL: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy URL"),
		),
		CopyIssueNumber: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "copy #"),
		),
		CopyWorktreePath: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy path"),
		),
		OpenWorkflowRun: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open run"),
		),
		CopyWorkflowRunID: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy run ID"),
		),
		FetchWorkflowRunLogs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "failed logs"),
		),
		FilterStatus: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "status"),
		),
		FilterConclusion: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "conclusion"),
		),
		FilterBranch: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "branch"),
		),
		FilterWorkflow: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "workflow"),
		),
		FilterEvent: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "event"),
		),
		FilterActor: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "actor"),
		),
		ClearFilters: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "clear filters"),
		),
		CycleIssueState: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "state"),
		),
		CycleIssueAssignee: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "assignee"),
		),
		CycleIssueLabel: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "label"),
		),
		CycleIssueMilestone: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "milestone"),
		),
		StartIssueSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ClearIssueFilters: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "clear filters"),
		),
		BackToWorkbench: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/esc", "back"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k workbenchKeyMap) actionBindings(mode appMode) []actionBinding {
	common := []actionBinding{
		{id: actionQuit, binding: k.Quit},
		{id: actionToggleHelp, binding: k.ToggleHelp},
		{id: actionFocusNextPane, binding: k.FocusNextPane},
		{id: actionFocusPreviousPane, binding: k.FocusPreviousPane},
		{id: actionFocusPane1, binding: k.FocusPane1},
		{id: actionFocusPane2, binding: k.FocusPane2},
		{id: actionFocusPane3, binding: k.FocusPane3},
		{id: actionMoveDown, binding: k.MoveDown},
		{id: actionMoveUp, binding: k.MoveUp},
		{id: actionJumpTop, binding: k.JumpTop},
		{id: actionJumpBottom, binding: k.JumpBottom},
		{id: actionRefresh, binding: k.Refresh},
	}
	if mode == modeActions {
		return append(common,
			actionBinding{id: actionShowWorkbench, binding: k.ShowWorkbench},
			actionBinding{id: actionOpenWorkflowRun, binding: k.OpenWorkflowRun},
			actionBinding{id: actionCopyURL, binding: k.CopyURL},
			actionBinding{id: actionCopyWorkflowRunID, binding: k.CopyWorkflowRunID},
			actionBinding{id: actionFetchWorkflowRunLogs, binding: k.FetchWorkflowRunLogs},
			actionBinding{id: actionFilterStatus, binding: k.FilterStatus},
			actionBinding{id: actionFilterConclusion, binding: k.FilterConclusion},
			actionBinding{id: actionFilterBranch, binding: k.FilterBranch},
			actionBinding{id: actionFilterWorkflow, binding: k.FilterWorkflow},
			actionBinding{id: actionFilterEvent, binding: k.FilterEvent},
			actionBinding{id: actionFilterActor, binding: k.FilterActor},
			actionBinding{id: actionClearFilters, binding: k.ClearFilters},
		)
	}
	return append(common,
		actionBinding{id: actionShowActions, binding: k.ShowActions},
		actionBinding{id: actionOpenPullRequest, binding: k.OpenPullRequest},
		actionBinding{id: actionOpenIssue, binding: k.OpenIssue},
		actionBinding{id: actionOpenInBrowser, binding: k.OpenInBrowser},
		actionBinding{id: actionCopyURL, binding: k.CopyURL},
		actionBinding{id: actionCopyWorktreePath, binding: k.CopyWorktreePath},
	)
}

func (k workbenchKeyMap) issueActionBindings() []actionBinding {
	return []actionBinding{
		{id: actionBackToWorkbench, binding: k.BackToWorkbench},
		{id: actionQuit, binding: k.ForceQuit},
		{id: actionToggleHelp, binding: k.ToggleHelp},
		{id: actionFocusNextPane, binding: k.FocusNextPane},
		{id: actionFocusPreviousPane, binding: k.FocusPreviousPane},
		{id: actionFocusPane1, binding: k.FocusPane1},
		{id: actionFocusPane2, binding: k.FocusPane2},
		{id: actionMoveDown, binding: k.MoveDown},
		{id: actionMoveUp, binding: k.MoveUp},
		{id: actionJumpTop, binding: k.JumpTop},
		{id: actionJumpBottom, binding: k.JumpBottom},
		{id: actionRefresh, binding: k.Refresh},
		{id: actionOpenInBrowser, binding: k.OpenInBrowser},
		{id: actionCopyURL, binding: k.CopyURL},
		{id: actionCopyIssueNumber, binding: k.CopyIssueNumber},
		{id: actionCycleIssueState, binding: k.CycleIssueState},
		{id: actionCycleIssueAssignee, binding: k.CycleIssueAssignee},
		{id: actionCycleIssueLabel, binding: k.CycleIssueLabel},
		{id: actionCycleIssueMilestone, binding: k.CycleIssueMilestone},
		{id: actionStartIssueSearch, binding: k.StartIssueSearch},
		{id: actionClearIssueFilters, binding: k.ClearIssueFilters},
	}
}

func (k workbenchKeyMap) contextualHelp(screen appScreen, mode appMode, focus paneFocus, visiblePanes []paneFocus) contextualHelpKeyMap {
	if screen == screenIssues {
		return k.issueContextualHelp(focus, visiblePanes)
	}

	paneNumbers := k.visiblePaneBinding(visiblePanes)
	paneKeys := combinedBinding("pane", k.FocusPreviousPane, k.FocusNextPane)
	system := []key.Binding{k.ToggleHelp, k.Quit}
	actions := []key.Binding{k.ShowActions, k.OpenPullRequest, k.OpenIssue, k.OpenInBrowser, k.CopyURL, k.CopyWorktreePath, k.Refresh}
	if mode == modeActions {
		actions = []key.Binding{k.ShowWorkbench, k.OpenWorkflowRun, k.CopyURL, k.CopyWorkflowRunID, k.FetchWorkflowRunLogs, k.Refresh}
		system = []key.Binding{k.ToggleHelp, k.Quit}
	}
	panes := []key.Binding{k.FocusPreviousPane, k.FocusNextPane, paneNumbers}

	short := []key.Binding{paneNumbers, paneKeys, k.ToggleHelp, k.Quit}
	full := [][]key.Binding{panes, actions, system}
	if mode == modeActions {
		filters := []key.Binding{k.FilterStatus, k.FilterConclusion, k.FilterBranch, k.FilterWorkflow, k.FilterEvent, k.FilterActor, k.ClearFilters}
		full = [][]key.Binding{panes, actions, filters, system}
	}

	if focus == paneRepositories || focus == paneWorkItems {
		move := combinedBinding("move", k.MoveDown, k.MoveUp)
		jump := combinedBinding("jump", k.JumpTop, k.JumpBottom)
		short = append([]key.Binding{move, jump}, short...)
		full = append([][]key.Binding{
			{k.MoveUp, k.MoveDown, k.JumpTop, k.JumpBottom},
		}, full...)
	}

	return contextualHelpKeyMap{short: short, full: full}
}

func (k workbenchKeyMap) issueContextualHelp(focus paneFocus, visiblePanes []paneFocus) contextualHelpKeyMap {
	paneNumbers := k.visiblePaneBinding(visiblePanes)
	paneKeys := combinedBinding("pane", k.FocusPreviousPane, k.FocusNextPane)
	system := []key.Binding{k.BackToWorkbench, k.ForceQuit, k.ToggleHelp}
	actions := []key.Binding{k.OpenInBrowser, k.CopyURL, k.CopyIssueNumber, k.Refresh}
	filters := []key.Binding{k.CycleIssueState, k.CycleIssueAssignee, k.CycleIssueLabel, k.CycleIssueMilestone, k.StartIssueSearch, k.ClearIssueFilters}
	panes := []key.Binding{k.FocusPreviousPane, k.FocusNextPane, paneNumbers}

	short := []key.Binding{paneNumbers, paneKeys, k.ToggleHelp, k.BackToWorkbench}
	full := [][]key.Binding{panes, actions, filters, system}

	if focus == paneWorkItems || focus == panePreview {
		move := combinedBinding("move", k.MoveDown, k.MoveUp)
		jump := combinedBinding("jump", k.JumpTop, k.JumpBottom)
		short = append([]key.Binding{move, jump}, short...)
		full = append([][]key.Binding{
			{k.MoveUp, k.MoveDown, k.JumpTop, k.JumpBottom},
		}, full...)
	}

	return contextualHelpKeyMap{short: short, full: full}
}

func (k workbenchKeyMap) visiblePaneBinding(visiblePanes []paneFocus) key.Binding {
	paneBindings := []key.Binding{k.FocusPane1, k.FocusPane2, k.FocusPane3}
	visibleCount := min(len(visiblePanes), len(paneBindings))
	keys := make([]string, 0, visibleCount)
	helpKeys := make([]string, 0, visibleCount)

	for i := range visibleCount {
		binding := paneBindings[i]
		keys = append(keys, binding.Keys()...)
		helpKeys = append(helpKeys, binding.Help().Key)
	}

	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(strings.Join(helpKeys, "/"), "pane"),
	)
}

func combinedBinding(description string, bindings ...key.Binding) key.Binding {
	keys := []string{}
	helpKeys := make([]string, 0, len(bindings))

	for _, binding := range bindings {
		keys = append(keys, binding.Keys()...)
		if helpKey := firstHelpKey(binding); helpKey != "" {
			helpKeys = append(helpKeys, helpKey)
		}
	}

	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(strings.Join(helpKeys, "/"), description),
	)
}

func firstHelpKey(binding key.Binding) string {
	helpKey := binding.Help().Key
	if helpKey == "" {
		keys := binding.Keys()
		if len(keys) == 0 {
			return ""
		}
		return keys[0]
	}
	return strings.Split(helpKey, "/")[0]
}
