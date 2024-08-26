package queue

import (
	"sync"
)

// minCapacity is the smallest capacity that queue may have.
// Must be power of 2 for bitwise modulus: x % n == x & (n - 1).
const minCapacity = 2

// Queue represents a single instance of a bounded queue data structure
// with access to both side. If maxlen is non-zero the queue is bounded
// otherwise unbounded.
type Queue struct {
	sync.RWMutex

	buf    []interface{}
	head   int
	tail   int
	count  int
	maxlen int
}

// NewQueue creates a new instance of Queue with the provided maxlen
func NewQueue(maxlen ...int) *Queue {
	if len(maxlen) > 0 {
		return &Queue{
			maxlen: maxlen[0],
		}
	}
	return &Queue{}
}

// Len returns the number of element currently stored in the queue.
func (q *Queue) Len() int {
	q.RLock()
	defer q.RUnlock()

	return q.count
}

// MaxLen returns the maxlen of the queue
func (q *Queue) MaxLen() int {
	q.RLock()
	defer q.RUnlock()

	return q.maxlen
}

// Size returns teh current size of the queue
func (q *Queue) Size() int {
	q.RLock()
	defer q.RUnlock()

	return len(q.buf)
}

// Empty returns true if the queue is empty false otherwise
func (q *Queue) Empty() bool {
	q.RLock()
	defer q.RUnlock()

	return q.count == 0
}

// Full returns true if the queue is full false otherwise
func (q *Queue) Full() bool {
	q.RLock()
	defer q.RUnlock()

	return q.count == q.maxlen
}

// Push appends an element to the back of the queue.
func (q *Queue) Push(elem interface{}) {
	q.Lock()
	defer q.Unlock()

	q.growIfFull()

	q.buf[q.tail] = elem
	// Calculate new tail position.
	q.tail = q.next(q.tail)
	q.count++
}

// Pop removes and returns the element from the front of the queue.
func (q *Queue) Pop() interface{} {
	q.Lock()
	defer q.Unlock()

	if q.count <= 0 {
		return nil
	}

	ret := q.buf[q.head]
	q.buf[q.head] = nil

	//calculate new head position
	q.head = q.next(q.head)
	q.count--

	q.shrinkIfExcess()
	return ret
}

// Peek returns the element at the front of the queue.
func (q *Queue) Peek() interface{} {
	q.RLock()
	defer q.RUnlock()

	if q.count <= 0 {
		return nil
	}
	return q.buf[q.head]
}

// next returns the next buffer position wrapping around buffer.
func (q *Queue) next(i int) int {
	return (i + 1) & (len(q.buf) - 1) // bitwise modulus
}

// growIfFull resizes up if the buffer is full.
func (q *Queue) growIfFull() {
	if len(q.buf) == 0 {
		q.buf = make([]interface{}, minCapacity)
		return
	}
	if q.count == len(q.buf) && q.count < q.maxlen {
		q.resize()
	}
}

// shrinkIfExcess resize down if the buffer 1/4 full.
func (q *Queue) shrinkIfExcess() {
	if len(q.buf) > minCapacity && (q.count<<2) == len(q.buf) {
		q.resize()
	}
}

// resize resizes the queue to fit exactly twice its current contents.
// This results in shrinking if the queue is less than half-full, or growing
// the queue when it is full.
func (q *Queue) resize() {
	newBuf := make([]interface{}, q.count<<1)
	if q.tail > q.head {
		copy(newBuf, q.buf[q.head:q.tail])
	} else {
		n := copy(newBuf, q.buf[q.head:])
		copy(newBuf[n:], q.buf[:q.tail])
	}

	q.head = 0
	q.tail = q.count
	q.buf = newBuf
}
