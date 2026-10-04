package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/revdiff/app/annotation"
	"github.com/umputun/revdiff/app/diff"
	"github.com/umputun/revdiff/app/keymap"
)

// fixtures for the cross-file motion tests: a.go ends on a context line so a down
// press has nowhere to go, b.go and c.go open with a divider so a forward landing
// must skip it.

func crossFileALines() []diff.DiffLine {
	return []diff.DiffLine{
		{ChangeType: diff.ChangeContext, Content: "a-ctx1", OldNum: 1, NewNum: 1},
		{ChangeType: diff.ChangeAdd, Content: "a-add", OldNum: 2, NewNum: 2},
		{ChangeType: diff.ChangeContext, Content: "a-ctx2", OldNum: 3, NewNum: 3},
	}
}

func crossFileBLines() []diff.DiffLine {
	return []diff.DiffLine{
		{ChangeType: diff.ChangeDivider},
		{ChangeType: diff.ChangeContext, Content: "b-ctx1", OldNum: 1, NewNum: 1},
		{ChangeType: diff.ChangeAdd, Content: "b-add", OldNum: 2, NewNum: 2},
	}
}

func crossFileCLines() []diff.DiffLine {
	return []diff.DiffLine{
		{ChangeType: diff.ChangeDivider},
		{ChangeType: diff.ChangeContext, Content: "c-ctx1", OldNum: 1, NewNum: 1},
		{ChangeType: diff.ChangeAdd, Content: "c-add", OldNum: 2, NewNum: 2},
	}
}

func crossFileDiffs() map[string][]diff.DiffLine {
	return map[string][]diff.DiffLine{
		"a.go": crossFileALines(),
		"b.go": crossFileBLines(),
		"c.go": crossFileCLines(),
	}
}

// crossFileMotionModel loads a.go with cross-file motion enabled and the diff pane focused.
func crossFileMotionModel(t *testing.T, files ...string) Model {
	t.Helper()
	if len(files) == 0 {
		files = []string{"a.go", "b.go"}
	}
	m := loadFileIntoModel(t, files, crossFileDiffs())
	m.cfg.crossFileMotion = true
	m.layout.focus = paneDiff
	return m
}

// crossFileSelectAndLoad moves the tree cursor to path and delivers its load synchronously.
func crossFileSelectAndLoad(t *testing.T, m Model, path string) Model {
	t.Helper()
	require.True(t, m.tree.SelectByPath(path), "tree must contain %s", path)
	m.file.loadSeq++
	result, _ := m.Update(m.loadFileDiff(path)())
	m = result.(Model)
	m.layout.viewport.Height = 20
	m.layout.focus = paneDiff
	return m
}

// crossFileParkAtBoundary puts the cursor on the first or last selectable line of the
// displayed file, the position from which the motion is expected to cross.
func crossFileParkAtBoundary(m Model, forward bool) Model {
	if forward {
		m.nav.diffCursor = len(m.file.lines) - 1
		return m
	}
	m.nav.diffCursor = 1 // index 0 is the leading divider
	return m
}

func TestModel_MotionCrossFile_DownForward(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	model := result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump, "down at the last line must queue a landing")
	assert.True(t, *model.nav.pendingBoundaryJump, "forward motion lands at the top of the next file")
	assert.Equal(t, "b.go", model.tree.SelectedFile(), "the tree must advance to the next file")
	require.NotNil(t, cmd, "the next file's load must be requested")
	assert.Equal(t, paneDiff, model.layout.focus, "the diff pane keeps focus")
}

func TestModel_MotionCrossFile_UpBackward(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileSelectAndLoad(t, crossFileMotionModel(t), "b.go"), false)

	result, cmd := m.handleDiffAction(keymap.ActionUp)
	model := result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump, "up at the first line must queue a landing")
	assert.False(t, *model.nav.pendingBoundaryJump, "backward motion lands at the bottom of the previous file")
	assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree must move to the previous file")
	require.NotNil(t, cmd, "the previous file's load must be requested")
}

