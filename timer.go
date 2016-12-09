package timer

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"
)

// ExpireFunc represents a function will be executed when a event is trigged.
type ExpireFunc func()

// An Event represents an elemenet of the events in the timer.
type Event struct {
	index int // index in the min heap structure

	// time to live for event
	ttl    time.Duration
	expire time.Time
	fn     ExpireFunc

	next *Event
}

// Less is used to compare expiration with other events.
func (e *Event) Less(o *Event) bool {
	return e.expire.Before(o.expire)
}

// Delay is used to give the duration that event will expire.
func (e *Event) Delay() time.Duration {
	return e.expire.Sub(time.Now())
}

func (e *Event) String() string {
	return fmt.Sprintf("index %d ttl %v, expire at %v", e.index, e.ttl, e.expire)
}

const (
	// InfiniteDuration is the default time that timer bLocks.
	InfiniteDuration = time.Duration(1<<63 - 1)
)

var (
	// DefaultAllocCap is the expanding capicity when the timer has no capicity to store more events.
	DefaultAllocCap = 1024
)

// A Timer represents a set that manage events which is related with time.
// It uses min-heap structure to orginaze and handle events.
type Timer struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	allocCap int      // the cap that reallocate more events
	free     *Event   // free events
	events   []*Event // min heap array

	raw *time.Timer
}

// New returns an timer instance with the default allocate capicity
func New() *Timer {
	t := &Timer{
		allocCap: DefaultAllocCap,
		raw:      time.NewTimer(InfiniteDuration),
	}
	t.allocate()
	return t
}

// Len returns the length of min heap array
func (t *Timer) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.events)
}

func (t *Timer) Events() []*Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.events
}

// allocate is used to expand min-heap array
func (t *Timer) allocate() {
	events := make([]Event, t.allocCap)
	t.free = &events[0]
	for i := 0; i < t.allocCap; i++ {
		if i < t.allocCap-1 {
			events[i].next = &events[i+1]
		}
	}
}

// Add is used to add new event
func (t *Timer) Add(ttl time.Duration, fn ExpireFunc) *Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	event := t.get()
	event.ttl = ttl
	event.expire = time.Now().Add(ttl)
	event.fn = fn
	t.add(event)
	return event
}

func (t *Timer) get() *Event {
	event := t.free
	if event == nil {
		t.allocate()
		event = t.free
	}
	t.free = event.next
	return event
}

func (t *Timer) add(event *Event) {
	event.index = len(t.events)
	t.events = append(t.events, event)
	t.upEvent(event.index)
	if event.index == 0 {
		// reset signal
		t.reset(event.Delay())
	}
	return
}

func (t *Timer) upEvent(j int) {
	for {
		i := (j - 1) / 2
		if i == j || !t.events[j].Less(t.events[i]) {
			break
		}
		t.swapEvent(i, j)
		j = i
	}
}

func (t *Timer) swapEvent(i, j int) {
	t.events[i], t.events[j] = t.events[j], t.events[i]
	t.events[i].index = i
	t.events[j].index = j
}

// Del is used to remove event from timer.
func (t *Timer) Del(event *Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.del(event)
	t.put(event)
}

// put is used to put event to free-event linked list
func (t *Timer) put(event *Event) {
	event.fn = nil
	event.next = t.free
	t.free = event
}

func (t *Timer) del(event *Event) {
	i := event.index
	last := len(t.events) - 1
	if i < 0 || i > last || t.events[i] != event {
		// invalid event or event has been removed
		return
	}

	if i != last {
		t.swapEvent(i, last)
		t.downEvent(i)
		t.upEvent(i)
	}

	// remove the last event
	t.events[last].index = -1
	t.events = t.events[:last]
}

func (t *Timer) downEvent(i int) {
	n := len(t.events) - 1
	for {
		left := 2*i + 1
		if left >= n || left < 0 {
			// greather than max index or number overflow
			break
		}
		j := left
		if right := left + 1; right < n && t.events[right].Less(t.events[left]) {
			j = right
		}
		if t.events[i].Less(t.events[j]) {
			break
		}
		t.swapEvent(i, j)
		i = j
	}
}

// Start is used to start the timer.
func (t *Timer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	t.ctx, t.cancel = ctx, cancel
	go t.loop()
}

// Stop is used to stop the timer.
func (t *Timer) Stop() {
	t.cancel()
}

func (t *Timer) loop() {
	var (
		d     time.Duration
		fn    ExpireFunc
		event *Event
	)
	for {
		select {
		case <-t.ctx.Done():
			t.raw.Stop()
			return
		case <-t.raw.C:
			t.mu.Lock()
			for {
				if len(t.events) == 0 {
					d = InfiniteDuration
					break
				}

				event = t.events[0]
				if d = event.Delay(); d > 0 {
					break
				}

				fn = event.fn
				t.del(event)
				t.put(event)
				t.mu.Unlock()
				// todo: tune performance with go routines
				if fn != nil {
					fn()
				}
				t.mu.Lock()
			}

			// reset signal
			t.reset(d)
			t.mu.Unlock()
		}
	}
}

func (t *Timer) reset(d time.Duration) {
	t.raw.Reset(d)
}