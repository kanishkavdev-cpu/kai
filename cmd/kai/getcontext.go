package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/KanishkaVerma054/k8s-ai/internal/kube"
)

func newGetContextCmd(root *rootOptions) *cobra.Command {
	var (
		logLines int64
		timeout  time.Duration
	)
	cmd := &cobra.Command{
		Use:   "get-context deploy/<name>",
		Short: "Print everything kai knows about a Deployment as JSON",
		Long: "Collects the Deployment, its pods (status, container states, events, logs), " +
			"and the related Services, PVCs and Ingresses, and prints them as JSON. No LLM is used.",
		Example: "  kai get-context deploy/checkout-service -n crash-missing-env --context kind-kai-lab",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := parseDeployTarget(args[0])
			if err != nil {
				return err
			}

			contextName, ns, err := kube.ResolveTarget(root.kube)
			if err != nil {
				return err
			}
			if root.namespace != "" {
				ns = root.namespace
			}
			// Print the target on stderr, so stdout stays pure JSON.
			cmd.PrintErrf("kai: context %s, namespace %s\n", contextName, ns)

			cs, err := kube.NewClientset(root.kube)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			dc, err := kube.New(cs).GetContext(ctx, ns, name, logLines)
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(dc)
		},
	}
	cmd.Flags().Int64Var(&logLines, "log-lines", kube.DefaultLogLines, "maximum log lines per container")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "give up if the cluster doesn't answer in time")
	return cmd
}

// parseDeployTarget turns "deploy/checkout-service" into "checkout-service".
// It accepts the same spellings as kubectl: deploy, deployment, deployments.
func parseDeployTarget(arg string) (string, error) {
	kind, name, ok := strings.Cut(arg, "/")
	if !ok || name == "" {
		return "", fmt.Errorf("target %q: want deploy/<name>", arg)
	}
	switch strings.ToLower(kind) {
	case "deploy", "deployment", "deployments":
		return name, nil
	default:
		return "", fmt.Errorf("target %q: only deployments are supported, want deploy/<name>", arg)
	}
}