// assertMotionCrosses drives a page-sized boundary motion and expects one crossing.
func assertMotionCrosses(t *testing.T, action keymap.Action, forward bool) {
	t.Helper()

	m := crossFileMotionModel(t)
	if !forward {
		m = crossFileSelectAndLoad(t, m, "b.go")
	}
	m = crossFileParkAtBoundary(m, forward)

	result, cmd := m.handleDiffAction(action)
	model := result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump, string(action)+" at the boundary must queue a landing")
	assert.Equal(t, forward, *model.nav.pendingBoundaryJump, string(action)+" must land in the direction of the motion")
	assert.NotNil(t, cmd, string(action)+" must request the adjacent file's load")
	want := "b.go"
	if !forward {
		want = "a.go"
	}
	assert.Equal(t, want, model.tree.SelectedFile(), string(action)+" must step exactly one file")
}

func TestModel_MotionCrossFile_PageDown(t *testing.T) {
	assertMotionCrosses(t, keymap.ActionPageDown, true)
}

func TestModel_MotionCrossFile_PageUp(t *testing.T) {
	assertMotionCrosses(t, keymap.ActionPageUp, false)
}

func TestModel_MotionCrossFile_HalfPageDown(t *testing.T) {
	assertMotionCrosses(t, keymap.ActionHalfPageDown, true)
}

func TestModel_MotionCrossFile_HalfPageUp(t *testing.T) {
	assertMotionCrosses(t, keymap.ActionHalfPageUp, false)
}

func TestModel_MotionCrossFile_LandsAtTop(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	m = result.(Model)
	require.NotNil(t, cmd)
	result, _ = m.Update(cmd())
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "the accepted load must consume the landing")
	assert.Equal(t, "b.go", model.file.name, "the next file must be displayed")
	assert.Equal(t, 1, model.nav.diffCursor, "the cursor lands on the first selectable line, past the divider")
	assert.Zero(t, model.layout.viewport.YOffset, "the new file is shown from the top")
	assert.Equal(t, "b.go", model.tree.SelectedFile(), "the tree selection must follow")
	assert.Equal(t, paneDiff, model.layout.focus)
}

func TestModel_MotionCrossFile_LandsAtBottom(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileSelectAndLoad(t, crossFileMotionModel(t), "b.go"), false)

	result, cmd := m.handleDiffAction(keymap.ActionUp)
	m = result.(Model)
	require.NotNil(t, cmd)
	result, _ = m.Update(cmd())
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "the accepted load must consume the landing")
	assert.Equal(t, "a.go", model.file.name, "the previous file must be displayed")
	assert.Equal(t, len(crossFileALines())-1, model.nav.diffCursor, "the cursor lands on the last selectable line")
	assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree selection must follow")
}

func TestModel_MotionCrossFile_AtEndsNoOp(t *testing.T) {
	t.Run("down on the last file", func(t *testing.T) {
		m := crossFileParkAtBoundary(crossFileSelectAndLoad(t, crossFileMotionModel(t), "b.go"), true)

		result, cmd := m.handleDiffAction(keymap.ActionDown)
		model := result.(Model)

		assert.Nil(t, model.nav.pendingBoundaryJump, "no wrap-around past the last file")
		assert.Equal(t, "b.go", model.tree.SelectedFile(), "the tree selection must not move")
		assert.Nil(t, cmd, "no load may be requested")
		assert.Equal(t, len(crossFileBLines())-1, model.nav.diffCursor, "the cursor stays on the last line")
	})

	t.Run("up on the first file", func(t *testing.T) {
		m := crossFileParkAtBoundary(crossFileMotionModel(t), false)
		m.nav.diffCursor = 0 // a.go has no leading divider

		result, cmd := m.handleDiffAction(keymap.ActionUp)
		model := result.(Model)

		assert.Nil(t, model.nav.pendingBoundaryJump, "no wrap-around before the first file")
		assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree selection must not move")
		assert.Nil(t, cmd, "no load may be requested")
		assert.Zero(t, model.nav.diffCursor, "the cursor stays on the first line")
	})
}

func TestModel_MotionCrossFile_SingleFileNoCross(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)
	m.file.singleFile = true
	m.layout.treeWidth = 0

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "single-file mode (stdin, compare, one-file diff) never crosses")
	assert.Equal(t, "a.go", model.tree.SelectedFile())
	assert.Nil(t, cmd)
}

func TestModel_MotionCrossFile_DefaultDoesNotCross(t *testing.T) {
	m := loadFileIntoModel(t, []string{"a.go", "b.go"}, crossFileDiffs())
	m.layout.focus = paneDiff // crossFileMotion stays at its default: off
	m.nav.diffCursor = len(crossFileALines()) - 1

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "the flag is off by default")
	assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree must not move without the flag")
	assert.Nil(t, cmd)
	assert.Equal(t, len(crossFileALines())-1, model.nav.diffCursor)
}

