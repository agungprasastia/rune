package tui

import "rune/internal/tools"

type ExecutionState string

const (
	ExecutionRunning     ExecutionState = "running"
	ExecutionCompleted   ExecutionState = "completed"
	ExecutionFailed      ExecutionState = "failed"
	ExecutionInterrupted ExecutionState = "interrupted"
	ExecutionPermission  ExecutionState = "permission"
)

type DisplayState string

const (
	DisplayExpanded  DisplayState = "expanded"
	DisplayCollapsed DisplayState = "collapsed"
)

type TranscriptScrollState string

const (
	FollowTail   TranscriptScrollState = "follow-tail"
	UserAnchored TranscriptScrollState = "user-anchored"
)

func (m model) transcriptScrollState() TranscriptScrollState {
	if m.chatScrollOffset > 0 {
		return UserAnchored
	}
	return FollowTail
}

func (row transcriptRow) executionState() ExecutionState {
	if row.permission != nil {
		return ExecutionPermission
	}
	if row.kind == rowToolCall {
		return ExecutionRunning
	}
	if row.status == tools.StatusError {
		return ExecutionFailed
	}
	return ExecutionCompleted
}

func (row transcriptRow) displayState() DisplayState {
	if row.expanded {
		return DisplayExpanded
	}
	return DisplayCollapsed
}
