# Security Features

ycode implements multiple security features to ensure safe operation.

## Sandbox System

The sandbox system provides secure command execution with resource limits.

### Features

- **Resource Limits**: Timeout, memory, file size constraints
- **Network Control**: Allow/deny network access
- **Path Restrictions**: Restrict file system access to specific directories
- **Command Blocking**: Block dangerous commands (rm -rf, format, etc.)
- **Environment Protection**: Hide sensitive environment variables (API keys, secrets)

### Configuration

```yaml
sandbox:
  enabled: true
  timeout: 30s
  max_memory: 512MB
  max_file_size: 100MB
  network: false
  allowed_paths:
    - ./src
    - ./tests
  blocked_commands:
    - rm -rf
    - format
    - del /s
```

### Platform Support

- **Unix**: Uses cgroups and ulimit for resource control
- **Windows**: Uses job objects and process limits

### Usage

```go
// Create sandbox executor
executor := sandbox.NewExecutor(workDir, config)

// Execute command in sandbox
result, err := executor.Execute(ctx, "npm test")
```

## Permission System

### Permission Modes

| Mode | Description |
|------|-------------|
| `strict` | All operations require confirmation |
| `normal` | Safe operations allowed, dangerous ones need confirmation |
| `permissive` | Most operations allowed automatically |
| `yolo` | Auto-approve all operations (use with caution) |

### Permission Rules

Configure fine-grained permission rules:

```yaml
permissions:
  mode: normal
  rules:
    - tool: bash
      action: confirm
      patterns:
        - "rm *"
        - "npm publish"
    
    - tool: write_file
      action: allow
      patterns:
        - "./src/**"
        - "./docs/**"
    
    - tool: edit_file
      action: confirm
      patterns:
        - "**/*.go"
```

### Permission Checker

The unified permission checker validates all tool operations:

```go
// Check permission before execution
allowed, err := checker.Check(ctx, toolName, args)
if !allowed {
    // Handle denied operation
}
```

## Audit System

The audit system logs all sensitive operations for security review.

### Audit Events

- File writes and edits
- Shell command executions
- Permission decisions
- Configuration changes
- Plugin operations

### Audit Log Format

```json
{
  "timestamp": "2024-01-16T10:30:00Z",
  "event": "file_write",
  "tool": "write_file",
  "path": "./src/main.go",
  "user_decision": "approved",
  "duration_ms": 15
}
```

### Configuration

```yaml
audit:
  enabled: true
  log_file: ~/.ycode/audit.log
  max_size: 100MB
  max_backups: 10
  sensitive_patterns:
    - "*password*"
    - "*secret*"
    - "*api_key*"
    - "*.env"
```

### Usage

```go
// Create audit logger
audit := audit.NewLogger(config)

// Log sensitive operation
audit.Log(ctx, audit.Event{
    Type: "file_write",
    Tool: "write_file",
    Path: filePath,
    Decision: "approved",
})
```

## Path Security

All file operations validate paths to prevent directory traversal attacks:

```go
// Validate path is within working directory
func validatePath(workDir, path string) error {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return err
    }
    
    if !strings.HasPrefix(absPath, workDir) {
        return errors.New("path traversal detected")
    }
    
    return nil
}
```

## Binary File Detection

Binary files are automatically detected and rejected when reading as text:

```go
// Check if file is binary
func isBinary(data []byte) bool {
    // Check for null bytes (common in binary files)
    for _, b := range data {
        if b == 0 {
            return true
        }
    }
    return false
}
```

## Best Practices

### For Users

1. **Use Appropriate Permission Mode**
   - Development: `normal` or `permissive`
   - Production: `strict`
   - Never use `yolo` in production

2. **Review Audit Logs**
   - Regularly check audit logs
   - Monitor for suspicious activity

3. **Configure Sandbox**
   - Always enable sandbox in production
   - Set appropriate resource limits
   - Restrict network access

### For Developers

1. **Always Validate Paths**
   ```go
   if err := validatePath(workDir, userPath); err != nil {
       return nil, err
   }
   ```

2. **Use Context Timeouts**
   ```go
   ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
   defer cancel()
   ```

3. **Check Permissions**
   ```go
   allowed, err := checker.Check(ctx, toolName, args)
   if err != nil || !allowed {
       return nil, err
   }
   ```

4. **Log Sensitive Operations**
   ```go
   audit.Log(ctx, audit.Event{
       Type: "sensitive_op",
       Tool: toolName,
   })
   ```

## Security Checklist

- [ ] Sandbox enabled with resource limits
- [ ] Appropriate permission mode configured
- [ ] Audit logging enabled
- [ ] Path validation for all file operations
- [ ] Context timeouts for external operations
- [ ] Binary file detection enabled
- [ ] Environment variable protection
- [ ] Command blocking configured