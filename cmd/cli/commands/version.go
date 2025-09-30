package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version and build information for PubSubGo CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("PubSubGo CLI v1.0.0\n")
			fmt.Printf("A command-line client for PubSubGo message broker\n")
			fmt.Printf("Build: %s\n", getBuildInfo())
		},
	}
}

func getBuildInfo() string {
	// This could be set during build time with -ldflags
	return "development"
}