func TestModel_MotionCrossFile_TreeFocusNotAffected(t *testing.T) {
	m := crossFileMotionModel(t)
	m.layout.focus = paneTree
	m.nav.diffCursor = 0 // parked at the top of the displayed file

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "tree-pane motion must not queue a diff landing")
	assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree keeps its own cursor semantics")
	assert.Nil(t, cmd)
	assert.Zero(t, model.nav.diffCursor, "the diff cursor must not move from the tree pane")
	assert.Equal(t, paneTree, model.layout.focus)
}

func TestModel_MotionCrossFile_ScrollActionsDoNotCross(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)
	m.layout.viewport.SetContent(m.renderDiff())
	m.layout.viewport.SetYOffset(m.layout.viewport.TotalLineCount()) // parked at the bottom

	for _, action := range []keymap.Action{
		keymap.ActionScrollDiffDown,
		keymap.ActionScrollDiffPageDown,
		keymap.ActionScrollDiffHalfPageDown,
		keymap.ActionHome,
		keymap.ActionEnd,
	} {
		result, cmd := m.handleDiffAction(action)
		model := result.(Model)

		assert.Nil(t, model.nav.pendingBoundaryJump, string(action)+" must not cross files")
		assert.Equal(t, "a.go", model.tree.SelectedFile(), string(action)+" must not move the tree")
		assert.Nil(t, cmd, string(action)+" must not request a load")
		m = model
	}
}

func TestModel_MotionCrossFile_VimCountRepeatDoesNotCross(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)
	m.modes.vimMotion = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m = result.(Model)
	require.Nil(t, cmd, "a digit accumulates the count without moving")

	result, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := result.(Model)

	assert.Nil(t, model.nav.pendingBoundaryJump, "a count-prefixed motion must not cross files")
	assert.Equal(t, "a.go", model.tree.SelectedFile())
	assert.Nil(t, cmd)
	assert.Equal(t, len(crossFileALines())-1, model.nav.diffCursor, "the cursor clamps at the last line")
}

func TestModel_MotionCrossFile_MouseWheelDoesNotCross(t *testing.T) {
	m := mouseTestModel(t, []string{"a.go", "b.go"}, crossFileDiffs())
	m.cfg.crossFileMotion = true
	m.layout.focus = paneDiff
	m.nav.diffCursor = len(crossFileALines()) - 1
	m.layout.viewport.SetContent(m.renderDiff())
	m.layout.viewport.SetYOffset(m.layout.viewport.TotalLineCount())

	model := updateWheelAndFlush(t, m, wheelMsg(tea.MouseButtonWheelDown, 60, 10, false))

	assert.Nil(t, model.nav.pendingBoundaryJump, "the wheel scrolls the viewport, it does not cross files")
	assert.Equal(t, "a.go", model.tree.SelectedFile(), "the tree must not move on wheel events")
}

func TestModel_MotionCrossFile_AnnotationSubRow(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)
	m.store.Add(annotation.Annotation{File: "a.go", Line: 3, Type: " ", Comment: "note on the last line"})

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	model := result.(Model)

	require.True(t, model.annot.cursorOnAnnotation, "the press lands on the annotation sub-row")
	assert.Nil(t, model.nav.pendingBoundaryJump, "a press that moved must not cross")
	assert.Equal(t, "a.go", model.tree.SelectedFile())
	assert.Nil(t, cmd)

	result, cmd = model.handleDiffAction(keymap.ActionDown)
	model = result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump, "the next press has nowhere to go, so it crosses")
	assert.Equal(t, "b.go", model.tree.SelectedFile())
	require.NotNil(t, cmd)
}

func TestModel_MotionCrossFile_FileAnnotationRowBackward(t *testing.T) {
	m := crossFileSelectAndLoad(t, crossFileMotionModel(t), "b.go")
	m.store.Add(annotation.Annotation{File: "b.go", Line: 0, Type: "", Comment: "file note"})
	m.nav.diffCursor = -1 // the file-level annotation row

	result, cmd := m.handleDiffAction(keymap.ActionUp)
	model := result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump, "up from the file annotation row crosses to the previous file")
	assert.False(t, *model.nav.pendingBoundaryJump)
	assert.Equal(t, "a.go", model.tree.SelectedFile())
	require.NotNil(t, cmd)
}

