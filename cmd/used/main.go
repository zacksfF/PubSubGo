package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmcloughlin/professor"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/zacksfF/PubSubGo/pubsub"
	"github.com/zacksfF/PubSubGo/version"
)

const (
	helpText = `
PubSubGo is a self-hosted pub/sub server for publishing
artbitrary messages onto queues (topics) and subscribing to them with
a client such as curl or in real-time with websockets (using PubSubGo).

Valid optinos:
`
)

var (
	versions bool
	debug   bool
	bind    string

	bufferLength    int
	maxQueueSize    int
	maxPlayloadSize int
)

func init() {
	baseProg := filepath.Base(os.Args[0])
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", baseProg)
		fmt.Fprintf(os.Stderr, helpText)
		flag.PrintDefaults()
	}

	flag.BoolVarP(&debug, "debug", "D", false, "enable debug logging")
	flag.StringVarP(&bind, "bind", "b", "0.0.0.0:8000", "[int]:<port> to bind to")
	flag.BoolVarP(&versions, "version", "v", false, "display version information")

	// Basic options
	flag.IntVarP(
		&bufferLength, "buffer-length", "B", pubsub.DefaultBufferLength,
		"set the buffer length for subscribers before messages are dropped",
	)
	flag.IntVarP(
		&maxQueueSize, "max-queue-size", "Q", pubsub.DefaultMaxQueueSize,
		"set the maximum queue size per topic",
	)
	flag.IntVarP(
		&maxPlayloadSize, "max-payload-size", "P", pubsub.DefaultMaxPayloadSize,
		"set the maximum payload size per message",
	)
}

func flagNameFromEnvironmentName(s string) string {
	s = strings.ToLower(s)
	s = strings.Replace(s, "_", "-", -1)
	return s
}

func parseArgs() error {
	for _, v := range os.Environ() {
		vals := strings.SplitN(v, "=", 2)
		flagName := flagNameFromEnvironmentName(vals[0])
		fn := flag.CommandLine.Lookup(flagName)
		if fn == nil || fn.Changed {
			continue
		}
		if err := fn.Value.Set(vals[1]); err != nil {
			return err
		}
	}
	flag.Parse()
	return nil
}

func main() {
	parseArgs()

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if versions {
		fmt.Printf("msgbusd %s", version.FullVersion())
		os.Exit(0)
	}

	if debug {
		go professor.Launch(":6060")
	}

	opts := pubsub.Options{
		BufferLength:   bufferLength,
		MaxQueueSize:   maxQueueSize,
		MaxPayloadSize: maxPlayloadSize,
		WithMetrics:    true,
	}
	mb := pubsub.New(&opts)

	http.Handle("/", mb)
	http.Handle("/metrics", mb.Metrics().Handler())
	log.Infof("PubSub %s listening on %s", version.FullVersion(), bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}
