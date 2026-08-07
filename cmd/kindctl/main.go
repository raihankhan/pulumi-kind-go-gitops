package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kindctl <create|delete|status>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(action string) error {
	name := getenv("KIND_CLUSTER_NAME", "pulumi-gitops")
	config := "kind/kind-config.yaml"
	args := []string{"cluster", action, "--name", name}
	if action == "create" {
		args = append(args, "--config", config)
	}
	cmd := exec.Command("kind", args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