func TestModel_MotionCrossFile_CollapsedLandsOnVisibleLine(t *testing.T) {
	// a.go's tail is a delete-only hunk: collapsed mode keeps only its placeholder,
	// so the backward landing must stop there rather than on a hidden line.
	aLines := []diff.DiffLine{
		{ChangeType: diff.ChangeContext, Content: "a-ctx", OldNum: 1, NewNum: 1},
		{ChangeType: diff.ChangeAdd, Content: "a-add", NewNum: 2},
		{ChangeType: diff.ChangeRemove, Content: "a-del1", OldNum: 2},
		{ChangeType: diff.ChangeRemove, Content: "a-del2", OldNum: 3},
	}
	diffs := crossFileDiffs()
	diffs["a.go"] = aLines

	m := loadFileIntoModel(t, []string{"a.go", "b.go"}, diffs)
	m.cfg.crossFileMotion = true
	m.modes.collapsed.enabled = true
	m = crossFileSelectAndLoad(t, m, "b.go")
	m.nav.diffCursor = 1

	result, cmd := m.handleDiffAction(keymap.ActionUp)
	m = result.(Model)
	require.NotNil(t, cmd)
	result, _ = m.Update(cmd())
	model := result.(Model)

	require.Equal(t, "a.go", model.file.name)
	require.NotEmpty(t, model.file.lines)
	assert.False(t, model.isCollapsedHidden(model.nav.diffCursor, model.findHunks()),
		"the landing must not park on a collapsed-hidden line")
}

func TestModel_MotionCrossFile_BoundaryBeatsStartAtChange(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t), true)
	m.cfg.startAtChange = true

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	m = result.(Model)
	require.NotNil(t, cmd)
	result, _ = m.Update(cmd())
	model := result.(Model)

	// b.go's first change sits at index 2; the explicit motion wins at index 1.
	assert.Equal(t, 1, model.nav.diffCursor, "the boundary landing wins over start-at-change")
	assert.Nil(t, model.nav.pendingBoundaryJump)
}

func TestModel_MotionCrossFile_RapidDoublePressLandsOnSecondFile(t *testing.T) {
	m := crossFileParkAtBoundary(crossFileMotionModel(t, "a.go", "b.go", "c.go"), true)

	result, firstCmd := m.handleDiffAction(keymap.ActionDown)
	m = result.(Model)
	require.NotNil(t, firstCmd, "the first press must request b.go")

	result, secondCmd := m.handleDiffAction(keymap.ActionDown)
	m = result.(Model)
	require.NotNil(t, secondCmd, "the second press must request c.go")
	assert.Equal(t, "c.go", m.tree.SelectedFile(), "one press steps one file, even before the load lands")
	require.NotNil(t, m.nav.pendingBoundaryJump)

	// the superseded b.go result arrives first: it must be dropped without consuming the landing
	result, _ = m.Update(firstCmd())
	model := result.(Model)
	assert.Equal(t, "a.go", model.file.name, "a superseded load must not change the displayed file")
	require.NotNil(t, model.nav.pendingBoundaryJump, "a dropped load must not consume the landing")

	result, _ = model.Update(secondCmd())
	model = result.(Model)
	assert.Equal(t, "c.go", model.file.name, "the accepted load is the second one")
	assert.Nil(t, model.nav.pendingBoundaryJump, "the accepted load consumes the landing exactly once")
	assert.Equal(t, 1, model.nav.diffCursor, "the landing is at the top of c.go")
}

func TestModel_MotionCrossFile_FilteredTreeCrossesToVisibleFile(t *testing.T) {
	m := crossFileMotionModel(t, "a.go", "b.go", "c.go")
	m.tree.ToggleFilter(map[string]bool{"b.go": true, "c.go": true})
	m = crossFileSelectAndLoad(t, m, "b.go")
	m = crossFileParkAtBoundary(m, true)

	result, cmd := m.handleDiffAction(keymap.ActionDown)
	model := result.(Model)

	require.NotNil(t, model.nav.pendingBoundaryJump)
	assert.Equal(t, "c.go", model.tree.SelectedFile(), "crossing follows the filtered tree order")
	require.NotNil(t, cmd)
}

