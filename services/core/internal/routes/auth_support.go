package routes

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/authsteps"
	"mobile/services/core/internal/googleid"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/identity"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/sms"
)

type processSecrets struct{}

func (processSecrets) NewUUID() (string, error) { return repo.NewUUID() }

func (processSecrets) NewSessionToken() (string, error) { return mintGuestToken() }

func (processSecrets) TokenDigest(raw string) []byte { return auth.TokenDigest(raw) }

func (processSecrets) RandomBelow(bound int64) (int64, error) {
	if bound <= 0 {
		return 0, fmt.Errorf("secrets.randbelow() is only defined for positive bounds")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(bound))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

type hostIdentity struct{ raw string }

func (h hostIdentity) CanonicalMobile(raw string) (string, bool) {
	return identity.CanonicalMobile(raw)
}

func (h hostIdentity) ReadKey() ([]byte, error) {
	key, err := identity.ReadKey(h.raw)
	var missing *identity.PersonIDKeyMissing
	if errors.As(err, &missing) {
		return nil, &authsteps.KeyMissing{Message: missing.Message}
	}
	return key, err
}

func (h hostIdentity) PhoneDigest(canonical string, key []byte) ([]byte, error) {
	return identity.DerivePhoneDigest(canonical, key)
}

func (h hostIdentity) CodeDigest(challengeID, code string, key []byte) ([]byte, error) {
	id, err := uuidWireBytes(challengeID)
	if err != nil {
		return nil, err
	}
	return identity.DeriveCodeDigest(id, code, key)
}

func (h hostIdentity) PersonID(canonical string, key []byte) (string, error) {
	return identity.DerivePersonID(canonical, key)
}

func uuidWireBytes(id string) ([16]byte, error) {
	var out [16]byte
	canonical, err := auth.ParsePythonUUID(id)
	if err != nil {
		return out, err
	}
	n, err := hex.Decode(out[:], []byte(strings.ReplaceAll(canonical, "-", "")))
	if err != nil || n != 16 {
		return out, fmt.Errorf("routes: challenge id is not 16 bytes")
	}
	return out, nil
}

type smsDoor struct{ inner sms.Sender }

func (s smsDoor) SendOTP(canonicalPhone, code, challengeID string) error {
	if s.inner == nil {
		return fmt.Errorf("routes: SMS sender is not configured")
	}
	err := s.inner.SendOTP(canonicalPhone, code, challengeID)
	var undelivered *sms.DeliveryError
	if errors.As(err, &undelivered) {
		return &authsteps.DeliveryError{Message: undelivered.Message}
	}
	return err
}

type googleDoor struct{ inner googleid.Verifier }

func (g googleDoor) Verify(idToken string) (authsteps.GoogleClaims, error) {
	claims, err := g.inner.Verify(idToken)
	if err != nil {
		var broken *googleid.TokenInvalid
		if errors.As(err, &broken) {
			return authsteps.GoogleClaims{}, &authsteps.GoogleTokenInvalid{Message: broken.Message}
		}
		return authsteps.GoogleClaims{}, err
	}
	return authsteps.GoogleClaims{Subject: claims.Subject, DisplayName: claims.DisplayName}, nil
}

func googleVerifier(call *endpoint.Call) authsteps.GoogleVerifier {
	if call.Google == nil {
		return nil
	}
	return googleDoor{inner: call.Google}
}

func smsSender(call *endpoint.Call) (authsteps.SMSSender, error) {
	if call.SMS == nil {
		return nil, fmt.Errorf("routes: SMS sender is not configured")
	}
	return smsDoor{inner: call.SMS}, nil
}

func authRefusal(err error) error {
	var refused *authsteps.Refusal
	if errors.As(err, &refused) {
		return endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
	}
	return err
}

func authActor(call *endpoint.Call) authsteps.Actor {
	return authsteps.Actor{ID: call.Actor.ID, Roles: call.Actor.Roles}
}

func authNow() time.Time { return time.Now().UTC() }

func jsonObject(body []byte) (*pyjson.OrderedMap, error) {
	parsed, err := pyjson.Loads(body)
	if err != nil {
		var decodeErr *pyjson.DecodeError
		var unicodeErr *pyjson.UnicodeDecodeError
		if errors.As(err, &decodeErr) || errors.As(err, &unicodeErr) {
			return nil, endpoint.Refuse(422, "invalid_body", "Thân yêu cầu phải là JSON.")
		}
		return nil, err
	}
	object, ok := parsed.(*pyjson.OrderedMap)
	if !ok {
		return nil, endpoint.Refuse(422, "invalid_body", "Thân yêu cầu phải là một đối tượng JSON.")
	}
	return object, nil
}

func jsonGet(object *pyjson.OrderedMap, key string) any {
	value, ok := object.Get(key)
	if !ok || value == nil {
		return nil
	}
	switch t := value.(type) {
	case pyjson.Null:
		return nil
	case pyjson.String:
		return string(t)
	case pyjson.Bool:
		return bool(t)
	default:
		return t
	}
}

func challengeIDFrom(raw any) (string, error) {
	text, ok := raw.(string)
	if !ok {
		return "", endpoint.Refuse(422, "challenge_id_invalid", "Thiếu hoặc sai challenge_id.")
	}
	id, err := auth.ParsePythonUUID(text)
	if err != nil {
		return "", endpoint.Refuse(422, "challenge_id_invalid", "Thiếu hoặc sai challenge_id.")
	}
	return id, nil
}

func bearerFromValue(authorization string) (string, error) {
	scheme, rest, _ := strings.Cut(authorization, " ")
	token := strings.TrimSpace(rest)
	// Python uses str.strip() over the latin-1 header; auth.BearerToken does
	// that. The route parameter is already the decoded header string.
	if strings.ToLower(scheme) != "bearer" || token == "" {
		return "", endpoint.Refuse(401, "authentication_required", "Missing bearer session")
	}
	return token, nil
}

func optionalAuthorization(call *endpoint.Call) (*string, error) {
	value, ok := call.Values["authorization"]
	if !ok {
		return nil, nil
	}
	switch t := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyjson.String:
		text := string(t)
		if text == "" {
			return nil, nil
		}
		return &text, nil
	}
	return nil, fmt.Errorf("routes: authorization is %T, not a str or None", call.Values["authorization"])
}

