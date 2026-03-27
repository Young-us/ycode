---
name: code-review
description: Expert code review focusing on security, performance, and maintainability. Use when reviewing pull requests or examining code quality.
triggers:
  - review
  - code review
  - pr review
commands:
  - /review
---

# Code Review Guidelines

## Review Process

1. **Security Check**
   - Look for SQL injection, XSS, CSRF vulnerabilities
   - Check for hardcoded secrets or credentials
   - Verify input validation and sanitization
   - Review authentication and authorization logic

2. **Performance Analysis**
   - Identify N+1 queries
   - Check for unnecessary database calls
   - Look for memory leaks
   - Review algorithm complexity

3. **Code Quality**
   - Follow SOLID principles
   - Check for code duplication
   - Verify proper error handling
   - Review naming conventions

4. **Maintainability**
   - Assess code readability
   - Check for proper documentation
   - Verify test coverage
   - Review dependency management

## Output Format

Provide feedback in this format:

### Issues Found
- **[Severity]** Description of issue
  - File: `path/to/file.go`
  - Line: 42
  - Suggestion: How to fix

### Suggestions
- Recommendation 1
- Recommendation 2

### Summary
Overall assessment of code quality
