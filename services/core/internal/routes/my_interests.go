package routes

import (
	"context"
	"errors"
	"time"

	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

const notInVocabulary = "Lựa chọn không nằm trong danh sách máy chủ biết."

// putMyInterests is PUT /people/me/interests (routes/preferences.py
// put_my_interests, ApiService.set_my_interests): the whole answer replaces the
// stored one, the band is written only when it changed, and the reply lists the
// stored tags in vocabulary order.
func putMyInterests() Route {
	return Route{ID: "PUT /people/me/interests", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		refused, err := service.RequirePermission("manage_own_interests", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_self": true}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		chosen, err := stringListField(request, "interests")
		if err != nil {
			return endpoint.Reply{}, err
		}
		rawBand, err := optionalStringField(request, "budget_band")
		if err != nil {
			return endpoint.Reply{}, err
		}
		var unknown *interests.InterestError
		tags, err := interests.NormaliseInterests(chosen)
		if errors.As(err, &unknown) {
			return endpoint.Reply{}, endpoint.Refuse(422, unknown.Code, notInVocabulary)
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		band, err := interests.NormaliseBudgetBand(rawBand)
		if errors.As(err, &unknown) {
			return endpoint.Reply{}, endpoint.Refuse(422, unknown.Code, notInVocabulary)
		}
		if err != nil {
			return endpoint.Reply{}, err
		}

		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		people := repo.Repository{Q: tx}
		person, err := people.GetPerson(ctx, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if person == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "person_not_found", "Chưa có hồ sơ cho tài khoản này.")
		}
		stored, err := people.SetPersonInterests(ctx, call.Actor.ID, tags, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		if !sameBand(band, person.BudgetBand) {
			if _, err := people.UpdatePersonProfile(ctx, call.Actor.ID, repo.ProfileChanges{BudgetBand: repo.SetText(band)}); err != nil {
				return endpoint.Reply{}, err
			}
		}
		// Python normalises the stored tags again and lets any refusal escape
		// as an unhandled 500; an error here takes the same path.
		ordered, err := interests.NormaliseInterests(stored)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := pyjson.List{}
		for _, tag := range ordered {
			list = append(list, pyjson.String(tag))
		}
		body := pyjson.NewOrderedMap()
		body.Set("interests", list)
		body.Set("budget_band", textOrNull(band))
		return endpoint.Reply{Body: body}, nil
	}}
}

// sameBand is Python's `band != person.budget_band` negated: None equals only
// None.
func sameBand(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
