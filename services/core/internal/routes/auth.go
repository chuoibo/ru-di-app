package routes

import (
	"context"

	"mobile/services/core/internal/domain/authsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/limit"
)

// The three doors of app/api/routes/auth.py. Bodies are parsed by hand so a
// telephone number or an ID token is never echoed in a 422. The address
// limiter runs before the body is read, so a refused or broken request spends
// one slot too.

func requestOTP() Route {
	return Route{ID: "POST /auth/otp/request", Status: 202, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if call.Limits == nil {
			return endpoint.Reply{}, errNoLimits
		}
		if err := spendAddressWindow(call, call.Limits.OTPRequestLimit, limit.OTPRequestConfig); err != nil {
			return endpoint.Reply{}, err
		}
		object, err := jsonObject(call.Body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		sender, err := smsSender(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := newAuthStore(ctx, call)
		view, err := authsteps.RequestOTP(
			store, hostIdentity{raw: call.PersonIDKey}, processSecrets{}, sender,
			jsonGet(object, "phone"), call.OTPDebugCode, authNow(),
		)
		if err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Body: wireOtpRequest(view)}, nil
	}}
}

func verifyOTP() Route {
	return Route{ID: "POST /auth/otp/verify", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if call.Limits == nil {
			return endpoint.Reply{}, errNoLimits
		}
		if err := spendAddressWindow(call, call.Limits.OTPVerifyLimit, limit.OTPVerifyConfig); err != nil {
			return endpoint.Reply{}, err
		}
		object, err := jsonObject(call.Body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		challengeID, err := challengeIDFrom(jsonGet(object, "challenge_id"))
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := newAuthStore(ctx, call)
		view, err := authsteps.VerifyOTP(
			store, hostIdentity{raw: call.PersonIDKey}, processSecrets{},
			challengeID, jsonGet(object, "phone"), jsonGet(object, "code"), authNow(),
		)
		if err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Body: wireSession(view)}, nil
	}}
}

func loginGoogle() Route {
	return Route{ID: "POST /auth/google", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if call.Limits == nil {
			return endpoint.Reply{}, errNoLimits
		}
		if err := spendAddressWindow(call, call.Limits.GoogleLoginLimit, limit.GoogleLoginConfig); err != nil {
			return endpoint.Reply{}, err
		}
		object, err := jsonObject(call.Body)
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := authsteps.LoginWithGoogle(
			newAuthStore(ctx, call), processSecrets{}, googleVerifier(call), jsonGet(object, "id_token"), authNow(),
		)
		if err != nil {
			return endpoint.Reply{}, authRefusal(err)
		}
		return endpoint.Reply{Body: wireSession(view)}, nil
	}}
}
