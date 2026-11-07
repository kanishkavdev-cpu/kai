package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/KanishkaVerma054/k8s-ai/internal/kube"
)

// rootOptions holds the flags shared by every subcommand.
type rootOptions struct {
	kube      kube.Options
	namespace string
}

func main() {
	// Ctrl+C cancels ctx, which stops any API call in progress.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		stop()
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}
	cmd := &cobra.Command{
		Use:   "kai",
		Short: "Helps in debugging your k8s cluster",
		Long:  "With the help of AI, kai helps debug Kubernetes cluster failures like CrashLoopBackOff and ImagePullBackOff.",
		// Errors from RunE are printed once by cobra; don't also print the usage.
		SilenceUsage: true,
	}

	f := cmd.PersistentFlags()
	f.StringVar(&opts.kube.Kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default $KUBECONFIG or ~/.kube/config)")
	f.StringVar(&opts.kube.Context, "context", "", "kubeconfig context to use (default the current context)")
	f.StringVarP(&opts.namespace, "namespace", "n", "", "namespace (default the context's namespace)")

	cmd.AddCommand(newGetContextCmd(opts))
	return cmd
}
