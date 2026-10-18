package webui

import (
	"sync"
	"time"
)

const (
	BufferCapacity  = 250
	MaxDetailBytes  = 2048
	maxMessageBytes = 512
	subscriberQueue = 64
)

const (
	KindToolRead     = "TOOL_READ"
	KindToolDryRun   = "TOOL_DRY_RUN"
	KindToolMutation = "TOOL_MUTATION"
	KindToolError    = "TOOL_ERROR"
	KindResourceRead = "RESOURCE_READ"
	KindPromptGet    = "PROMPT_GET"
	KindServerLog    = "SERVER_LOG"
)

type Entry struct {
	ID       uint64    `json:"id"`
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"`
	Level    string    `json:"level,omitempty"`
	Message  string    `json:"message"`
	Detail   string    `json:"detail,omitempty"`
	Duration int64     `json:"duration_ms,omitempty"`
}

type Buffer struct {
	mu     sync.Mutex
	ring   []Entry
	start  int
	count  int
	nextID uint64
	subs   map[chan Entry]struct{}
	closed bool
}

func NewBuffer() *Buffer {
	return &Buffer{
		ring: make([]Entry, BufferCapacity),
		subs: make(map[chan Entry]struct{}),
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut] + "…[truncated]"
}

func (b *Buffer) Add(e Entry) {
	e.Message = truncate(e.Message, maxMessageBytes)
	e.Detail = truncate(e.Detail, MaxDetailBytes)
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.nextID++
	e.ID = b.nextID
	if b.count < len(b.ring) {
		b.ring[(b.start+b.count)%len(b.ring)] = e
		b.count++
	} else {
		b.ring[b.start] = e
		b.start = (b.start + 1) % len(b.ring)
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			delete(b.subs, ch)
			close(ch)
		}
	}
}

func (b *Buffer) Snapshot() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snapshotLocked()
}

func (b *Buffer) snapshotLocked() []Entry {
	out := make([]Entry, 0, b.count)
	for i := 0; i < b.count; i++ {
		out = append(out, b.ring[(b.start+i)%len(b.ring)])
	}
	return out
}

func (b *Buffer) Subscribe() (history []Entry, live <-chan Entry, cancel func()) {
	ch := make(chan Entry, subscriberQueue)
	b.mu.Lock()
	history = b.snapshotLocked()
	if b.closed {
		close(ch)
	} else {
		b.subs[ch] = struct{}{}
	}
	b.mu.Unlock()

	cancel = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
	return history, ch, cancel
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.ring {
		b.ring[i] = Entry{}
	}
	b.start, b.count = 0, 0
}

func (b *Buffer) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
		delete(b.subs, ch)
	}
	b.ring = nil
	b.start, b.count = 0, 0
}
