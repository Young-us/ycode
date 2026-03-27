package layout

import (
	"reflect"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Focusable represents a component that can receive/lose focus
type Focusable interface {
	Focus() tea.Cmd
	Blur() tea.Cmd
	IsFocused() bool
}

// Sizeable represents a component with configurable dimensions
type Sizeable interface {
	SetSize(width, height int) tea.Cmd
	GetSize() (int, int)
}

// Bindings represents a component with key bindings
type Bindings interface {
	BindingKeys() []key.Binding
}

// KeyMapToSlice extracts all key.Binding fields from a struct using reflection
func KeyMapToSlice(t any) (bindings []key.Binding) {
	typ := reflect.TypeOf(t)
	if typ.Kind() != reflect.Struct {
		return nil
	}
	for i := range typ.NumField() {
		v := reflect.ValueOf(t).Field(i)
		bindings = append(bindings, v.Interface().(key.Binding))
	}
	return
}
