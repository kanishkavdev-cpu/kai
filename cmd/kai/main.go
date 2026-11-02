package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kai",
		Short: "Helps in debugging your k8s cluster",
		Long:  "With the help of AI, kai helps debug Kubernetes cluster failures like CrashLoopBackOff and ImagePullBackOff.",
	}
}
