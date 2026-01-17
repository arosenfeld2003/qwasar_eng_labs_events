package organizer

import (
	"container/heap"
	"sync"

	"marry-me/internal/event"
)

// PriorityQueue implements a thread-safe priority queue for events
type PriorityQueue struct {
	items eventHeap
	mu    sync.RWMutex
}

// eventHeap implements heap.Interface for events
type eventHeap []*event.Event

func (h eventHeap) Len() int { return len(h) }

// Less returns true if item i has higher priority than item j
// Higher priority values should come first, so we use > instead of <
// For same priority, earlier deadline comes first
func (h eventHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	// For same priority, earlier deadline comes first
	return h[i].Deadline.Before(h[j].Deadline)
}

func (h eventHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *eventHeap) Push(x interface{}) {
	*h = append(*h, x.(*event.Event))
}

func (h *eventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*h = old[0 : n-1]
	return item
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items: make(eventHeap, 0),
	}
	heap.Init(&pq.items)
	return pq
}

// Push adds an event to the queue
func (pq *PriorityQueue) Push(e *event.Event) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	heap.Push(&pq.items, e)
}

// Pop removes and returns the highest priority event
func (pq *PriorityQueue) Pop() (*event.Event, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return nil, false
	}

	item := heap.Pop(&pq.items)
	return item.(*event.Event), true
}

// Peek returns the highest priority event without removing it
func (pq *PriorityQueue) Peek() (*event.Event, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.items.Len() == 0 {
		return nil, false
	}

	return pq.items[0], true
}

// Len returns the number of items in the queue
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.items.Len()
}

// IsEmpty returns true if the queue is empty
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// RemoveExpired removes and returns all expired events from the queue
func (pq *PriorityQueue) RemoveExpired() []*event.Event {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	var expired []*event.Event
	var remaining eventHeap

	for _, e := range pq.items {
		if e.IsExpired() {
			expired = append(expired, e)
		} else {
			remaining = append(remaining, e)
		}
	}

	if len(expired) > 0 {
		pq.items = remaining
		heap.Init(&pq.items)
	}

	return expired
}

// Clear removes all items from the queue
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items = make(eventHeap, 0)
}

// GetAll returns all events (for inspection/debugging)
func (pq *PriorityQueue) GetAll() []*event.Event {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]*event.Event, len(pq.items))
	copy(result, pq.items)
	return result
}
