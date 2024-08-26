package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zacksfF/PubSubGo/client"
)

// pullCmd represents the pub command
var pullCmd = &cobra.Command{
	Use:     "pull [flags] <topic>",
	Aliases: []string{"get"},
	Short:   "Pulls a message from a given topic",
	Long: `This pulls a message from the given topic if there are any messages
and prints the message to standard output. Otherwise if the queue for the
given topic is empty, this does nothing.

This is primarily useful in situations where a subscription was lost and you
want to "catch up" and pull any messages left in the queue for that topic.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		client := client.NewClient(uri, nil)

		topic := args[0]

		pull(client, topic)
	},
}

func init() {
	RootCMD.AddCommand(pullCmd)
}

func pull(client *client.Client, topic string) {
	if topic == "" {
		topic = defaultTopic
	}

	msg, err := client.Pull(topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading message: %s\n", err)
		os.Exit(2)
	}
	fmt.Printf("%s\n", msg.Payload)
}
