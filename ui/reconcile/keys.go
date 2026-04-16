package reconcileui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Confirm key.Binding
	Quit    key.Binding
	Yes     key.Binding
	No      key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Toggle:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "finish")),
		Quit:    key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
		Yes:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		No:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "cancel")),
	}
}
