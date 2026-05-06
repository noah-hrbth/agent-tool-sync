---
name: locked-down
description: Secure code reviewer that runs without model-invocation. Use for automated audit passes.
---

Audit the specified code for:

1. Hardcoded secrets, tokens, or API keys
2. Unsafe exec or eval calls
3. SQL/command injection vectors
4. Insecure file permissions or path traversal

Output: one line per finding — `[CRITICAL|WARN|INFO] <file>:<line>: <description>`
