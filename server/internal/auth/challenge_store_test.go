package auth

import (
	"testing"
	"time"
)

func TestChallengeStore_ConsumeAndReplay(t *testing.T) {
	cs := NewChallengeStore(5)
	cs.Set("passkey_login:abc", "chal-1", "user-1", map[string]string{"k": "v"})
	ch, extra, ok := cs.GetAndVerify("passkey_login:abc", "")
	if !ok || ch != "chal-1" || extra["k"] != "v" {
		t.Fatalf("first get: ok=%v ch=%q extra=%v", ok, ch, extra)
	}
	cs.Delete("passkey_login:abc")
	if _, _, ok := cs.GetAndVerify("passkey_login:abc", ""); ok {
		t.Fatal("replay after delete should fail")
	}
}

func TestChallengeStore_Expired(t *testing.T) {
	cs := NewChallengeStore(0) // defaults to 5 min; force expire via direct store
	cs.mu.Lock()
	cs.store["k"] = &challengeEntry{
		Challenge: "c",
		UserID:    "u",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	cs.mu.Unlock()
	if _, _, ok := cs.GetAndVerify("k", ""); ok {
		t.Fatal("expired challenge should fail")
	}
}

func TestChallengeStore_CrossUserBinding(t *testing.T) {
	cs := NewChallengeStore(5)
	cs.Set("passkey_register:user-a", "chal-a", "user-a", nil)
	// wrong key (other user) does not resolve
	if _, _, ok := cs.GetAndVerify("passkey_register:user-b", "chal-a"); ok {
		t.Fatal("cross-user key should not match")
	}
	// wrong expected challenge
	if _, _, ok := cs.GetAndVerify("passkey_register:user-a", "wrong"); ok {
		t.Fatal("challenge mismatch should fail")
	}
	if _, _, ok := cs.GetAndVerify("passkey_register:user-a", "chal-a"); !ok {
		t.Fatal("correct binding should succeed")
	}
}
