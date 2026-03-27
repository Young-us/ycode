package plugin

// Hook name constants define the available hook points in the system.
// Plugins can register handlers for these hooks to extend or modify behavior.
const (
	// HookOnChatStart is triggered when a new chat session begins.
	// Args: "messages" ([]Message), "config" (map[string]interface{})
	// Return: modified "messages" to alter the conversation context
	HookOnChatStart = "on_chat_start"

	// HookOnChatComplete is triggered when a chat response is generated.
	// Args: "response" (string), "messages" ([]Message), "usage" (map[string]int)
	// Return: modified "response" to alter the output
	HookOnChatComplete = "on_chat_complete"

	// HookOnToolExecute is triggered before a tool is executed.
	// Args: "tool_name" (string), "args" (map[string]interface{})
	// Return: modified "args" to alter tool input, or set "skip" to skip execution
	HookOnToolExecute = "on_tool_execute"

	// HookOnToolComplete is triggered after a tool execution completes.
	// Args: "tool_name" (string), "result" (string), "error" (error)
	// Return: modified "result" to alter tool output
	HookOnToolComplete = "on_tool_complete"

	// HookOnError is triggered when an error occurs in the system.
	// Args: "error" (error), "context" (string), "severity" (string)
	// Return: modified "error" to transform or wrap the error
	HookOnError = "on_error"

	// HookOnAgentSwitch is triggered when switching between agents.
	// Args: "from_agent" (string), "to_agent" (string), "reason" (string)
	// Return: modified "to_agent" to redirect to a different agent
	HookOnAgentSwitch = "on_agent_switch"

	// HookOnFileRead is triggered after reading a file.
	// Args: "path" (string), "content" (string)
	// Return: modified "content" to transform file contents
	HookOnFileRead = "on_file_read"

	// HookOnFileWrite is triggered before writing to a file.
	// Args: "path" (string), "content" (string)
	// Return: modified "content" to transform before writing
	HookOnFileWrite = "on_file_write"

	// HookOnCommandExecute is triggered before executing a shell command.
	// Args: "command" (string), "working_dir" (string)
	// Return: modified "command" to alter the command
	HookOnCommandExecute = "on_command_execute"

	// HookOnStartup is triggered when the application starts.
	// Args: "version" (string), "config" (map[string]interface{})
	// Return: no effect, informational only
	HookOnStartup = "on_startup"

	// HookOnShutdown is triggered when the application is shutting down.
	// Args: "reason" (string)
	// Return: no effect, informational only
	HookOnShutdown = "on_shutdown"
)

// AllHooks returns a list of all available hook names.
func AllHooks() []string {
	return []string{
		HookOnChatStart,
		HookOnChatComplete,
		HookOnToolExecute,
		HookOnToolComplete,
		HookOnError,
		HookOnAgentSwitch,
		HookOnFileRead,
		HookOnFileWrite,
		HookOnCommandExecute,
		HookOnStartup,
		HookOnShutdown,
	}
}

// HookDescription returns a human-readable description for a hook name.
func HookDescription(hookName string) string {
	descriptions := map[string]string{
		HookOnChatStart:      "Triggered when a new chat session begins",
		HookOnChatComplete:   "Triggered when a chat response is generated",
		HookOnToolExecute:    "Triggered before a tool is executed",
		HookOnToolComplete:   "Triggered after a tool execution completes",
		HookOnError:          "Triggered when an error occurs",
		HookOnAgentSwitch:    "Triggered when switching between agents",
		HookOnFileRead:       "Triggered after reading a file",
		HookOnFileWrite:      "Triggered before writing to a file",
		HookOnCommandExecute: "Triggered before executing a shell command",
		HookOnStartup:        "Triggered when the application starts",
		HookOnShutdown:       "Triggered when the application shuts down",
	}
	if desc, ok := descriptions[hookName]; ok {
		return desc
	}
	return "Unknown hook"
}
