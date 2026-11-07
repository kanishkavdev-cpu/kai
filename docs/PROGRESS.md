# kai: Progress and Learning Journal

What I built, how I built it, and what I learned. One entry per task, plus a summary per day.

## Concepts I've learned
### Go
- The module path is the prefix of every import, and `go install` uses it to find the code
- The compiler blocks other projects from importing anything under `internal/`
- The `go` line in `go.mod` sets the minimum version, not the compiler used, and dependencies can raise it
- `package main` with `func main()` builds a program; any other package is a library
- `main()` can't return an error, so failure is reported with `os.Exit(1)`
- `go.sum` stores a checksum for every dependency; `go mod tidy` fills it in once my code imports a package, and removes `// indirect` from modules I import directly
- An unused import or an unused variable is a compile error in Go, not a warning
- Read compiler errors as `file:line:column: message`; `have (...)` / `want (...)` compares what I passed with what the function takes
- A function's arguments and a struct's fields are different things: `NewDefaultClientConfigLoadingRules()` takes no arguments, and I set `ExplicitPath` on the struct it returns
- Use keyed struct literals (`ConfigOverrides{CurrentContext: ctx}`); positional ones fill fields in order and must fill every field
- `fmt.Errorf` only builds an error; I must `return` it, or the function carries on with broken values
- Every `return` must give every value in the signature: on error return `nil` plus the error, and on success the value plus `nil`. The caller checks `err` first and never uses the other value when it isn't `nil`
- `%w` wraps an error so the message keeps the real cause and `errors.Is` still works
- An interface is a list of methods, and any type with those methods satisfies it. "Accept interfaces, return structs": `NewClient` returns `*kubernetes.Clientset`, and the functions that use it will take `kubernetes.Interface` so tests can pass a fake
- Every package is named by the last part of its path, so four `.../v1` imports clash; give each an alias like `corev1 "k8s.io/api/core/v1"`
- A package never imports itself: code in the same package already sees everything, so call `New(...)`, not `kube.New(...)`. Go forbids import cycles
- In a method `func (c *Client) ...`, `c` is the receiver, the value the method was called on, and it already holds what the method needs (`c.cs`)
- Create the client once at the edge of the program (`main`) and pass it in (dependency injection), so `--context` is respected and tests can pass a fake
- Uppercase names are exported (usable from other packages), lowercase names are private: `New` and `String()` are public, the `cs` field is not
- Go makes me receive every return value: `sel, err := ...`, not `sel := ...`
- `:=` needs at least one new variable on the left; otherwise it reuses the existing ones, which can overwrite a parameter by accident
- Count the `%` verbs: each one takes exactly one value, in order. Format strings hold the fixed words, and values that change go in the `%s` slots
- Error messages start lowercase and name the step that failed, because they get joined (`get context: list pods for deployment ns/name: ...`)
- After a syntax error, fix only the first error and rebuild; the rest are usually caused by it
- Empty braces (`metav1.ListOptions{}`) give every field its zero value, which means "no filter"
- `go.mod` marks a module direct once my code imports it; `k8s.io/api` and `apimachinery` lost `// indirect` when `resources.go` imported them
- Short, general names (`list`, `err`) suit short scopes; use the same name for the same role in every function
- A pure function (no API calls, no `ctx`, no `error`) like `SummarizePod` is the easiest kind of code to test: build the input by hand and compare the output
- A pointer field means "may be absent": `LastTermination *StateSummary` is nil when the container never terminated, which a plain struct can't express
- JSON struct tags (`json:"restart_count,omitempty"`) set the field names in the output; `omitempty` drops empty fields so the JSON stays short
- Struct embedding (`PodContext` contains `PodSummary` with no field name) promotes the inner fields, and `encoding/json` flattens them into one object
- A sentinel error (`var ErrLogsUnavailable = errors.New(...)`) wrapped with `%w` lets callers check `errors.Is(err, kube.ErrLogsUnavailable)`
- `context.WithTimeout` puts a deadline on every API call made with that ctx; `signal.NotifyContext` cancels it on Ctrl+C
- Write data to stdout and messages to stderr, so `kai get-context ... | jq` gets pure JSON
- Table-driven tests: a slice of cases, one `t.Run` each. `fake.NewClientset(objects...)` gives an in-memory cluster for tests
- Duplication is only a problem when the same *decision* is copied, so one change must be made in many places. My four `List` functions look alike but each calls a different API and returns a different type, so they stay separate ("a little copying is better than a little dependency")
- Generics (`func f[T any](...)`) could merge them, but a type parameter can't read a field like `.Items`, so each caller would still need an adapter. The merged code would be longer and harder to read. Generics pay off when the shared logic is substantial (sorting, filtering, retries)
- Rule of three: wait until the same logic appears three times and is clearly the same before abstracting. If every `List` later needs the same retry and RBAC handling (Day 6), that shared code is worth a helper
### Reading library source
- Downloaded modules live at the exact `go.mod` version: `go list -m -f '{{.Dir}}' k8s.io/client-go`
- Ways in: F12 (Go to Definition) and Ctrl+F12 (Go to Implementations) in VS Code, `go doc` and `go doc -src`, `grep -rn` in the module folder, pkg.go.dev
- Start with one question, read the doc comment and then the code, go one level deep, check the tests and examples, and stop once I can point to the file and line that answers it
- client-go ignores an empty `ExplicitPath` (`loader.go:208`), and `DeferredLoadingClientConfig.ClientConfig()` already falls back to in-cluster config when no kubeconfig is found (`merged_client_builder.go`)
### Interview notes (things worth being able to explain)
- **"Walk me through kai's architecture."** Day 1 is the read-only data layer: `NewClientset` (kubeconfig or in-cluster) → `kube.Client` wrapping a `kubernetes.Interface` → small functions that each fetch one thing → `GetContext` that combines them into one JSON document. The analyzers and the agent sit on top of it and never call client-go directly.
- **"Why `kubernetes.Interface` and not `*Clientset`?"** "Accept interfaces, return structs": the constructor returns the concrete type, and everything that uses it takes the interface, so tests pass `fake.NewClientset(...)` and need no cluster.
- **"How do you avoid talking to the wrong cluster?"** `--context` on every command, the same loader for the client and the printed context, and kai prints `context X, namespace Y` before doing anything. I saw the danger myself: my current context switched to `kind-kai` without me noticing.
- **"Why summaries instead of the raw objects?"** A pod object has hundreds of fields. The phase says `Running` for a crashlooping pod; the cause is in `lastState.terminated` (exit 137 = OOMKilled), events or logs. Summaries keep the evidence and cut the tokens an LLM would spend.
- **"How do you handle a nil pointer in Kubernetes types?"** `ContainerState` has three pointers and only one is set; `LastTerminationState.Terminated` is nil if the container never ran (CreateContainerConfigError). Check every pointer before reading it, and use a pointer in my own struct (`LastTermination *StateSummary`) so "never terminated" is different from "exited 0".
- **"What happens when part of the data can't be read?"** Fail on what makes the answer meaningless (the Deployment doesn't exist); record everything else in `errors` and keep going. Logs that can't be read become `previous_error`, not a crash.
- **"Why cap logs?"** One chatty container could produce megabytes. Both lines (`TailLines`) and bytes (`LimitBytes`) are capped on the server side, so the data never even crosses the network.
- **"Tell me about a tricky bug."** `kubectl logs --previous` sometimes returned "unable to retrieve container logs" as the log text with HTTP success. It happens right after a crash, when "previous" points to a container the kubelet already deleted. I turned it into a sentinel error (`ErrLogsUnavailable`) callers can check with `errors.Is`.
- **"Why server-side filtering?"** Label selectors (pods of a Deployment) and field selectors (events for one object) make the API server do the filtering, so kai doesn't download a whole namespace. The fake clientset ignores field selectors, so the code double-checks in Go, and that's covered by a test.
- **"Why not merge the four List functions with generics?"** They look alike but call different APIs and return different types; a type parameter can't read `.Items`, so a generic version would be longer and harder to read. Rule of three, and "a little copying is better than a little dependency".
- **"Why deterministic data collection before the LLM?"** It's cheap, testable, and repeatable. Seven of the eight scenarios are fully explained by one field or one event, with no model at all. The LLM is for explaining and for the ambiguous cases.

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
- Inside a pod, Kubernetes sets `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT` and mounts a service-account token at `/var/run/secrets/kubernetes.io/serviceaccount/token`; that's how client-go knows it's in-cluster
- kai loads the kubeconfig first and falls back to in-cluster config, like kubectl; `--context` (empty means the current context) picks the cluster explicitly
- A Deployment doesn't list its pods; its `Spec.Selector` says which labels they carry, and `metav1.LabelSelectorAsSelector(...).String()` turns it into `app=checkout-service` for `ListOptions.LabelSelector`
- A Service's selector points at pods, not the other way round, so kai lists all Services in the namespace and checks which should match
- A pod's phase is coarse: a crashlooping pod still shows `phase=Running`. The real reason is in the container statuses
- For a missing namespace, the API server just says the object is `not found`, so kai's error prefix (`ns/name`) is what shows where it looked
- Field selectors filter on fields, not labels: `involvedObject.kind=Pod,involvedObject.name=...` returns only one object's events
- Events have several timestamp fields, filled in differently by different components (`series.lastObservedTime`, `lastTimestamp`, `eventTime`, `firstTimestamp`), so pick the first one that is set
- The same failure is often recorded as many separate Event objects; merge them by type, reason, message and object, and add up the counts
- Each scenario's evidence lives in a different place: logs (1), events (2, 3, 5, 6), last termination plus the memory limit (4), the probe port vs the container port (5), the PVC's events (7), and the Service selector vs pod labels (8)
- API groups match the YAML `apiVersion`: Pods, Services, PVCs and Events are `CoreV1()` (`v1`), Deployments are `AppsV1()` (`apps/v1`), Ingresses are `NetworkingV1()` (`networking.k8s.io/v1`)

## Open questions
- *(none right now)*

### Answered
- **Why does `kubectl logs --previous` sometimes return `unable to retrieve container logs`?** For a crashlooping container, "previous" means the run before the current one. Right after a crash, the *current* container is the one that just died, so "previous" points to the run before that, which the kubelet has already deleted (it keeps only the most recent dead container). A moment later the container is `waiting` again and "previous" is the run that just crashed, so a retry works. The kubelet sends the message as log text with a success status, not as an error, so kai checks for it and returns `ErrLogsUnavailable`.

---

## Day 1: Lab cluster and read-only Kubernetes access
**Status:** done

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

### Task: `internal/kube`: kubeconfig loading, and get pods, deployments, services, PVCs, ingresses, events
*(Drafted by Claude at my request. I wrote the functions; Claude fixed the import aliases, the self-import and the `GetDeployment` body when I asked, and renamed the `list` variables.)*
- **What I built:**
  - `internal/kube/client.go`: the `Options` struct (`Kubeconfig`, `Context`) and `NewClientset(opts)`, which returns a `*kubernetes.Clientset`.
  - `internal/kube/resources.go`: the `Client` struct holding a `kubernetes.Interface`, `New(cs)`, `GetDeployment`, `ListPodsForDeployment`, `ListServices`, `ListPVCs`, `ListIngresses` and `ListEvents`.
- **How it works:** `NewClientset` builds loading rules (with `ExplicitPath` set only if a path was given) and overrides (with `CurrentContext`), and client-go reads the kubeconfig or falls back to in-cluster config. `New` wraps that clientset, or a fake one in tests, in a `Client`, and every method reaches the API through `c.cs`. `ListPodsForDeployment` turns the Deployment's `Spec.Selector` into a string such as `app=checkout-service` and lists only the pods with those labels, so a stuck rollout returns all of its pods. Every method takes `ctx` first, wraps errors with the namespace and name, and returns `.Items` from the list.
- **Go concepts used:** packages and import aliases, structs and methods with a receiver, interfaces ("accept interfaces, return structs"), keyed struct literals, multiple return values, error wrapping with `%w`, exported vs unexported names, `context.Context`.
- **Agent concepts used:** these functions are kai's read-only view of the cluster. Tested against `kai-lab`, they already return the root-cause evidence for scenario 3 (image pull events), 7 (PVC asks for `fast-ssd`) and 8 (Service selects `app=checkout`).
- **Problems I hit and how I solved them:**
  - `missing go.sum entry` errors everywhere: nothing had imported client-go before, so `go.sum` had no checksums for its dependencies. `go mod tidy` fixed it.
  - `too many arguments` for `NewDefaultClientConfigLoadingRules(path)`: it takes no arguments, and `ExplicitPath` is a field I set on the struct it returns.
  - `cannot use opts.Context as api.AuthInfo`: a positional struct literal filled the first field. A keyed literal (`CurrentContext: opts.Context`) fixed it.
  - `fmt.Errorf` without `return`: the error was built and thrown away.
  - `v1 redeclared`: four packages are all named `v1`, so each import needs an alias (`appsv1`, `corev1`, `networkingv1`, `metav1`).
  - `import cycle not allowed`: I put the caller's code (`kube.NewClientset`, `kube.New`) inside a method. Inside the package, the client is already available as `c.cs`.
  - Reusing `d` for the selector overwrote the Deployment; a new variable `sel` fixed it.
  - `assignment mismatch: 1 variable but ... returns 2 values`: Go makes me take every return value, including the error.
  - `sel.string()` vs `String()`: lowercase names are private, so the method is `String()`.
  - `fmt.Errorf call has arguments but no formatting directives` (from `go vet`): each `%` verb needs exactly one value.
  - A syntax error in `ListOptions{ListServices: }` produced 11 errors; only the first one was real.
  - The file looked unchanged because I hadn't saved it in VS Code.
- **Review feedback I applied:**
  - Renamed `NewClient` to `NewClientset` so it doesn't clash with `New`.
  - Each error message names the step that failed (`parse selector` vs `list pods`) and uses `ns/name` with `%s/%s`.
  - Removed a hard-coded namespace from a format string.
  - Lowercase error messages, no trailing spaces, gofmt clean.
  - Named every list variable `list`, so all five functions read the same.

### Task: describe-equivalent summaries, logs, events for one object, and `kai get-context`
*(Written by Claude at my request at the end of the day, to finish Day 1. I need to read through this code before Day 2 so I can explain it.)*
- **What I built:**
  - `internal/kube/summary.go`: `PodSummary`, `ContainerSummary`, `StateSummary`, `ConditionSummary`, `SummarizePod` and `summarizeState`.
  - `internal/kube/logs.go`: `GetLogs` (current or `--previous`, capped at 50 lines and 16 KB) and the sentinel error `ErrLogsUnavailable`.
  - `internal/kube/events.go`: `ListEventsFor(kind, name)`, which filters with a field selector, sorts newest first and merges duplicates.
  - `internal/kube/context.go`: `GetContext`, which builds a `DeploymentContext`: the Deployment (replicas, conditions, container spec with resources, ports, probes and ConfigMap/Secret references, events), each pod (summary, events, logs), Services (with `selects_deployment_pods`), the PVCs the pods mount (with their events) and Ingresses.
  - `internal/kube/client.go`: `ResolveTarget` returns the context name and default namespace; a shared `loader` keeps it in step with `NewClientset`.
  - `cmd/kai/main.go`: global flags `--context`, `--kubeconfig`, `-n`; Ctrl+C cancels the context.
  - `cmd/kai/getcontext.go`: `kai get-context deploy/<name>` with `--log-lines` and `--timeout`.
  - Tests: `summary_test.go`, `context_test.go` (fake clientset), `getcontext_test.go`.
- **How it works:** `get-context` parses `deploy/<name>`, prints the context and namespace to stderr, builds the client, and calls `GetContext` with a 30-second timeout. Only a missing Deployment is fatal; any other part that fails goes into `errors` and the rest is still printed. `SummarizePod` is a pure function that doesn't call the API, so it's tested with hand-built pods. Every pointer (`Waiting`, `Running`, `Terminated`, `LastTerminationState.Terminated`, probes) is checked before it's read. The JSON goes to stdout and the context line to stderr, so the output can be piped to `jq`.
- **Go concepts used:** struct design with JSON tags and `omitempty`, pointer fields meaning "may be absent", nil checks, `switch` with no condition, `for ... range` and `append`, struct embedding (`PodContext` embeds `PodSummary`), closures (`addErr`), maps as set keys (`mergeDuplicateEvents`), `slices.SortStableFunc`, sentinel errors with `errors.New` and `%w`, `context.WithTimeout`, `signal.NotifyContext`, cobra subcommands and persistent flags, table-driven tests and `client-go/kubernetes/fake`.
- **Agent concepts used:** this JSON is what the analyzers (Day 2) and the model (Day 3) will read. Summaries keep only the evidence and drop the noise, logs are capped by lines and bytes, and duplicate events are merged, because every extra byte costs tokens and can bury the real cause. Partial failures are reported, not fatal, so the agent still gets what could be read (this matters with RBAC on Day 4).
- **Problems hit and how they were solved:**
  - The same event appeared up to 9 times (PVC `ProvisioningFailed`). These are separate Event objects with the same text, so `mergeDuplicateEvents` folds them into one and adds up the counts (562 in total).
  - Scenario 1's previous log came back as the text `unable to retrieve container logs ...`. The kubelet returns this with a success status. `GetLogs` now turns it into `ErrLogsUnavailable` (see "Answered" above).
  - The fake clientset ignores field selectors, so `ListEventsFor` also checks `InvolvedObject` in Go.
  - A test expected timestamps rounded to seconds, but the fake keeps full precision (only the real API server rounds).
  - `golangci-lint` (errcheck) flagged an unchecked `fmt.Fprintf`; switched to cobra's `cmd.PrintErrf`.
- **Review feedback applied:** none yet. I'll review this code myself before Day 2.

**Day summary**
- "Done when" result: **passed.** `make lab` reported every scenario `unchanged` (all 8 were already running), and `kai get-context deploy/checkout-service -n <scenario> --context kind-kai-lab` shows the real failure reason for each one, without an LLM:
  ```
  crash-missing-env          FATAL: DATABASE_URL is not set                         (previous log)
  crash-missing-config-key   Error: couldn't find key user in ConfigMap crash-missing-config-key/checkout-service
  imagepull-bad-tag          Failed to pull image "busybox:1.36-typo": rpc error: code = NotFound
  oomkilled-low-memory       OOMKilled exit 137, limit 32Mi                         (last_termination + spec)
  liveness-wrong-port        Liveness probe failed: Get "http://10.244.0.7:9090/" ... (spec: probe :9090, port 8080)
  pending-too-large          0/1 nodes are available: 1 Insufficient cpu, 1 Insufficient memory
  pvc-missing-storageclass   storageclass.storage.k8s.io "fast-ssd" not found         (PVC event)
  service-selector-mismatch  service selector {app: checkout} selects_deployment_pods=false
  ```
  `go vet`, `golangci-lint` (0 issues) and `go test ./...` (8 tests in `internal/kube`, 1 table test in `cmd/kai`) all pass.
- What I learned: see "Concepts I've learned" and "Interview notes" above. The biggest lesson: each failure leaves its evidence somewhere different (logs, events, last termination, the spec, a related PVC or Service), so a troubleshooting tool has to collect all of it.
- Still unclear: I didn't write the last part of Day 1 myself (summaries, logs, events, `get-context`). I need to read `summary.go`, `events.go`, `logs.go` and `context.go` and be able to explain them.
- Scope cuts:
  - Init containers and ReplicaSet events aren't collected yet.
  - EndpointSlices aren't read; `selects_deployment_pods` is computed from labels instead.
  - The default namespace comes from the kubeconfig context; there's no `--all-namespaces`.
  - No unit test for `ErrLogsUnavailable`, because the fake clientset always returns `fake logs`.

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