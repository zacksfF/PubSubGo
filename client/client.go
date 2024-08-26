package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/pubsub"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	// DefaultReconnectInterval ...
	DefaultReconnectInterval = 2

	// DefaultMaxReconnectInterval ...
	DefaultMaxReconnectInterval = 64

	// DefaultPingInterval is the default time interval between pings
	DefaultPingInterval = 60 * time.Second
)

func noopHandler(msg *pubsub.Message) error {
	return nil
}

// define the client
type Client struct {
	sync.RWMutex

	url string

	reconnectInterval    time.Duration
	maxReconnectInterval time.Duration
}

type Options struct {
	ReconnectInterval    int
	MaxReconnectInterval int
}

func NewClient(url string, options *Options) *Client {
	var (
		reconnectInterval    = DefaultReconnectInterval
		maxReconnectInterval = DefaultMaxReconnectInterval
	)

	url = strings.TrimSuffix(url, "/")

	client := &Client{url: url}

	if options != nil {
		if options.ReconnectInterval != 0 {
			reconnectInterval = options.ReconnectInterval
		}

		if options.MaxReconnectInterval != 0 {
			maxReconnectInterval = options.MaxReconnectInterval
		}
	}

	client.reconnectInterval = time.Duration(reconnectInterval) * time.Second
	client.maxReconnectInterval = time.Duration(maxReconnectInterval) * time.Second

	return client
}

func (c *Client) Pull(topic string) (msg *pubsub.Message, err error) {
	c.RLock()
	defer c.RUnlock()

	url := fmt.Sprintf("%s/%s", c.url, topic)
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		//EMpty queue
		return nil, nil
	}

	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Client) Publish(topic, message string) error {
	c.RLock()
	defer c.RUnlock()

	var payload bytes.Buffer

	payload.Write([]byte(message))

	url := fmt.Sprintf("%s/%s", c.url, topic)

	client := &http.Client{}

	req, err := http.NewRequest("PUT", url, &payload)
	if err != nil {
		return fmt.Errorf("error constructing request: %s", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error publishing message: %s", err)
	}

	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response: %s", res.Status)
	}
	return nil
}

func (c *Client) Subscribe(topic string, handler pubsub.HandlerFunc) *Subscriber {
	return NewSubscriber(c, topic, handler)
}

type Subscriber struct {
	sync.RWMutex

	conn *websocket.Conn

	client *Client

	topic   string
	handler pubsub.HandlerFunc

	url                  string
	reconnectInterval    time.Duration
	maxReconnectInterval time.Duration
}

func NewSubscriber(client *Client, topic string, handler pubsub.HandlerFunc) *Subscriber {
	if handler == nil {
		handler = noopHandler
	}
	u, err := url.Parse(client.url)
	if err != nil {
		log.Fatalf("invalid url: %s", client.url)
	}

	if strings.HasPrefix(client.url, "https") {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	u.Path += fmt.Sprintf("/%s", topic)

	url := u.String()

	return &Subscriber{
		client:  client,
		topic:   topic,
		handler: handler,

		url:                  url,
		reconnectInterval:    client.reconnectInterval,
		maxReconnectInterval: client.maxReconnectInterval,
	}
}

func (s *Subscriber) closeAndReconnect() {
	s.RLock()
	s.conn.Close(websocket.StatusNormalClosure, "Closing and reconnecting...")
	s.RUnlock()
	go s.connect()
}

func (s *Subscriber) connect() {
	s.RLock()
	b := &backoff.Backoff{
		Min:    s.reconnectInterval,
		Max:    s.maxReconnectInterval,
		Factor: 2,
		Jitter: false,
	}
	s.RUnlock()

	ctx := context.Background()

	for {
		conn, _, err := websocket.Dial(ctx, s.url, nil)
		if err != nil {
			time.Sleep(b.Duration())
			continue
		}

		s.Lock()
		s.conn = conn
		s.Unlock()

		go s.readLoop(ctx)
		go s.heartbeat(ctx, DefaultPingInterval)

		break
	}
}

func (s *Subscriber) readLoop(ctx context.Context) {
	var msg *pubsub.Message

	for {
		err := wsjson.Read(ctx, s.conn, &msg)
		if err != nil {
			s.closeAndReconnect()
			return
		}

		if err := s.handler(msg); err != nil {
			log.Warnf("error handling message: %s", err)
		}
	}
}

func (s *Subscriber) Start() {
	go s.connect()
}

// Stop ...
func (s *Subscriber) Stop() {
	s.Lock()
	defer s.Unlock()

	if err := s.conn.Close(websocket.StatusNormalClosure, "Subscriber stopped"); err != nil {
		log.Warnf("error sending close message: %s", err)

	}

	s.conn = nil
}

func (s *Subscriber) heartbeat(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		// c.Ping returns on receiving a pong
		err := s.conn.Ping(ctx)
		if err != nil {
			s.closeAndReconnect()
		}
		t.Reset(time.Minute)
	}
}
