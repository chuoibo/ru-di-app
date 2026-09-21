package routes

import (
	"context"
	"encoding/hex"
	"errors"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/identity"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

var errNoLimits = errors.New("routes: the endpoint environment carries no limiter set")

// mintPersonID is POST /identity/person-id (routes/identity.py
// mint_person_id): the limiter, the hand-parsed body, the number, the key,
// then the derived id. No actor dependency and no database.
func mintPersonID() Route {
	return Route{ID: "POST /identity/person-id", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if call.Limits == nil {
			return endpoint.Reply{}, errNoLimits
		}
		if err := spendAddressWindow(call, call.Limits.PersonIDLimit, limit.PersonIDConfig); err != nil {
			return endpoint.Reply{}, err
		}
		canonical, err := mobileFromBody(call.Body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		key, err := personIDKey(call, "Máy chủ chưa cấu hình khoá danh tính nên chưa đăng nhập được.")
		if err != nil {
			return endpoint.Reply{}, err
		}
		// Unicode digits pass canonical_mobile and fail encode("ascii") here:
		// an unhandled exception in Python, a 500 in Go.
		personID, err := identity.DerivePersonID(canonical, key)
		if err != nil {
			return endpoint.Reply{}, err
		}
		out := pyjson.NewOrderedMap()
		out.Set("person_id", pyjson.String(personID))
		return endpoint.Reply{Body: out}, nil
	}}
}

// findPersonByPhone is POST /friends/lookup (routes/friends.py
// find_person_by_phone, ApiService.find_person_by_phone_identity and
// find_person_by_person_id). get_actor has already run. Then the limiter, the
// body, the number, the key, both derivations, the bound account identity,
// the permission, and the person, whom an ended account or a hidden number
// turns into the same 404 as nobody.
func findPersonByPhone() Route {
	return Route{ID: "POST /friends/lookup", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if call.Limits == nil {
			return endpoint.Reply{}, errNoLimits
		}
		if err := spendAddressWindow(call, call.Limits.FriendLookupLimit, limit.FriendLookupConfig); err != nil {
			return endpoint.Reply{}, err
		}
		canonical, err := mobileFromBody(call.Body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		key, err := personIDKey(call, "Máy chủ chưa cấu hình khoá danh tính nên chưa tìm bạn được.")
		if err != nil {
			return endpoint.Reply{}, err
		}
		digest, err := identity.DerivePhoneDigest(canonical, key)
		if err != nil {
			return endpoint.Reply{}, err
		}
		derivedID, err := identity.DerivePersonID(canonical, key)
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		bound, err := store.GetAccountIdentity(ctx, "phone", hex.EncodeToString(digest))
		if err != nil {
			return endpoint.Reply{}, err
		}
		personID := derivedID
		if bound != nil {
			personID = bound.PersonID
		}
		if err := requireFacts(call, "find_person_by_phone", map[string]bool{}); err != nil {
			return endpoint.Reply{}, err
		}
		person, err := store.GetPerson(ctx, personID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if person != nil && (person.DeletedAt != nil || !person.DiscoverableByPhone) {
			person = nil
		}
		if person == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "person_not_found", "Chưa có ai dùng số này trong Rủ Đi.")
		}
		out := pyjson.NewOrderedMap()
		out.Set("person_id", pyjson.String(person.ID))
		out.Set("display_name", pyjson.String(person.DisplayName))
		return endpoint.Reply{Body: out}, nil
	}}
}

// spendAddressWindow is `if not limiter.allow(_caller(request))`. It runs
// before the body is read, so a refused or broken request spends one too.
func spendAddressWindow(call *endpoint.Call, window *limit.AddressWindow, cfg limit.AddressConfig) error {
	if !window.Allow(limit.Caller(call.Request)) {
		return endpoint.Refuse(429, cfg.Code, cfg.Detail)
	}
	return nil
}

// mobileFromBody is what mint_person_id and find_person_by_phone share after
// the limiter: `await request.json()`, `body.get("phone")` when the body is an
// object, a string check, then canonical_mobile. Every refusal is a fixed
// sentence; the number is never echoed.
func mobileFromBody(body []byte) (string, error) {
	parsed, err := pyjson.Loads(body)
	if err != nil {
		var decodeErr *pyjson.DecodeError
		var unicodeErr *pyjson.UnicodeDecodeError
		if errors.As(err, &decodeErr) || errors.As(err, &unicodeErr) {
			return "", endpoint.Refuse(422, "invalid_body", "Thân yêu cầu phải là JSON.")
		}
		return "", err
	}
	var phone pyjson.Value
	if object, ok := parsed.(*pyjson.OrderedMap); ok {
		phone, _ = object.Get("phone")
	}
	text, ok := phone.(pyjson.String)
	if !ok {
		return "", endpoint.Refuse(422, "phone_required", "Thiếu trường phone, và phải là chuỗi.")
	}
	canonical, ok := identity.CanonicalMobile(string(text))
	if !ok {
		return "", endpoint.Refuse(422, "phone_not_mobile", "Chưa đúng dạng số di động Việt Nam.")
	}
	return canonical, nil
}

// personIDKey is read_key; a missing or short key is the route's 503.
func personIDKey(call *endpoint.Call, detail string) ([]byte, error) {
	key, err := identity.ReadKey(call.PersonIDKey)
	var missing *identity.PersonIDKeyMissing
	if errors.As(err, &missing) {
		return nil, endpoint.Refuse(503, "identity_key_missing", detail)
	}
	return key, err
}
