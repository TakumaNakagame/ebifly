package room

import (
	"sync"
	"time"
)

const MaxThrowsPerRoom = 100

// Token-bucket rate limit per participant.
//   - RefillRate tokens/sec is the sustained throughput (human hammer rate).
//   - Capacity is the burst size; at a faster hammer rate than refill, the bucket
//     empties after roughly capacity / (hammerRate - refillRate) seconds, which
//     is ~10s for a typical human rapid-click (~10/s vs 5/s refill). Mechanical
//     spam beyond the bucket is silently dropped until the bucket refills.
const (
	BucketCapacity = 50
	RefillRate     = 5.0 // tokens per second
)

type Throw struct {
	ID                string `json:"id"`
	Emoji             string `json:"emoji"`
	TargetParticipant string `json:"targetParticipantId,omitempty"` // empty = broadcast to all
	ThrownAt          int64  `json:"thrownAt"`
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type ThrowBuffer struct {
	mu           sync.Mutex
	throwsByRoom map[string][]Throw
	bucketByUser map[string]*bucket
}

func NewThrowBuffer() *ThrowBuffer {
	return &ThrowBuffer{
		throwsByRoom: map[string][]Throw{},
		bucketByUser: map[string]*bucket{},
	}
}

// Add returns false if the participant is rate-limited.
func (b *ThrowBuffer) Add(roomID, participantID string, t Throw) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.takeTokenLocked(participantID) {
		return false
	}
	list := b.throwsByRoom[roomID]
	list = append(list, t)
	if len(list) > MaxThrowsPerRoom {
		list = list[len(list)-MaxThrowsPerRoom:]
	}
	b.throwsByRoom[roomID] = list
	return true
}

func (b *ThrowBuffer) takeTokenLocked(participantID string) bool {
	now := time.Now()
	bk, ok := b.bucketByUser[participantID]
	if !ok {
		bk = &bucket{tokens: BucketCapacity, lastRefill: now}
		b.bucketByUser[participantID] = bk
	}
	elapsed := now.Sub(bk.lastRefill).Seconds()
	if elapsed > 0 {
		bk.tokens += elapsed * RefillRate
		if bk.tokens > BucketCapacity {
			bk.tokens = BucketCapacity
		}
		bk.lastRefill = now
	}
	if bk.tokens < 1 {
		return false
	}
	bk.tokens -= 1
	return true
}

func (b *ThrowBuffer) List(roomID string) []Throw {
	b.mu.Lock()
	defer b.mu.Unlock()
	src := b.throwsByRoom[roomID]
	out := make([]Throw, len(src))
	copy(out, src)
	return out
}

func (b *ThrowBuffer) Clear(roomID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.throwsByRoom, roomID)
}

// ForgetParticipant removes rate-limit tracking for a participant (on leave).
func (b *ThrowBuffer) ForgetParticipant(participantID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.bucketByUser, participantID)
}