func wireSession(view authsteps.SessionView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("token", pyjson.String(view.Token))
	out.Set("person_id", pyjson.String(view.PersonID))
	out.Set("expires_at", pyjson.String(pyjson.DateTime(view.ExpiresAt.UTC())))
	out.Set("issued_via", pyjson.String(view.IssuedVia))
	out.Set("is_new_person", pyjson.Bool(view.IsNewPerson))
	profile := pyjson.NewOrderedMap()
	profile.Set("display_name", pyjson.String(view.Profile.DisplayName))
	out.Set("profile", profile)
	contexts := make(pyjson.List, len(view.Contexts))
	for i, summary := range view.Contexts {
		contexts[i] = wireContextSummary(summary)
	}
	out.Set("contexts", contexts)
	out.Set("context_id", textOrNull(view.ContextID))
	out.Set("membership_state", textOrNull(view.MembershipState))
	out.Set("membership_id", textOrNull(view.MembershipID))
	return out
}

func wireSessionList(view authsteps.SessionListView) *pyjson.OrderedMap {
	list := make(pyjson.List, len(view.Sessions))
	for i, row := range view.Sessions {
		item := pyjson.NewOrderedMap()
		item.Set("id", pyjson.String(row.ID))
		item.Set("issued_via", pyjson.String(row.IssuedVia))
		item.Set("created_at", pyjson.String(pyjson.DateTime(row.CreatedAt.UTC())))
		item.Set("expires_at", pyjson.String(pyjson.DateTime(row.ExpiresAt.UTC())))
		item.Set("current", pyjson.Bool(row.Current))
		list[i] = item
	}
	out := pyjson.NewOrderedMap()
	out.Set("sessions", list)
	return out
}

func wireOtpRequest(view authsteps.OtpRequestView) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("challenge_id", pyjson.String(view.ChallengeID))
	out.Set("expires_in_seconds", pyjson.NewInt(view.ExpiresInSeconds))
	out.Set("resend_after_seconds", pyjson.NewInt(view.ResendAfterSeconds))
	return out
}
