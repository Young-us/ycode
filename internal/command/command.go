package command

// Source represents command source
type Source string

const (
	SourceCommand Source = "command"
	SourceMCP     Source = "mcp"
	SourceSkill   Source = "skill"
)

// Command represents a slash command
type Command struct {
	Name        string
	Description string
	Usage       string
	Template    string // 支持 $1, $2, $ARGUMENTS
	Agent       string
	Model       string
	Source      Source
	Subtask     bool
	Hints       []string
	Handler     func(args []string) (string, error)
}
