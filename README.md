# PubSubGo
A lightweight, in-memory message bus library written in Golang that facilitates asynchronous communication between different components of an application using the publish-subscribe pattern. Designed for simplicity and efficiency.

## Features

* Simple HTTP API
* Simple command-line client
* In memory queues
* WebSockets for real-time messages
* Pull and Push model
* Concurrency Support
* Thread-Safe

##  Use Cases

- **Microservices Communication**: In a microservices architecture, msgbus can be used to facilitate internal communication between services or components.
- **Event-Driven Systems**: Applications that rely on event-driven architecture can use msgbus to propagate events between different modules.
- **Decoupling Components**: When you want to decouple components of a large application, msgbus can be a good choice to enable communication without tight coupling.

## How It Works

- **Publish**: Components publish messages to specific topics on the message bus.
- **Subscribe**: Other components subscribe to those topics and receive messages asynchronously when they are published.
- **Handling Messages**: Subscribers define handlers that process incoming messages in a way that suits the application’s needs.

## Install

```#!bash
$ go install git.mills.io/prologic/msgbus/...
```

## Use Cases


## Usage (library)


```#!go
package main

import (
    "log"

    "github.com/zacksfF/PubSubGo/pubsub"
)

func main() {
    m := pubsub.New()
    m.Put("foo", m.NewMessage([]byte("Hello World!")))

    msg, ok := m.Get("foo")
    if !ok {
        log.Printf("No more messages in queue: foo")
    } else {
        log.Printf
	    "Received message: id=%s topic=%s payload=%s",
	    msg.ID, msg.Topic, msg.Payload,
	)
    }
}
```

Running this example should yield something like this:

```#!bash
$ go run examples/main.go
2017/08/09 03:01:54 [pubsub] PUT id=0 topic=foo payload=Hello World!
2017/08/09 03:01:54 [pubsub] NotifyAll id=0 topic=foo payload=Hello World!
2017/08/09 03:01:54 [pubsub] GET topic=foo
2017/08/09 03:01:54 Received message: id=%!s(uint64=0) topic=foo payload=Hello World!
``` 

## Usage (tool)

Run the message bus daemon/server:

```#!bash
$ PubSubGo
2017/08/07 01:11:16 [msgbus] Subscribe id=[::1]:55341 topic=foo
2017/08/07 01:11:22 [msgbus] PUT id=0 topic=foo payload=hi
2017/08/07 01:11:22 [msgbus] NotifyAll id=0 topic=foo payload=hi
2017/08/07 01:11:26 [msgbus] PUT id=1 topic=foo payload=bye
2017/08/07 01:11:26 [msgbus] NotifyAll id=1 topic=foo payload=bye
2017/08/07 01:11:33 [msgbus] GET topic=foo
2017/08/07 01:11:33 [msgbus] GET topic=foo
2017/08/07 01:11:33 [msgbus] GET topic=foo
```

Subscribe to a topic using the message bus client:

```#!bash
$ PubSubGo sub foo
2017/08/07 01:11:22 [PubSubGo] received message: id=0 topic=foo payload=hi
2017/08/07 01:11:26 [PubSubGo] received message: id=1 topic=foo payload=bye
```


You can also manually pull messages using the client:

```#!bash
$ PubSubGo pull foo
2017/08/07 01:11:33 [PubSubGo] received message: id=0 topic=foo payload=hi
2017/08/07 01:11:33 [PubSubGo] received message: id=1 topic=foo payload=bye
```

> This is slightly different from a listening subscriber (*using websockets*) where messages are pulled directly.

## Usage (HTTP)

Run the message bus daemon/server:

```#!bash
$ PubSubGo
2018/03/25 13:21:18 PubSubGo listening on :8000
```

Send a message with using `curl`:

```#!bash
$ curl -q -o - -X PUT -d '{"message": "hello"}' http://localhost:8000/hello
```

Pull the messages off the "hello" queue using `curl`:

```#!bash
$ curl -q -o - http://localhost:8000/hello
{"id":0,"topic":{"name":"hello","ttl":60000000000,"seq":1,"created":"2018-03-25T13:18:38.732437-07:00"},"payload":"eyJtZXNzYWdlIjogImhlbGxvIn0=","created":"2018-03-25T13:18:38.732465-07:00"}
```

Decode the payload:

```#!bash
$ echo 'eyJtZXNzYWdlIjogImhlbGxvIn0=' | base64 -d
{"message": "hello"}
```

## API

### GET /

List all known topics/queues.

Example:

```#!bash
$ curl -q -o - http://localhost:8000/ | jq '.'
{
  "hello": {
    "name": "hello",
    "ttl": 60000000000,
    "seq": 1,
    "created": "2018-05-07T23:44:25.681392205-07:00"
  }
}
```

## POST|PUT /topic

Post a new message to the queue named by `<topic>`.

**NB:** Either `POST` or `PUT` methods can be used here.

Example:

```#!bash
$ curl -q -o - -X PUT -d '{"message": "hello"}' http://localhost:8000/hello
message successfully published to hello with sequence 1
```

## GET /topic

Get the next message of the queue named by `<topic>`.

- If the topic is not found. Returns: `404 Not Found`
- If the Websockets `Upgrade` header is found, upgrades to a websocket channel
  and subscribes to the topic `<topic>`. Each new message published to the
  topic `<topic>` are instantly published to all subscribers.

Example:

```#!bash
$ curl -q -o - http://localhost:8000/hello
{"id":0,"topic":{"name":"hello","ttl":60000000000,"seq":1,"created":"2018-03-25T13:18:38.732437-07:00"},"payload":"eyJtZXNzYWdlIjogImhlbGxvIn0=","created":"2018-03-25T13:18:38.732465-07:00"}
```

## DELETE /topic

Deletes a queue named by `<topic>`.


## License


