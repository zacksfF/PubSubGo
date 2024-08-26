package pubsub

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
	"github.com/zacksfF/PubSubGo/metrics"
	"github.com/zacksfF/PubSubGo/queue"
)

const (
	// DefaultMaxQueueSize is the default maximum size of queues
	DefaultMaxQueueSize = 1024 // ~8MB per queue (1000 * 4KB)

	// DefaultMaxPayloadSize is the default maximum payload size
	DefaultMaxPayloadSize = 8192 // 8KB

	// DefaultBufferLength is the default buffer length for subscriber chans
	DefaultBufferLength = 256

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// TODO: Make this configurable?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandlerFunc ...
type HandlerFunc func(msg *Message) error

// Topic ...
type Topic struct {
	Name     string    `json:"name"`
	Sequence uint64    `json:"seq"`
	Created  time.Time `json:"created"`
}

func (t *Topic) String() string {
	return t.Name
}

// Message ...
type Message struct {
	ID      uint64    `json:"id"`
	Topic   *Topic    `json:"topic"`
	Payload []byte    `json:"payload"`
	Created time.Time `json:"created"`
}

// ListenerOptions ...
type ListenerOptions struct {
	BufferLength int
}

// Listeners ...
type Listeners struct {
	sync.RWMutex

	buflen int

	ids map[string]bool
	chs map[string]chan Message
}

// NewListeners ...
func NewListeners(options *ListenerOptions) *Listeners {
	var (
		bufferLength int
	)

	if options != nil {
		bufferLength = options.BufferLength
	} else {
		bufferLength = DefaultBufferLength
	}

	return &Listeners{
		buflen: bufferLength,

		ids: make(map[string]bool),
		chs: make(map[string]chan Message),
	}
}

// Length ...
func (ls *Listeners) Length() int {
	ls.RLock()
	defer ls.RUnlock()

	return len(ls.ids)
}

// Add ...
func (ls *Listeners) Add(id string) chan Message {
	ls.Lock()
	defer ls.Unlock()

	ls.ids[id] = true
	ls.chs[id] = make(chan Message, ls.buflen)
	return ls.chs[id]
}

// Remove ...
func (ls *Listeners) Remove(id string) {
	ls.Lock()
	defer ls.Unlock()

	delete(ls.ids, id)

	close(ls.chs[id])
	delete(ls.chs, id)
}

// Exists ...
func (ls *Listeners) Exists(id string) bool {
	ls.RLock()
	defer ls.RUnlock()

	_, ok := ls.ids[id]
	return ok
}

// Get ...
func (ls *Listeners) Get(id string) (chan Message, bool) {
	ls.RLock()
	defer ls.RUnlock()

	ch, ok := ls.chs[id]
	if !ok {
		return nil, false
	}
	return ch, true
}

// NotifyAll ...
func (ls *Listeners) NotifyAll(message Message) int {
	ls.RLock()
	defer ls.RUnlock()

	i := 0
	for id, ch := range ls.chs {
		select {
		case ch <- message:
			i++
		default:
			// TODO: Drop this client?
			// TODO: Retry later?
			log.Warnf("cannot publish message to %s: %+v", id, message)
		}
	}

	return i
}

// Options ...
type Options struct {
	BufferLength   int
	MaxQueueSize   int
	MaxPayloadSize int
	WithMetrics    bool
}

// MessageBus ...
type MessageBus struct {
	sync.RWMutex

	metrics *metrics.Metrics

	bufferLength   int
	maxQueueSize   int
	maxPayloadSize int

	topics    map[string]*Topic
	queues    map[*Topic]*queue.Queue
	listeners map[*Topic]*Listeners
}

// New ...
func New(options *Options) *MessageBus {
	var (
		bufferLength   int
		maxQueueSize   int
		maxPayloadSize int
		withMetrics    bool
	)

	if options != nil {
		bufferLength = options.BufferLength
		maxQueueSize = options.MaxQueueSize
		maxPayloadSize = options.MaxPayloadSize
		withMetrics = options.WithMetrics
	} else {
		bufferLength = DefaultBufferLength
		maxQueueSize = DefaultMaxQueueSize
		maxPayloadSize = DefaultMaxPayloadSize
		withMetrics = false
	}

	var metrics *metrics.Metrics

	if withMetrics {
		metrics = metrics.NewMetrics("msgbus")

		ctime := time.Now()

		// server uptime counter
		metrics.NewCounterFunc(
			"server", "uptime",
			"Number of nanoseconds the server has been running",
			func() float64 {
				return float64(time.Since(ctime).Nanoseconds())
			},
		)

		// server requests counter
		metrics.NewCounter(
			"server", "requests",
			"Number of total requests processed",
		)

		// client latency summary
		metrics.NewSummary(
			"client", "latency_seconds",
			"Client latency in seconds",
		)

		// client errors counter
		metrics.NewCounter(
			"client", "errors",
			"Number of errors publishing messages to clients",
		)

		// bus messages counter
		metrics.NewCounter(
			"bus", "messages",
			"Number of total messages exchanged",
		)

		// bus dropped counter
		metrics.NewCounter(
			"bus", "dropped",
			"Number of messages dropped to subscribers",
		)

		// bus delivered counter
		metrics.NewCounter(
			"bus", "delivered",
			"Number of messages delivered to subscribers",
		)

		// bus fetched counter
		metrics.NewCounter(
			"bus", "fetched",
			"Number of messages fetched from clients",
		)

		// bus topics gauge
		metrics.NewCounter(
			"bus", "topics",
			"Number of active topics registered",
		)

		// queue len gauge vec
		metrics.NewGaugeVec(
			"queue", "len",
			"Queue length of each topic",
			[]string{"topic"},
		)

		// queue size gauge vec
		// TODO: Implement this gauge by somehow getting queue sizes per topic!
		metrics.NewGaugeVec(
			"queue", "size",
			"Queue length of each topic",
			[]string{"topic"},
		)

		// bus subscribers gauge
		metrics.NewGauge(
			"bus", "subscribers",
			"Number of active subscribers",
		)
	}

	return &MessageBus{
		metrics: metrics,

		bufferLength:   bufferLength,
		maxQueueSize:   maxQueueSize,
		maxPayloadSize: maxPayloadSize,

		topics:    make(map[string]*Topic),
		queues:    make(map[*Topic]*queue.Queue),
		listeners: make(map[*Topic]*Listeners),
	}
}

// Len ...
func (mb *MessageBus) Len() int {
	return len(mb.topics)
}

// Metrics ...
func (mb *MessageBus) Metrics() *metrics.Metrics {
	return mb.metrics
}

// NewTopic ...
func (mb *MessageBus) NewTopic(topic string) *Topic {
	mb.Lock()
	defer mb.Unlock()

	t, ok := mb.topics[topic]
	if !ok {
		t = &Topic{Name: topic, Created: time.Now()}
		mb.topics[topic] = t
		if mb.metrics != nil {
			mb.metrics.Counter("bus", "topics").Inc()
		}
	}
	return t
}

// NewMessage ...
func (mb *MessageBus) NewMessage(topic *Topic, payload []byte) Message {
	defer func() {
		topic.Sequence++
		if mb.metrics != nil {
			mb.metrics.Counter("bus", "messages").Inc()
		}
	}()

	return Message{
		ID:      topic.Sequence,
		Topic:   topic,
		Payload: payload,
		Created: time.Now(),
	}
}

// Put ...
func (mb *MessageBus) Put(message Message) {
	mb.Lock()
	defer mb.Unlock()

	t := message.Topic
	q, ok := mb.queues[t]
	if !ok {
		q = queue.NewQueue(mb.maxQueueSize)
		mb.queues[message.Topic] = q
	}
	q.Push(message)

	if mb.metrics != nil {
		mb.metrics.GaugeVec("queue", "len").WithLabelValues(t.Name).Inc()
	}

	mb.publish(message)
}

// Get ...
func (mb *MessageBus) Get(t *Topic) (Message, bool) {
	mb.RLock()
	defer mb.RUnlock()

	q, ok := mb.queues[t]
	if !ok {
		return Message{}, false
	}

	m := q.Pop()
	if m == nil {
		return Message{}, false
	}

	if mb.metrics != nil {
		mb.metrics.Counter("bus", "fetched").Inc()
		mb.metrics.GaugeVec("queue", "len").WithLabelValues(t.Name).Dec()
	}

	return m.(Message), true
}

// publish ...
func (mb *MessageBus) publish(message Message) {
	ls, ok := mb.listeners[message.Topic]
	if !ok {
		return
	}

	n := ls.NotifyAll(message)
	if n != ls.Length() && mb.metrics != nil {
		log.Warnf("%d/%d subscribers notified", n, ls.Length())
		mb.metrics.Counter("bus", "dropped").Add(float64(ls.Length() - n))
	}
}

// Subscribe ...
func (mb *MessageBus) Subscribe(id, topic string) chan Message {
	mb.Lock()
	defer mb.Unlock()

	t, ok := mb.topics[topic]
	if !ok {
		t = &Topic{Name: topic, Created: time.Now()}
		mb.topics[topic] = t
	}

	ls, ok := mb.listeners[t]
	if !ok {
		ls = NewListeners(&ListenerOptions{BufferLength: mb.bufferLength})
		mb.listeners[t] = ls
	}

	if ls.Exists(id) {
		// Already verified the listener exists
		ch, _ := ls.Get(id)
		return ch
	}

	if mb.metrics != nil {
		mb.metrics.Gauge("bus", "subscribers").Inc()
	}

	return ls.Add(id)
}

// Unsubscribe ...
func (mb *MessageBus) Unsubscribe(id, topic string) {
	mb.Lock()
	defer mb.Unlock()

	t, ok := mb.topics[topic]
	if !ok {
		return
	}

	ls, ok := mb.listeners[t]
	if !ok {
		return
	}

	if ls.Exists(id) {
		// Already verified the listener exists
		ls.Remove(id)

		if mb.metrics != nil {
			mb.metrics.Gauge("bus", "subscribers").Dec()
		}
	}
}

func (mb *MessageBus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if mb.metrics != nil {
			mb.metrics.Counter("server", "requests").Inc()
		}
	}()

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "GET" && (r.URL.Path == "/" || r.URL.Path == "") {
		// XXX: guard with a mutex?
		out, err := json.Marshal(mb.topics)
		if err != nil {
			msg := fmt.Sprintf("error serializing topics: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
		return
	}

	topic := strings.TrimLeft(r.URL.Path, "/")
	topic = strings.TrimRight(topic, "/")

	t := mb.NewTopic(topic)

	switch r.Method {
	case "POST", "PUT":
		if r.ContentLength > int64(mb.maxPayloadSize) {
			msg := "payload exceeds max-payload-size"
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := fmt.Sprintf("error reading payload: %s", err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if len(body) > mb.maxPayloadSize {
			msg := "payload exceeds max-payload-size"
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		}

		mb.Put(mb.NewMessage(t, body))

		w.WriteHeader(http.StatusAccepted)
	case "GET":
		if r.Header.Get("Upgrade") == "websocket" {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Errorf("error creating websocket client: %s", err)
				return
			}

			NewClient(conn, t, mb).Start()
			return
		}

		message, ok := mb.Get(t)

		if !ok {
			msg := fmt.Sprintf("no messages enqueued for topic: %s", topic)
			http.Error(w, msg, http.StatusNotFound)
			return
		}

		out, err := json.Marshal(message)
		if err != nil {
			msg := fmt.Sprintf("error serializing message: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	case "DELETE":
		http.Error(w, "Not Implemented", http.StatusNotImplemented)
		// TODO: Implement deleting topics
	}
}

// Client ...
type Client struct {
	conn  *websocket.Conn
	topic *Topic
	bus   *MessageBus

	id string
	ch chan Message
}

// NewClient ...
func NewClient(conn *websocket.Conn, topic *Topic, bus *MessageBus) *Client {
	return &Client{conn: conn, topic: topic, bus: bus}
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(message string) error {
		t, err := strconv.ParseInt(message, 10, 64)
		d := time.Duration(time.Now().UnixNano() - t)
		if err != nil {
			log.Warnf("garbage pong reply from %s: %s", c.id, err)
		} else {
			log.Debugf("pong latency of %s: %s", c.id, d)
		}
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if c.bus.metrics != nil {
			v := c.bus.metrics.Summary("client", "latency_seconds")
			v.Observe(d.Seconds())
		}

		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			c.bus.Unsubscribe(c.id, c.topic.Name)
			return
		}
		log.Debugf("recieved message from %s: %s", c.id, message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	var err error

	for {
		select {
		case msg, ok := <-c.ch:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The bus closed the channel.
				message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bus closed")
				c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(writeWait))
				return
			}

			err = c.conn.WriteJSON(msg)
			if err != nil {
				// TODO: Retry? Put the message back in the queue?
				log.Errorf("Error sending msg to %s: %s", c.id, err)
				if c.bus.metrics != nil {
					c.bus.metrics.Counter("client", "errors").Inc()
				}
			} else {
				if c.bus.metrics != nil {
					c.bus.metrics.Counter("bus", "delivered").Inc()
				}
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			t := time.Now()
			message := []byte(fmt.Sprintf("%d", t.UnixNano()))
			if err := c.conn.WriteMessage(websocket.PingMessage, message); err != nil {
				log.Errorf("error sending ping to %s: %s", c.id, err)
				return
			}
		}
	}
}

// Start ...
func (c *Client) Start() {
	c.id = c.conn.RemoteAddr().String()
	c.ch = c.bus.Subscribe(c.id, c.topic.Name)

	c.conn.SetCloseHandler(func(code int, text string) error {
		c.bus.Unsubscribe(c.id, c.topic.Name)
		message := websocket.FormatCloseMessage(code, text)
		c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(writeWait))
		return nil
	})

	go c.writePump()
	go c.readPump()
}
