// Package viewers polls each connected platform for its live viewer count.
// Every source is an undocumented public endpoint, so a failure is never an
// error for the user: the last good number is held for a while, then dropped.
package viewers

import (
	"context"
	"sort"
	"sync"
	"time"

	"ghost-chat/internal/chat"
	"ghost-chat/internal/logging"
)

const (
	StaleAfter   = 2 * time.Minute
	pollTimeout  = 10 * time.Second
	maxBodyBytes = 4 << 20
)

type Update struct {
	Platform chat.Platform `json:"platform"`
	Count    int           `json:"count"`
	Live     bool          `json:"live"`
}

// Source reports the current viewer count of one channel. live is false when
// the channel is not streaming; err means the count could not be read.
type Source func(ctx context.Context) (count int, live bool, err error)

type poller struct {
	cancel   context.CancelFunc
	latest   Update
	hasValue bool
	lastGood time.Time
}

type Manager struct {
	mu         sync.Mutex
	emit       func(Update)
	pollers    map[chat.Platform]*poller
	staleAfter time.Duration
}

func NewManager(emit func(Update)) *Manager {
	return &Manager{
		emit:       emit,
		pollers:    map[chat.Platform]*poller{},
		staleAfter: StaleAfter,
	}
}

// Start begins polling src for platform p, replacing any poller already
// running for it. The first poll happens immediately.
func (m *Manager) Start(p chat.Platform, src Source, every time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	entry := &poller{cancel: cancel}

	m.mu.Lock()

	if old, ok := m.pollers[p]; ok {
		old.cancel()

		entry.latest = old.latest
		entry.hasValue = old.hasValue
		entry.lastGood = old.lastGood
	}

	m.pollers[p] = entry

	m.mu.Unlock()

	go m.run(ctx, p, entry, src, every)
}

func (m *Manager) run(ctx context.Context, p chat.Platform, entry *poller, src Source, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		m.poll(ctx, p, entry, src)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) poll(ctx context.Context, p chat.Platform, entry *poller, src Source) {
	pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
	count, live, err := src(pollCtx)

	cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx.Err() != nil {
		return
	}

	if err != nil {
		logging.Platform(string(p)).Debug("viewer count unavailable", "error", err)

		if entry.hasValue && entry.latest.Live && time.Since(entry.lastGood) > m.staleAfter {
			m.set(entry, Update{Platform: p})
		}

		return
	}

	if !live || count < 0 {
		count = 0
	}

	entry.lastGood = time.Now()

	m.set(entry, Update{Platform: p, Count: count, Live: live})
}

// set records u and emits it when it differs from what was last emitted.
// Callers hold m.mu.
func (m *Manager) set(entry *poller, u Update) {
	if entry.hasValue && entry.latest == u {
		return
	}

	entry.latest = u
	entry.hasValue = true

	m.emit(u)
}

// Push records a count delivered by the platform itself (TikTok sends it on
// the chat socket). It is ignored unless the platform has been started.
func (m *Manager) Push(p chat.Platform, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.pollers[p]
	if !ok {
		return
	}

	entry.lastGood = time.Now()

	m.set(entry, Update{Platform: p, Count: max(count, 0), Live: true})
}

// Stop ends polling for p and reports it as not live.
func (m *Manager) Stop(p chat.Platform) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.pollers[p]
	if !ok {
		return
	}

	entry.cancel()
	delete(m.pollers, p)

	m.emit(Update{Platform: p})
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for p, entry := range m.pollers {
		entry.cancel()
		delete(m.pollers, p)
	}
}

// Latest returns the most recent update of every started platform that has
// one, ordered by platform id.
func (m *Manager) Latest() []Update {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]Update, 0, len(m.pollers))

	for _, entry := range m.pollers {
		if entry.hasValue {
			out = append(out, entry.latest)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Platform < out[j].Platform })

	return out
}
