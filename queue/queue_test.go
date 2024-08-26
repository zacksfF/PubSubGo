package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmpty(t *testing.T) {
	q := Queue{}
	assert.Zero(t, q.Len())
	assert.True(t, q.Empty())
}

func TestSimple(t *testing.T) {
	assert := assert.New(t)

	q := Queue{}

	for i := 0; i < minCapacity; i++ {
		q.Push(i)
	}

	for i := 0; i < minCapacity; i++ {
		assert.Equal(q.Peek(), i)
		assert.Equal(q.Pop(), i)
	}
}

func TestMaxLen(t *testing.T) {
	q := Queue{maxlen: minCapacity}
	assert.Equal(t, q.MaxLen(), minCapacity)
}

func TestFull(t *testing.T) {
	q := Queue{maxlen: minCapacity}

	for i := 0; i < minCapacity; i++ {
		q.Push(i)
	}

	assert.True(t, q.Full())
}

func TestBufferWrap(t *testing.T) {
	q := Queue{}

	for i := 0; i < minCapacity; i++ {
		q.Push(i)
	}

	for i := 0; i < 3; i++ {
		q.Pop()
		q.Push(minCapacity + i)
	}

	for i := 0; i < minCapacity; i++ {
		assert.Equal(t, q.Peek().(int), i+3)
		q.Pop()
	}
}

func TestLen(t *testing.T) {
	assert := assert.New(t)

	q := Queue{}
	assert.Zero(q.Len())

	for i := 0; i < 1000; i++ {
		q.Push(i)
		assert.Equal(q.Len(), i+1)
	}

	for i := 0; i < 1000; i++ {
		q.Pop()
		assert.Equal(q.Len(), 1000-i-1)
	}
}

func BenchmarkPush(b *testing.B) {
	q := Queue{}
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
}

func BenchmarkPushPop(b *testing.B) {
	q := Queue{}
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
	for i := 0; i < b.N; i++ {
		q.Pop()
	}
}
