---
name: code-review
description: Reviews code for quality, correctness, and adherence to project conventions. Use when asked to review, check, or audit code changes.
---

Review the specified code or diff for:

1. **Correctness** — logic errors, off-by-one errors, null/undefined handling
2. **Style** — adherence to project conventions (see AGENTS.md)
3. **Test coverage** — are critical paths tested?
4. **Security** — injection risks, exposed secrets, unsafe operations

Output a structured report:
- **Summary**: one-sentence verdict
- **Issues**: bulleted list (severity: 🔴 critical / 🟡 warning / 🔵 suggestion)
- **Positives**: what was done well