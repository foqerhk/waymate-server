package pair

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Permanent invites: QR/code stay valid until the family refreshes/revokes them.
// No wall-clock expiry is enforced.

type Signer struct {
	secret []byte
	base   string
}

func NewSigner(secret, publicBaseURL string, _ time.Duration) *Signer {
	return &Signer{secret: []byte(secret), base: strings.TrimRight(publicBaseURL, "/")}
}

func RandomCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out), nil
}

func (s *Signer) BuildPayload(familyID uuid.UUID, code string) (qrPayload, deepLink string) {
	msg := fmt.Sprintf("v=1&code=%s&fam=%s", code, familyID.String())
	sig := s.sign(msg)
	qrPayload = msg + "&sig=" + sig
	deepLink = "waymate://join?" + qrPayload
	_ = s.base
	return qrPayload, deepLink
}

// BuildElderOffer creates a QR that a family phone can scan to pull this elder in.
func (s *Signer) BuildElderOffer(deviceID uuid.UUID) (qrPayload, deepLink string) {
	msg := fmt.Sprintf("v=2&kind=elder&dev=%s", deviceID.String())
	sig := s.sign(msg)
	qrPayload = msg + "&sig=" + sig
	deepLink = "waymate://bind-elder?" + qrPayload
	return qrPayload, deepLink
}

// VerifyElderOffer returns the elder device id from a bind-elder QR / deep link.
func (s *Signer) VerifyElderOffer(raw string) (deviceID uuid.UUID, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "waymate://bind-elder?") {
		raw = strings.TrimPrefix(raw, "waymate://bind-elder?")
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return uuid.Nil, err
	}
	if values.Get("kind") != "elder" {
		return uuid.Nil, fmt.Errorf("not an elder offer")
	}
	dev := values.Get("dev")
	sig := values.Get("sig")
	if dev == "" || sig == "" {
		return uuid.Nil, fmt.Errorf("missing elder offer fields")
	}
	deviceID, err = uuid.Parse(dev)
	if err != nil {
		return uuid.Nil, fmt.Errorf("bad device id")
	}
	msg := fmt.Sprintf("v=2&kind=elder&dev=%s", deviceID.String())
	if !hmac.Equal([]byte(sig), []byte(s.sign(msg))) {
		return uuid.Nil, fmt.Errorf("bad signature")
	}
	return deviceID, nil
}

// PayloadKind classifies a scanned QR for mutual pairing.
func PayloadKind(raw string) string {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "kind=elder") || strings.HasPrefix(lower, "waymate://bind-elder") {
		return "elder"
	}
	return "family"
}

func (s *Signer) VerifyPayload(raw string) (code string, familyID uuid.UUID, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "waymate://join?") {
		raw = strings.TrimPrefix(raw, "waymate://join?")
	}
	// Bare invite codes must be handled before url.ParseQuery — it treats
	// "ABC123" as a query key with an empty value (len(values)==1), which
	// incorrectly falls through to "missing code".
	if len(raw) >= 6 && !strings.ContainsAny(raw, "=&") {
		return strings.ToUpper(raw), uuid.Nil, nil
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", uuid.Nil, err
	}

	code = strings.ToUpper(values.Get("code"))
	fam := values.Get("fam")
	sig := values.Get("sig")
	if code == "" {
		return "", uuid.Nil, fmt.Errorf("missing code")
	}
	if fam == "" || sig == "" {
		// Bare code / legacy payload without signature.
		return code, uuid.Nil, nil
	}
	familyID, err = uuid.Parse(fam)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("bad family id")
	}
	msg := fmt.Sprintf("v=1&code=%s&fam=%s", code, familyID.String())
	// Accept legacy payloads that still carry exp= (ignore expiry).
	if values.Get("exp") != "" {
		legacy := fmt.Sprintf("v=1&code=%s&fam=%s&exp=%s", code, familyID.String(), values.Get("exp"))
		if hmac.Equal([]byte(sig), []byte(s.sign(legacy))) {
			return code, familyID, nil
		}
	}
	if !hmac.Equal([]byte(sig), []byte(s.sign(msg))) {
		return "", uuid.Nil, fmt.Errorf("bad signature")
	}
	return code, familyID, nil
}

func (s *Signer) sign(msg string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func EncodeJSONQR(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// FarFuture used only for DB column compatibility (NOT enforced).
func FarFuture() time.Time {
	return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
}
