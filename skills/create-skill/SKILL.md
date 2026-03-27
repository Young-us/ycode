---
name: create-skill
description: Create a new ycode skill with proper structure and format. Use when you need to create or scaffold a new skill for ycode.
triggers:
  - create skill
  - new skill
  - add skill
  - make skill
commands:
  - /create-skill
---

# Create Skill Guide

## Overview

This skill helps you create new ycode skills with proper structure and format.

## Usage

To create a new skill, provide:
1. **Skill name** (required): The name of the skill (e.g., "test-generator")
2. **Description** (required): A brief description of what the skill does
3. **Triggers** (optional): Keywords that trigger this skill
4. **Commands** (optional): Slash commands to invoke this skill (e.g., "/test-gen")

## Default Location

Skills are created in `.claude/skills/<skill-name>/` by default.

## Skill Template

When creating a skill, use this template:

```
---
name: <skill-name>
description: <Brief description of the skill>
triggers:
  - <trigger-word-1>
  - <trigger-word-2>
commands:
  - /<command-name>
permissions:
  - <required-permission>  # optional
---

# <Skill Name> Guide

## Overview

<Detailed explanation of what this skill does and when to use it>

## Instructions

<Step-by-step instructions for the skill>

1. **Step 1**
   - Details

2. **Step 2**
   - Details

## Output Format

<Expected output format when this skill is invoked>

## Examples

<Usage examples>
```

## Creating a Skill

When asked to create a skill:

1. **Ask for required information** if not provided:
   - Skill name
   - Description
   - Triggers (suggest based on skill purpose)
   - Commands (suggest based on skill name)

2. **Create the skill directory**:
   ```
   .claude/skills/<skill-name>/
   ```

3. **Create SKILL.md** with proper frontmatter and instructions

4. **Confirm creation** and show the skill content

## Example

User: "Create a skill called 'api-tester' for testing REST APIs"

Create:
```
.claude/skills/api-tester/SKILL.md
```

With content:
```yaml
---
name: api-tester
description: Test REST APIs by making HTTP requests and validating responses
triggers:
  - api test
  - test api
  - http test
commands:
  - /api-test
---

# API Tester Guide

## Overview
Test REST APIs by making HTTP requests...
```