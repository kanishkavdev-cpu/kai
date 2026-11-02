# kai: Progress and Learning Journal

What I built, how I built it, and what I learned. One entry per task, plus a summary per day.

## Concepts I've learned
### Go
- The module path is the prefix of every import, and `go install` uses it to find the code
- The compiler blocks other projects from importing anything under `internal/`
- The `go` line in `go.mod` sets the minimum version, not the compiler used, and dependencies can raise it
- `package main` with `func main()` builds a program; any other package is a library
- `main()` can't return an error, so failure is reported with `os.Exit(1)`
### Agentic AI / Kubernetes
- client-go versions map to Kubernetes versions (`v0.37.x` is Kubernetes 1.37), and all three `k8s.io` modules should stay on the same version
- Always pass `--context` explicitly, because the current kubectl context may point at the wrong cluster

## Open questions
-

---

## Day 1: Lab cluster and read-only Kubernetes access
**Status:** in progress

<!-- Add one entry per task:
### Task: <task name from PLAN.md>
- **What I built:**
- **How it works:**
- **Go concepts used:**
- **Agent concepts used:**
- **Problems I hit and how I solved them:**
- **Review feedback I applied:**
-->

### Task: `go mod init`, add cobra, client-go, a Makefile
*(Drafted by Claude at my request. `make lab` is covered by the next task.)*
- **What I built:** `cmd/kai/main.go` with `newRootCmd()` and `main()`, a Makefile with `build`, `run` and `test` targets, a `.gitignore`, and a `go.mod` with cobra 1.10.2 and client-go, api and apimachinery at v0.37.2.
- **How it works:** `newRootCmd()` builds a new cobra command each time it's called. `main()` runs it with `Execute()` and exits with status 1 if it returns an error. The command has no subcommands yet, so it only prints help text.
- **Go concepts used:** modules and the `go` directive, `package main`, pointers to structs, returning errors, exit codes with `os.Exit`.
- **Agent concepts used:** none yet.
- **Problems I hit and how I solved them:**
  - `package kai` built fine but wouldn't run ("is not a main package"), so I changed it to `package main`.
  - I chose `v0.37.2` over `v0.32.x` because 1.32 no longer gets upstream support. This raised the `go` line to 1.26.0 and meant upgrading kind and kubectl.
  - `go build ./...` leaves a `kai` binary in the root while there's only one package, so `/kai` went into `.gitignore`.
- **Review feedback I applied:**
  - Changed `newRootCmd` from a global variable to a function, so each call gives a new command and state can't leak between tests.
  - Built `./cmd/kai` instead of `./cmd/kai/main.go`, which would break once the folder has a second file.
  - Added `.PHONY`, a `test` target, and formatting with `gofmt`.

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 2: Deterministic analyzers
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 3: The agent loop with read-only tools
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 4: Remediation with approval and audit
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 5: MCP server mode and a local model
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 6: Eval suite and hardening
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts:

---

## Day 7: Deploy, document, demo
**Status:** not started

**Day summary**
- "Done when" result:
- What I learned:
- Still unclear:
- Scope cuts: