package routes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"mobile/services/core/internal/domain/outingsteps"
	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

type outingStore struct {
	ctx   context.Context
	store repo.Repository
}

func newOutingStore(ctx context.Context, call *endpoint.Call) (outingStore, error) {
	store, err := groupStore(ctx, call)
	if err != nil {
		return outingStore{}, err
	}
	return outingStore{ctx: ctx, store: store}, nil
}

func outingConflict(err error) error {
	var conflict *repo.Conflict
	if errors.As(err, &conflict) {
		return &outingsteps.Conflict{Code: conflict.Code}
	}
	return err
}

func outingDay(d outingsteps.Date) time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

func parseISODate(s string) (outingsteps.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return outingsteps.Date{}, err
	}
	return pairpaper.DateOf(t), nil
}

func decodeDays(raw []json.RawMessage) ([]outingsteps.Day, error) {
	out := make([]outingsteps.Day, 0, len(raw))
	for _, item := range raw {
		var row struct {
			Day           string  `json:"day"`
			TransportMode string  `json:"transport_mode"`
			StartAt       string  `json:"start_at"`
			StartStopID   *string `json:"start_stop_id"`
			EndStopID     *string `json:"end_stop_id"`
			ReturnToStart bool    `json:"return_to_start"`
		}
		if err := json.Unmarshal(item, &row); err != nil {
			return nil, err
		}
		day, err := parseISODate(row.Day)
		if err != nil {
			return nil, err
		}
		out = append(out, outingsteps.Day{
			Day: day, TransportMode: row.TransportMode, StartAt: row.StartAt,
			StartStopID: row.StartStopID, EndStopID: row.EndStopID, ReturnToStart: row.ReturnToStart,
		})
	}
	return out, nil
}

func dumpDays(days []outingsteps.Day) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, len(days))
	for i, day := range days {
		encoded, err := pyjson.Compact(wireDay(day))
		if err != nil {
			return nil, err
		}
		out[i] = encoded
	}
	return out, nil
}

func mapOutingStop(s repo.OutingStop) outingsteps.Stop {
	out := outingsteps.Stop{
		ID: s.ID, Position: int(s.Position), MinuteOfDay: s.MinuteOfDay, Label: s.Label,
		PlaceName: s.PlaceName, PlaceID: s.PlaceID, DurationMinutes: s.DurationMinutes,
		TimeLocked: s.TimeLocked, MeetingLat: s.MeetingLat, MeetingLng: s.MeetingLng,
		MeetingLabel: s.MeetingLabel,
	}
	if s.Day != nil {
		d := pairpaper.DateOf(*s.Day)
		out.Day = &d
	}
	return out
}

func (o outingStore) outing(record repo.Outing) (outingsteps.Outing, error) {
	days, err := decodeDays(record.ItineraryDays)
	if err != nil {
		return outingsteps.Outing{}, err
	}
	stops := make([]outingsteps.Stop, 0, len(record.Stops))
	for _, stop := range record.Stops {
		stops = append(stops, mapOutingStop(stop))
	}
	return outingsteps.Outing{
		ID: record.ID, ContextID: record.ContextID, CreatedByID: record.CreatedByID,
		Title: record.Title, StartsOn: pairpaper.DateOf(record.StartsOn), EndsOn: pairpaper.DateOf(record.EndsOn),
		Headcount: record.Headcount, BudgetPerPersonVND: record.BudgetPerPersonVND, CreatedAt: record.CreatedAt,
		Stops: stops, TimelineRevision: record.TimelineRevision, ItineraryVersion: record.ItineraryVersion,
		ItineraryDays: days,
	}, nil
}

func mapOutingInvite(record repo.OutingInvite) outingsteps.Invite {
	return outingsteps.Invite{
		ID: record.ID, OutingID: record.OutingID, Source: record.Source,
		InvitedPersonID: record.InvitedPersonID, InvitedByID: record.InvitedByID,
		AcceptedAt: record.AcceptedAt, AcceptedByID: record.AcceptedByID,
		CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt, RevokedAt: record.RevokedAt,
	}
}

func (o outingStore) IsMember(contextID, personID string) (bool, error) {
	return o.store.IsMember(o.ctx, contextID, personID)
}

func (o outingStore) GetContext(contextID string) (*outingsteps.Context, error) {
	record, err := o.store.GetContext(o.ctx, contextID)
	if err != nil || record == nil {
		return nil, err
	}
	return &outingsteps.Context{Kind: record.Kind}, nil
}

func (o outingStore) ListMembers(contextID string) ([]outingsteps.Member, error) {
	rows, err := o.store.ListMembers(o.ctx, contextID)
	if err != nil {
		return nil, err
	}
	out := make([]outingsteps.Member, len(rows))
	for i, row := range rows {
		out[i] = outingsteps.Member{PersonID: row.PersonID, State: row.State}
	}
	return out, nil
}

func (o outingStore) GetPerson(personID string) (*outingsteps.Person, error) {
	record, err := o.store.GetPerson(o.ctx, personID)
	if err != nil || record == nil {
		return nil, err
	}
	return &outingsteps.Person{DisplayName: record.DisplayName}, nil
}

func (o outingStore) GetPlace(placeID string) (*outingsteps.Place, error) {
	record, err := o.store.GetPlace(o.ctx, placeID)
	if err != nil || record == nil {
		return nil, err
	}
	return &outingsteps.Place{Lat: record.Lat, Lng: record.Lng}, nil
}

func (o outingStore) CreateOuting(draft outingsteps.OutingDraft) (outingsteps.Outing, error) {
	record, err := o.store.CreateOuting(o.ctx, repo.OutingInput{
		ContextID: draft.ContextID, CreatedByID: draft.CreatedByID, Title: draft.Title,
		StartsOn: outingDay(draft.StartsOn), EndsOn: outingDay(draft.EndsOn),
		Headcount: draft.Headcount, BudgetPerPersonVND: draft.BudgetPerPersonVND, Now: draft.Now,
	})
	if err != nil {
		return outingsteps.Outing{}, outingConflict(err)
	}
	return o.outing(record)
}

func (o outingStore) GetOuting(outingID string) (*outingsteps.Outing, error) {
	record, err := o.store.GetOuting(o.ctx, outingID)
	if err != nil || record == nil {
		return nil, err
	}
	view, err := o.outing(*record)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (o outingStore) ListOutings(contextID string) ([]outingsteps.Outing, error) {
	rows, err := o.store.ListOutings(o.ctx, contextID)
	if err != nil {
		return nil, err
	}
	out := make([]outingsteps.Outing, 0, len(rows))
	for _, row := range rows {
		view, err := o.outing(row)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (o outingStore) ReplaceOutingStops(outingID string, stops []outingsteps.StopDraft, expectedRevision *int64) (outingsteps.Outing, error) {
	rows := make([]repo.TimelineStop, len(stops))
	for i, stop := range stops {
		rows[i] = repo.TimelineStop{
			MinuteOfDay: stop.MinuteOfDay, Label: stop.Label, PlaceName: stop.PlaceName, PlaceID: stop.PlaceID,
		}
	}
	record, err := o.store.ReplaceOutingStops(o.ctx, outingID, rows, expectedRevision)
	if err != nil {
		return outingsteps.Outing{}, outingConflict(err)
	}
	return o.outing(record)
}

func (o outingStore) ReplaceOutingItinerary(outingID string, stops []outingsteps.ItineraryStopDraft, days []outingsteps.Day, expectedRevision int64) (outingsteps.Outing, error) {
	rows := make([]repo.ItineraryStop, len(stops))
	for i, stop := range stops {
		row := repo.ItineraryStop{
			ID: stop.ID, MinuteOfDay: stop.MinuteOfDay, Label: stop.Label,
			PlaceName: stop.PlaceName, PlaceID: stop.PlaceID, DurationMinutes: stop.DurationMinutes,
			TimeLocked: stop.TimeLocked,
		}
		if stop.Day != nil {
			day := outingDay(*stop.Day)
			row.Day = &day
		}
		if stop.MeetingPoint != nil {
			row.MeetingLat = &stop.MeetingPoint.Lat
			row.MeetingLng = &stop.MeetingPoint.Lng
			row.MeetingLabel = &stop.MeetingPoint.Label
		}
		rows[i] = row
	}
	dumped, err := dumpDays(days)
	if err != nil {
		return outingsteps.Outing{}, err
	}
	record, err := o.store.ReplaceOutingItinerary(o.ctx, outingID, rows, dumped, expectedRevision)
	if err != nil {
		return outingsteps.Outing{}, outingConflict(err)
	}
	return o.outing(record)
}

func (o outingStore) GetOutingStop(stopID string) (*outingsteps.Stop, *outingsteps.Outing, error) {
	stop, outing, err := o.store.GetOutingStop(o.ctx, stopID)
	if err != nil || stop == nil || outing == nil {
		return nil, nil, err
	}
	view, err := o.outing(*outing)
	if err != nil {
		return nil, nil, err
	}
	mapped := mapOutingStop(*stop)
	return &mapped, &view, nil
}

func (o outingStore) CreateStopCheckin(stopID, personID string, now time.Time) (outingsteps.Checkin, error) {
	record, err := o.store.CreateStopCheckin(o.ctx, stopID, personID, now)
	if err != nil {
		return outingsteps.Checkin{}, outingConflict(err)
	}
	return outingsteps.Checkin{ID: record.ID, StopID: record.StopID, PersonID: record.PersonID, CreatedAt: record.CreatedAt}, nil
}

func (o outingStore) ListOutingCheckins(outingID string) ([]outingsteps.Checkin, error) {
	rows, err := o.store.ListOutingCheckins(o.ctx, outingID)
	if err != nil {
		return nil, err
	}
	out := make([]outingsteps.Checkin, len(rows))
	for i, row := range rows {
		out[i] = outingsteps.Checkin{ID: row.ID, StopID: row.StopID, PersonID: row.PersonID, CreatedAt: row.CreatedAt}
	}
	return out, nil
}

func (o outingStore) CreateOutingInvite(draft outingsteps.InviteDraft) (outingsteps.Invite, error) {
	record, err := o.store.CreateOutingInvite(o.ctx, repo.OutingInviteInput{
		OutingID: draft.OutingID, Source: draft.Source, InvitedPersonID: draft.InvitedPersonID,
		InvitedByID: draft.InvitedByID, TokenDigest: draft.TokenDigest, ExpiresAt: draft.ExpiresAt, Now: draft.Now,
	})
	if err != nil {
		return outingsteps.Invite{}, outingConflict(err)
	}
	return mapOutingInvite(record), nil
}

func (o outingStore) FindOutingInviteForPerson(outingID, personID string) (*outingsteps.Invite, error) {
	record, err := o.store.FindOutingInviteForPerson(o.ctx, outingID, personID)
	if err != nil || record == nil {
		return nil, err
	}
	invite := mapOutingInvite(*record)
	return &invite, nil
}

func (o outingStore) GetOutingInvite(inviteID string) (*outingsteps.Invite, error) {
	record, err := o.store.GetOutingInvite(o.ctx, inviteID)
	if err != nil || record == nil {
		return nil, err
	}
	invite := mapOutingInvite(*record)
	return &invite, nil
}

func (o outingStore) GetOutingInviteByDigest(tokenDigest []byte) (*outingsteps.Invite, error) {
	record, err := o.store.GetOutingInviteByDigest(o.ctx, tokenDigest)
	if err != nil || record == nil {
		return nil, err
	}
	invite := mapOutingInvite(*record)
	return &invite, nil
}

func (o outingStore) AcceptOutingInvite(inviteID, acceptedByID string, now time.Time) (outingsteps.Invite, error) {
	record, err := o.store.AcceptOutingInvite(o.ctx, inviteID, acceptedByID, now)
	if err != nil {
		return outingsteps.Invite{}, outingConflict(err)
	}
	return mapOutingInvite(record), nil
}

func (o outingStore) RevokeOutingInvite(inviteID string, now time.Time) (outingsteps.Invite, error) {
	record, err := o.store.RevokeOutingInvite(o.ctx, inviteID, now)
	if err != nil {
		return outingsteps.Invite{}, outingConflict(err)
	}
	return mapOutingInvite(record), nil
}

func (o outingStore) RotateOutingInviteDigest(inviteID string, tokenDigest []byte, expiresAt, now time.Time) (outingsteps.Invite, error) {
	record, err := o.store.RotateOutingInviteDigest(o.ctx, inviteID, tokenDigest, expiresAt, now)
	if err != nil {
		return outingsteps.Invite{}, outingConflict(err)
	}
	return mapOutingInvite(record), nil
}

func (o outingStore) EnsureInvitedMembership(draft outingsteps.MembershipDraft) (outingsteps.Membership, error) {
	record, err := o.store.EnsureInvitedMembership(o.ctx, draft.ContextID, draft.PersonID, draft.InvitedByID, draft.Origin, draft.Now)
	if err != nil {
		return outingsteps.Membership{}, outingConflict(err)
	}
	return outingsteps.Membership{ID: record.ID, State: record.State}, nil
}
