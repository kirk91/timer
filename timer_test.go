package timer

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventLess(t *testing.T) {
	now := time.Now()
	e1 := &Event{expire: now.Add(time.Second)}
	e2 := &Event{expire: now.Add(time.Second * 2)}
	assert.Equal(t, e1.Less(e2), true)
	assert.Equal(t, e2.Less(e1), false)
}

func TestEventDelay(t *testing.T) {
	now := time.Now()
	e1 := &Event{expire: now.Add(time.Second)}
	d := e1.Delay()
	if d >= time.Second {
		t.Fatal("expected duration less one second, but got: %v", d)
	}
}

func TestTimerAdd(t *testing.T) {
	timer := New()
	timer.Add(time.Millisecond, func() {})
	event := timer.Events()[0]
	assert.NotNil(t, event.fn)
	timer.Del(event)

	ins := []struct {
		ttl time.Duration
		fn  ExpireFunc
	}{
		{time.Millisecond * 100, nil},
		{time.Millisecond * 50, nil},
		{time.Millisecond * 20, nil},
		{time.Millisecond * 10, nil},
		{time.Millisecond * 30, nil},
		{time.Millisecond * 15, nil},
	}
	for _, in := range ins {
		timer.Add(in.ttl, in.fn)
	}
	assert.Equal(t, timer.Len(), 6)

	outs := []Event{
		{ttl: time.Millisecond * 10},
		{ttl: time.Millisecond * 20},
		{ttl: time.Millisecond * 15},
		{ttl: time.Millisecond * 100},
		{ttl: time.Millisecond * 30},
		{ttl: time.Millisecond * 50},
	}
	events := timer.Events()
	for i := range events {
		assert.Equal(t, events[i].ttl, outs[i].ttl)
	}
}

func TestTimerDel(t *testing.T) {
	// nodes down
	ins := []struct {
		ttl time.Duration
		fn  ExpireFunc
	}{
		{time.Millisecond * 100, nil},
		{time.Millisecond * 50, nil},
		{time.Millisecond * 20, nil},
		{time.Millisecond * 10, nil},
		{time.Millisecond * 30, nil},
		{time.Millisecond * 15, nil},
	}
	timer := New()
	for _, in := range ins {
		timer.Add(in.ttl, in.fn)
	}

	events := timer.Events()
	timer.Del(events[0])
	assert.Equal(t, timer.Len(), 5)
	outs := []Event{
		{ttl: time.Millisecond * 15},
		{ttl: time.Millisecond * 20},
		{ttl: time.Millisecond * 50},
		{ttl: time.Millisecond * 100},
		{ttl: time.Millisecond * 30},
	}
	events = timer.Events()
	for i := range events {
		assert.Equal(t, events[i].ttl, outs[i].ttl)
	}

	// including node up
	ins = []struct {
		ttl time.Duration
		fn  ExpireFunc
	}{
		{time.Millisecond * 10, nil},
		{time.Millisecond * 90, nil},
		{time.Millisecond * 220, nil},
		{time.Millisecond * 110, nil},
		{time.Millisecond * 120, nil},
		{time.Millisecond * 230, nil},
		{time.Millisecond * 240, nil},
		{time.Millisecond * 130, nil},
	}
	timer = New()
	for _, in := range ins {
		timer.Add(in.ttl, in.fn)
	}
	timer.Del(timer.Events()[timer.Len()-3])
	outs = []Event{
		{ttl: time.Millisecond * 10},
		{ttl: time.Millisecond * 90},
		{ttl: time.Millisecond * 130},
		{ttl: time.Millisecond * 110},
		{ttl: time.Millisecond * 120},
		{ttl: time.Millisecond * 220},
		{ttl: time.Millisecond * 240},
	}
	events = timer.Events()
	for i := range events {
		assert.Equal(t, events[i].ttl, outs[i].ttl)
	}
}

func TestTimerLoop(t *testing.T) {
	timer := New()
	var wg sync.WaitGroup

	wg.Add(1)
	begin := time.Now()
	timer.Add(time.Millisecond*20, func() {
		defer wg.Done()
		if elasped := time.Since(begin); elasped > 25*time.Millisecond {
			assert.Fail(t, "expected execute event after 20 milliseconds, but actual after %v", elasped.String())
		}
	})

	wg.Add(1)
	timer.Add(time.Millisecond*10, func() {
		defer wg.Done()
		if elasped := time.Since(begin); elasped > 15*time.Millisecond {
			assert.Fail(t, "expected execute event after 10 milliseconds, but actual after %v", elasped.String())
		}
	})

	event := timer.Events()[1]
	timer.Start()
	wg.Wait()
	assert.Equal(t, timer.free.ttl, event.ttl)
	assert.Nil(t, timer.free.fn)
}