func TestModel_MotionCrossFile_DividerOnlyFileCrossesOncePerPress(t *testing.T) {
	diffs := crossFileDiffs()
	diffs["b.go"] = []diff.DiffLine{{ChangeType: diff.ChangeDivider}}

	m := loadFileIntoModel(t, []string{"a.go", "b.go", "c.go"}, diffs)
	m.cfg.crossFileMotion = true
	m.layout.focus = paneDiff
	m.nav.diffCursor = len(crossFileALines()) - 1

	result, cmd := m.handleDiffAction(keymap.ActionDown) // a.go tail -> b.go
	m = result.(Model)
	require.NotNil(t, cmd)
	result, _ = m.Update(cmd())
	m = result.(Model)

	assert.Equal(t, "b.go", m.file.name)
	assert.Nil(t, m.nav.pendingBoundaryJump, "a divider-only file still consumes the landing")

	result, cmd = m.handleDiffAction(keymap.ActionDown) // nowhere to go inside b.go -> c.go
	m = result.(Model)
	require.NotNil(t, cmd)
	assert.Equal(t, "c.go", m.tree.SelectedFile(), "one press crosses exactly one file")

	result, _ = m.Update(cmd())
	model := result.(Model)

	assert.Equal(t, "c.go", model.file.name)
	assert.Nil(t, model.nav.pendingBoundaryJump, "the intent must never stick on a file with no selectable line")
}

func TestModel_ClearPendingJumps(t *testing.T) {
	m := crossFileMotionModel(t)
	backward := false
	m.nav.pendingHunkJump = &backward
	m.nav.pendingBoundaryJump = &backward
	m.pendingAnnotJump = &annotation.Annotation{File: "a.go", Line: 1, Type: " ", Comment: "note"}

	m.clearPendingJumps()

	assert.Nil(t, m.nav.pendingHunkJump)
	assert.Nil(t, m.nav.pendingBoundaryJump)
	assert.Nil(t, m.pendingAnnotJump)
}

// TestModel_ClearPendingJumps_OnManualNavigation drives the paths that must not let a
// queued landing hijack the load they trigger.
func TestModel_ClearPendingJumps_OnManualNavigation(t *testing.T) {
	tests := []struct {
		name   string
		launch func(t *testing.T, m Model) (tea.Model, tea.Cmd)
	}{
		{
			name: "next file",
			launch: func(_ *testing.T, m Model) (tea.Model, tea.Cmd) {
				return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
			},
		},
		{
			name: "reload",
			launch: func(t *testing.T, m Model) (tea.Model, tea.Cmd) {
				m.reload.applicable = true
				return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
			},
		},
		{
			name: "filter toggle",
			launch: func(_ *testing.T, m Model) (tea.Model, tea.Cmd) {
				m.store.Add(annotation.Annotation{File: "b.go", Line: 2, Type: "+", Comment: "note"})
				return m.handleFilterToggle()
			},
		},
		{
			name: "file picker jump",
			launch: func(_ *testing.T, m Model) (tea.Model, tea.Cmd) {
				return m.jumpToFile("b.go")
			},
		},
		{
			name: "tree navigation",
			launch: func(_ *testing.T, m Model) (tea.Model, tea.Cmd) {
				m.layout.focus = paneTree
				return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			},
		},
		{
			name: "annotation delete",
			launch: func(t *testing.T, m Model) (tea.Model, tea.Cmd) {
				m.store.Add(annotation.Annotation{File: "a.go", Line: 3, Type: " ", Comment: "note"})
				m.nav.diffCursor = 2
				m.annot.cursorOnAnnotation = true
				return m.handleDiffAction(keymap.ActionDeleteAnnotation)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := crossFileMotionModel(t, "a.go", "b.go", "c.go")
			backward := false
			m.nav.pendingHunkJump = &backward
			m.nav.pendingBoundaryJump = &backward
			m.pendingAnnotJump = &annotation.Annotation{File: "a.go", Line: 1, Type: " ", Comment: "note"}

			result, _ := tt.launch(t, m)
			model := result.(Model)

			assert.Nil(t, model.nav.pendingHunkJump, "a stale hunk landing must not survive")
			assert.Nil(t, model.nav.pendingBoundaryJump, "a stale boundary landing must not survive")
			assert.Nil(t, model.pendingAnnotJump, "a stale annotation jump must not survive")
		})
	}
}
