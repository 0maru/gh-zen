package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type actionID string

const (
	actionMoveDown              actionID = "move_down"
	actionMoveUp                actionID = "move_up"
	actionJumpTop               actionID = "jump_top"
	actionJumpBottom            actionID = "jump_bottom"
	actionFocusNextPane         actionID = "focus_next_pane"
	actionFocusPreviousPane     actionID = "focus_previous_pane"
	actionFocusPane1            actionID = "focus_pane_1"
	actionFocusPane2            actionID = "focus_pane_2"
	actionFocusPane3            actionID = "focus_pane_3"
	actionToggleHelp            actionID = "toggle_help"
	actionRefresh               actionID = "refresh"
	actionOpenPullRequest       actionID = "open_pr"
	actionOpenSelected          actionID = "open_selected"
	actionOpenIssue             actionID = "open_issue"
	actionCopyURL               actionID = "copy_url"
	actionCopyWorktreePath      actionID = "copy_worktree_path"
	actionCopyPullRequestNumber actionID = "copy_pr_number"
	actionCopyPullRequestHead   actionID = "copy_pr_head"
	actionShowPullRequests      actionID = "show_pull_requests"
	actionShowWorkbench         actionID = "show_workbench"
	actionSearchPullRequests    actionID = "search_prs"
	actionFilterPullRequests    actionID = "filter_prs"
	actionQuit                  actionID = "quit"
)

type actionBinding struct {
	id      actionID
	binding key.Binding
}

type workbenchKeyMap struct {
	MoveDown              key.Binding
	MoveUp                key.Binding
	JumpTop               key.Binding
	JumpBottom            key.Binding
	FocusNextPane         key.Binding
	FocusPreviousPane     key.Binding
	FocusPane1            key.Binding
	FocusPane2            key.Binding
	FocusPane3            key.Binding
	ToggleHelp            key.Binding
	Refresh               key.Binding
	OpenPullRequest       key.Binding
	OpenSelected          key.Binding
	OpenIssue             key.Binding
	CopyURL               key.Binding
	CopyWorktreePath      key.Binding
	CopyPullRequestNumber key.Binding
	CopyPullRequestHead   key.Binding
	ShowPullRequests      key.Binding
	ShowWorkbench         key.Binding
	SearchPullRequests    key.Binding
	FilterPullRequests    key.Binding
	Quit                  key.Binding
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
		OpenPullRequest: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "open PR"),
		),
		OpenSelected: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		OpenIssue: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "open issue"),
		),
		CopyURL: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy URL"),
		),
		CopyWorktreePath: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy path"),
		),
		CopyPullRequestNumber: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy #"),
		),
		CopyPullRequestHead: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "copy head"),
		),
		ShowPullRequests: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "PR view"),
		),
		ShowWorkbench: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "workbench"),
		),
		SearchPullRequests: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		FilterPullRequests: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k workbenchKeyMap) actionBindings(view appView) []actionBinding {
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
	if view == appViewPullRequests {
		return append(common,
			actionBinding{id: actionOpenSelected, binding: k.OpenSelected},
			actionBinding{id: actionCopyURL, binding: k.CopyURL},
			actionBinding{id: actionCopyPullRequestNumber, binding: k.CopyPullRequestNumber},
			actionBinding{id: actionCopyPullRequestHead, binding: k.CopyPullRequestHead},
			actionBinding{id: actionShowWorkbench, binding: k.ShowWorkbench},
			actionBinding{id: actionSearchPullRequests, binding: k.SearchPullRequests},
			actionBinding{id: actionFilterPullRequests, binding: k.FilterPullRequests},
		)
	}
	return append(common,
		actionBinding{id: actionOpenPullRequest, binding: k.OpenPullRequest},
		actionBinding{id: actionOpenIssue, binding: k.OpenIssue},
		actionBinding{id: actionCopyURL, binding: k.CopyURL},
		actionBinding{id: actionCopyWorktreePath, binding: k.CopyWorktreePath},
		actionBinding{id: actionShowPullRequests, binding: k.ShowPullRequests},
	)
}

func (k workbenchKeyMap) contextualHelp(view appView, focus paneFocus, visiblePanes []paneFocus) contextualHelpKeyMap {
	paneNumbers := k.visiblePaneBinding(visiblePanes)
	paneKeys := combinedBinding("pane", k.FocusPreviousPane, k.FocusNextPane)
	system := []key.Binding{k.ToggleHelp, k.Quit}
	actions := []key.Binding{k.OpenPullRequest, k.OpenIssue, k.CopyURL, k.CopyWorktreePath, k.ShowPullRequests, k.Refresh}
	if view == appViewPullRequests {
		actions = []key.Binding{k.OpenSelected, k.CopyURL, k.CopyPullRequestNumber, k.CopyPullRequestHead, k.SearchPullRequests, k.FilterPullRequests, k.ShowWorkbench, k.Refresh}
	}
	panes := []key.Binding{k.FocusPreviousPane, k.FocusNextPane, paneNumbers}

	short := []key.Binding{paneNumbers, paneKeys, k.ToggleHelp, k.Quit}
	full := [][]key.Binding{panes, actions, system}

	if focus == paneRepositories || focus == paneWorkItems || focus == panePullRequests {
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
