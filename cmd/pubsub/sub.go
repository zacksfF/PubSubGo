package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zacksfF/PubSubGo/client"
	"github.com/zacksfF/PubSubGo/pubsub"
)

// subCmd represents the pub command
var subCmd = &cobra.Command{
	Use:     "sub [flags] <topic> [<command>] [<arg> ...]",
	Aliases: []string{"reg"},
	Short:   "Subscribe to a topic",
	Long: `This subscribes to the given topic and for every message published
to the topic, the message is printed to standard output (default) or the
supplied command is executed with the contents of the message as stdin.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		client := client.NewClient(uri, nil)

		topic := args[0]

		var (
			command string
		)

		if len(args) > 1 {
			command = args[1]
			args = args[2:]
		}

		subscribe(client, topic, command, args)
	},
}

func init() {
	RootCMD.AddCommand(subCmd)
}

func handler(command string, args []string) pubsub.HandlerFunc {
	return func(msg *pubsub.Message) error {
		out, err := json.Marshal(msg)
		if err != nil {
			log.Printf("error marshalling message: %s", err)
			return err
		}

		if command == "" {
			os.Stdout.Write(out)
			os.Stdout.Write([]byte{'\r', '\n'})
			return nil
		}

		cmd := exec.Command(command, args...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Printf("error connecting to stdin of %s: %s", command, err)
			return err
		}

		go func() {
			defer stdin.Close()
			stdin.Write(out)
			stdin.Write([]byte{'\r', '\n'})
		}()

		stdout, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("error running %s: %s", command, err)
			return err
		}
		fmt.Print(string(stdout))

		return nil
	}
}

func subscribe(client *client.Client, topic, command string, args []string) {
	if topic == "" {
		topic = defaultTopic
	}

	s := client.Subscribe(topic, handler(command, args))
	s.Start()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("caught signal %s: ", sig)
		s.Stop()
		done <- true
	}()

	<-done
}
