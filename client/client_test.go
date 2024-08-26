package client

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zacksfF/PubSubGo/pubsub"
)

func TestClientPublish(t *testing.T){
	assert := assert.New(t)

	mb := pubsub.New(nil)

	server := httptest.NewServer(mb)
	defer server.Close()

	client := NewClient(server.URL, nil)

	err := client.Publish("hello", "hello world")
	assert.NoError(err)

	topic := mb.NewTopic("hello")
	expected := pubsub.Message{Topic: topic, Payload: []byte("hello world")}

	actual, ok := mb.Get(topic)
	assert.True(ok)
	assert.Equal(actual.ID, expected.ID)
	assert.Equal(actual.Topic, expected.Topic)
	assert.Equal(actual.Payload, expected.Payload)
}
