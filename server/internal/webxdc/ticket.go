package webxdc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TicketClaims is a short-lived capability for asset access on the webxdc host.
type TicketClaims struct {
	JTI   string `json:"jti"`
	Inst  string `json:"inst"`
	Room  string `json:"room"`
	Sub   string `json:"sub"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
	Scope string `json:"scope"`
}

const ticketScope = "webxdc-assets"

// MintTicket creates a signed ticket string (base64url payload + HMAC).
func MintTicket(secret, jti, instanceID, roomID, userID string, ttl time.Duration) (string, time.Time, error) {
	if secret == "" {
		return "", time.Time{}, errors.New("webxdc: empty ticket secret")
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)
	claims := TicketClaims{
		JTI:   jti,
		Inst:  instanceID,
		Room:  roomID,
		Sub:   userID,
		Exp:   exp.Unix(),
		Iat:   now.Unix(),
		Scope: ticketScope,
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret+"|webxdc-ticket"))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, exp, nil
}

// VerifyTicket validates signature, expiry, scope, and instance binding.
func VerifyTicket(secret, token, expectInstance string) (*TicketClaims, error) {
	if secret == "" || token == "" {
		return nil, errors.New("webxdc: missing ticket")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("webxdc: malformed ticket")
	}
	payload, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(secret+"|webxdc-ticket"))
	_, _ = mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil, errors.New("webxdc: bad ticket signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("webxdc: bad ticket payload")
	}
	var claims TicketClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, errors.New("webxdc: bad ticket json")
	}
	if claims.Scope != ticketScope {
		return nil, errors.New("webxdc: bad ticket scope")
	}
	if time.Now().UTC().Unix() > claims.Exp {
		return nil, errors.New("webxdc: ticket expired")
	}
	if expectInstance != "" && claims.Inst != expectInstance {
		return nil, fmt.Errorf("webxdc: ticket instance mismatch")
	}
	return &claims, nil
}

// SelfAddr derives opaque per-app address (not for UI display).
func SelfAddr(secret, roomID, instanceID, userID string) string {
	mac := hmac.New(sha256.New, []byte(secret+"|webxdc-addr"))
	_, _ = mac.Write([]byte(roomID + "|" + instanceID + "|" + userID))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}
