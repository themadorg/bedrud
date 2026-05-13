package auth

import (
	"sync"
	"time"
)

type challengeEntry struct {
	Challenge string
	UserID    string
	Extra     map[string]string
	ExpiresAt time.Time
}

type ChallengeStore struct {
	mu    sync.RWMutex
	store map[string]*challengeEntry
	ttl   time.Duration
}

func NewChallengeStore(ttlMinutes int) *ChallengeStore {
	ttl := 5 * time.Minute
	if ttlMinutes > 0 {
		ttl = time.Duration(ttlMinutes) * time.Minute
	}
	return &ChallengeStore{
		store: make(map[string]*challengeEntry),
		ttl:   ttl,
	}
}

func (cs *ChallengeStore) Set(key, challenge, userID string, extra map[string]string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.store[key] = &challengeEntry{
		Challenge: challenge,
		UserID:    userID,
		Extra:     extra,
		ExpiresAt: time.Now().Add(cs.ttl),
	}
}

func (cs *ChallengeStore) GetAndVerify(key, expectedChallenge string) (challenge string, extra map[string]string, ok bool) {
	cs.mu.RLock()
	entry, found := cs.store[key]
	cs.mu.RUnlock()
	if !found {
		return "", nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		cs.Delete(key)
		return "", nil, false
	}
	if expectedChallenge != "" && entry.Challenge != expectedChallenge {
		return "", nil, false
	}
	return entry.Challenge, entry.Extra, true
}

func (cs *ChallengeStore) Delete(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.store, key)
}

func (cs *ChallengeStore) StartCleanup(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cs.purgeExpired()
			case <-stop:
				return
			}
		}
	}()
}

func (cs *ChallengeStore) purgeExpired() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	now := time.Now()
	for key, entry := range cs.store {
		if now.After(entry.ExpiresAt) {
			delete(cs.store, key)
		}
	}
}
