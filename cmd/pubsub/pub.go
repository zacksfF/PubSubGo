package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zacksfF/PubSubGo/client"
)

// pubCmd represents the pub command
var pubCmd = &cobra.Command{
	Use:     "pub [flags] <topic> [<message>|-]",
	Aliases: []string{"put"},
	Short:   "Publish a new message",
	Long: `This publishes a new message either from positional command-line
arguments or from standard input if - is used as the first and only argument.

This is an asynchronous operation and does not wait for a response unless the
-w/--wait option is also present.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		client := client.NewClient(uri, nil)

		topic := args[0]

		message := ""
		if len(args) == 2 {
			message = args[1]
		}

		publish(client, topic, message)
	},
}

func init() {
	RootCMD.AddCommand(pubCmd)

	pubCmd.Flags().BoolP(
		"wait", "w", false,
		"Waits for a response and prints it before terminating",
	)
}

const defaultTopic = "hello"

func publish(client *client.Client, topic, message string) {
	if topic == "" {
		topic = defaultTopic
	}

	if message == "" || message == "-" {
		buf, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("error reading message from stdin: %s", err)
		}
		message = string(buf[:])
	}

	err := client.Publish(topic, message)
	if err != nil {
		log.Fatalf("error publishing message: %s", err)
	}
}
