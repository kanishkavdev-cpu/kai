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
- Make treats target names as files; `.PHONY` marks them as commands so they always run
- An idempotent command is safe to run repeatedly; destroying things should be a separate, deliberate command
- Pin images by digest (`@sha256:...`) so the exact version can't change under you
- A 403 from the API server comes after authentication: with no credentials you are `system:anonymous`, and RBAC refuses
- Each failure leaves its evidence in a different place: logs (missing env var), events (bad image, missing ConfigMap key, scheduling, PVC), Last State and exit code (OOMKilled = 137), or the Service and its endpoints (selector mismatch)
- `STATUS` in `kubectl get pods` is a snapshot that cycles (`Running` → `Error` → `CrashLoopBackOff`), so check restart count, Last State and events instead
- A stuck rollout keeps old and new pods side by side, so a Deployment can have several pods at once
- `Unknown` pod status means the kubelet hasn't reported yet, not that the app failed

## Open questions
- Why does `kubectl logs --previous` sometimes return `unable to retrieve container logs`? (Seen on scenarios 1 and 4; a retry sometimes works)

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

### Task: `lab/kind.yaml` and `make lab` to create and tear down the cluster
*(Drafted by Claude at my request. Claude wrote `lab/kind.yaml` and the `lab` targets, with my agreement.)*
- **What I built:** `lab/kind.yaml`, a single-node cluster pinned to `kindest/node:v1.37.0` by digest, plus the Makefile targets `lab`, `lab-down` and `lab-reset`.
- **How it works:** `make lab` creates the `kai-lab` cluster only if it doesn't already exist, then waits for the node to be Ready. `make lab-down` deletes the cluster, and `make lab-reset` runs both to start clean. All kubectl commands pass `--context kind-kai-lab`, so they can't reach another cluster by accident.
- **Go concepts used:** none; this task was configuration.
- **Agent concepts used:** a reproducible environment, so eval scores on Day 6 can be compared between runs.
- **Problems I hit and how I solved them:**
  - `make: kind: No such file or directory` (exit code 127): the shell's `PATH` didn't include `~/go/bin`, where kind is installed. Fixed by adding it to `PATH`.
  - Cluster creation failed most of the time with a `ClusterRoleBinding ... context deadline exceeded` error. That message made it look like a permissions problem, but the logs showed the control-plane containers were only starting around 47 seconds in, close to kubeadm's roughly 60-second limit. Raising kubeadm's timeouts to 5 minutes in `kind.yaml` fixed it (2 out of 2 successes, compared with 1 out of 7 before).
  - The `kai-lab` node container kept showing `Exited (137)` with Docker's `OOMKilled` flag set. At first we blamed running a second cluster (`kai`). Docker's event log showed a `stop` command instead, and the kernel log showed no out-of-memory kill of the node. The `OOMKilled` flag came from scenario 4's own expected OOM kills inside the node. Lesson: exit 137 only means "killed with SIGKILL", so check `docker events` before trusting the flag. After a restart, pods were `Unknown` for a few minutes.
- **Review feedback I applied:** single node instead of several; skip creation instead of failing when the cluster exists; keep the cluster name in one Makefile variable.

### Task: Write the 8 broken scenarios in `lab/scenarios/`
*(Drafted by Claude at my request. I wrote scenarios 2 and 3; Claude wrote 1 and 4 to 8.)*
- **What I built:** one folder per scenario under `lab/scenarios/`, each with a `manifest.yaml` containing its own Namespace and a `checkout-service` Deployment.

  | # | Namespace | The bug | What it shows |
  |---|---|---|---|
  | 1 | `crash-missing-env` | `DATABASE_URL` never set | `CrashLoopBackOff`, the cause only in logs |
  | 2 | `crash-missing-config-key` | Reads ConfigMap key `user`, which doesn't exist | `CreateContainerConfigError`, cause in events |
  | 3 | `imagepull-bad-tag` | Image `busybox:1.36-typo` | `ImagePullBackOff`, cause in events |
  | 4 | `oomkilled-low-memory` | Needs about 100 MB, limit 32Mi | `OOMKilled`, exit code 137 |
  | 5 | `liveness-wrong-port` | App on 8080, probe on 9090 | Restarts, `Liveness probe failed` events |
  | 6 | `pending-too-large` | Requests 1000 CPUs and 1Ti | `Pending`, `Insufficient cpu/memory` |
  | 7 | `pvc-missing-storageclass` | PVC asks for StorageClass `fast-ssd` | PVC and pod `Pending`, cause on the PVC |
  | 8 | `service-selector-mismatch` | Service selects `app=checkout` | Healthy pods, Service has no endpoints |
- **How it works:** each scenario has one planted bug. A separate namespace per scenario means `kubectl delete namespace <name>` resets it, and events from different scenarios don't mix. Every app keeps running once its bug is fixed, which was tested on throwaway copies, so a correct fix on Day 4 really makes the pod healthy.
- **Go concepts used:** none; this task was YAML.
- **Agent concepts used:** this is the eval set for Day 6. Checking the status alone isn't enough; the event or log has to name the exact bug.
- **Problems I hit and how I solved them:**
  - VS Code showed `Property kind is not allowed` (CrowdSec Scenario). The YAML extension matched the `scenarios/` folder to an unrelated schema. Fixed by mapping `lab/scenarios/**/*.yaml` to `kubernetes` in `.vscode/settings.json`.
  - My first versions of scenarios 2 and 3 had no `command`, so `busybox` exited straight away. Even with the bug fixed, the pod would never become healthy.
  - Scenario 2 is a `CreateContainerConfigError`, not a `CrashLoopBackOff` as the plan said, because the container never starts.
  - Changing scenario 2 left two pods stuck side by side, because the new pod could never be Ready. Deleting the namespace and re-applying gave a clean start.
- **Review feedback I applied:** added a `command` that keeps each container running; put the ConfigMap before the Deployment; fixed the copied comments at the top of each file.

### Notes for the `internal/kube` task
- `logs --previous` can fail, so treat it as "no logs available", not a fatal error.
- A Deployment can have several pods at once (stuck rollout), so don't only read the first pod.
- Some causes live on related objects: the PVC (scenario 7) or a Service whose selector should match (scenario 8).

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