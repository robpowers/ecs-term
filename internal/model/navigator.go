package model

import tea "github.com/charmbracelet/bubbletea"

type ViewID int

const (
	ViewContextSelector ViewID = iota
	ViewServices
	ViewTasks
	ViewLogs
	ViewContainerDetail
)

// View is the interface every view model must satisfy.
type View interface {
	tea.Model
	ViewID() ViewID
	SetSize(width, height int)
	KeyHints() []string
}

type Navigator struct {
	stack []View
}

func NewNavigator(initial View) Navigator {
	return Navigator{stack: []View{initial}}
}

func (n *Navigator) Push(v View) {
	n.stack = append(n.stack, v)
}

// Pop removes the top view and returns it. Returns nil if only one view remains.
func (n *Navigator) Pop() View {
	if len(n.stack) <= 1 {
		return nil
	}
	top := n.stack[len(n.stack)-1]
	n.stack = n.stack[:len(n.stack)-1]
	return top
}

func (n *Navigator) Current() View {
	return n.stack[len(n.stack)-1]
}

func (n *Navigator) Depth() int {
	return len(n.stack)
}

// SetSizeAll propagates a size update to every view in the stack.
func (n *Navigator) SetSizeAll(width, height int) {
	for _, v := range n.stack {
		v.SetSize(width, height)
	}
}
