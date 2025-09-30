package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zacksfF/PubSubGo/cmd/cli/commands"
)

var (
	serverURL string
	verbose   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "pubsub",
		Short: "PubSubGo CLI - A command-line client for PubSubGo message broker",
		Long: `PubSubGo CLI allows you to interact with PubSubGo server to publish and subscribe to messages.
		
Examples:
  pubsub publish --topic events --message "Hello World"
  pubsub subscribe --topic events
  pubsub topics list`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "PubSubGo server URL")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	// Add commands
	rootCmd.AddCommand(commands.NewPublishCommand(&serverURL, &verbose))
	rootCmd.AddCommand(commands.NewSubscribeCommand(&serverURL, &verbose))
	rootCmd.AddCommand(commands.NewTopicsCommand(&serverURL, &verbose))
	rootCmd.AddCommand(commands.NewVersionCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// Helper function to pretty print JSON
func prettyPrint(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("%+v\n", data)
		return
	}
	fmt.Println(string(jsonData))
}
