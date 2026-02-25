---
name: Engineering issue
about: Internal implementation-ready issue template for engineering work
title: ''
labels: ''
assignees: ''
---

## Summary

<One-paragraph summary of the problem and impact.>

## Why

<Why this matters now: correctness, reliability, security, performance, or maintainability risk.>

## Current gap in the code

- File: `<repo-relative-path>`
- Lines: `<start-end>`
- Gap: <What the current implementation does and why it is insufficient.>

Use file paths relative to the repository root to avoid machine-specific noise.

## Proposed change

- <Specific change 1>
- <Specific change 2>
- <Specific change 3>

## Testing

- <Unit/integration test updates needed>
- <Failure case that should be covered>
- <Regression case that should be covered>

## Validation command

```fish
# Replace with exact commands run locally
go test ./...
```

## Acceptance Criteria

- [ ] <Behavioral criterion 1>
- [ ] <Behavioral criterion 2>
- [ ] <Output/exit code/contract criterion>
- [ ] <Tests added/updated and passing>
