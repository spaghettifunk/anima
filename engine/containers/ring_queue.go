package containers

import "errors"

type RingQueue struct {
	data       []interface{}
	size       int
	readIndex  int
	writeIndex int
	count      int
}

// Create a new RingQueue
func NewRingQueue(size int) *RingQueue {
	return &RingQueue{
		data: make([]interface{}, size),
		size: size,
	}
}

// Enqueue adds an element to the queue
func (rq *RingQueue) Enqueue(value interface{}) error {
	if rq.IsFull() {
		return errors.New("queue is full")
	}

	rq.data[rq.writeIndex] = value
	rq.writeIndex = (rq.writeIndex + 1) % rq.size
	rq.count++
	return nil
}

// Dequeue removes and returns the front element in the queue
func (rq *RingQueue) Dequeue() (interface{}, error) {
	if rq.IsEmpty() {
		return 0, errors.New("queue is empty")
	}

	value := rq.data[rq.readIndex]
	rq.readIndex = (rq.readIndex + 1) % rq.size
	rq.count--
	return value, nil
}

// Peek returns the front element without removing it
func (rq *RingQueue) Peek() (interface{}, error) {
	if rq.IsEmpty() {
		return 0, errors.New("queue is empty")
	}
	return rq.data[rq.readIndex], nil
}

// IsEmpty checks if the queue is empty
func (rq *RingQueue) IsEmpty() bool {
	return rq.count == 0
}

// IsFull checks if the queue is full
func (rq *RingQueue) IsFull() bool {
	return rq.count == rq.size
}
