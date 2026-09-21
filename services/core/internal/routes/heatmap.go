package routes

import (
	"context"

	"mobile/services/core/internal/domain/socialmap"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// groupHeatmap is GET /contexts/{context_id}/heatmap (routes/social_map.py
// get_group_heatmap, ApiService.get_group_heatmap): districts and counts, with
// the scan ceiling and the check-ins no district claimed both disclosed.
func groupHeatmap() Route {
	return Route{ID: "GET /contexts/{context_id}/heatmap", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		refused, err := service.RequirePermission("view_group_heatmap", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_group_member": member}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}
		rows, truncated, err := scanCheckins(ctx, store, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		heatmap, err := socialmap.HeatmapRows(rows)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := pyjson.List{}
		resolved := int64(0)
		for _, row := range heatmap {
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(row.ID))
			entry.Set("label", pyjson.String(row.Label))
			entry.Set("lat", pyjson.Float(row.Lat))
			entry.Set("lng", pyjson.Float(row.Lng))
			entry.Set("visit_count", pyjson.NewInt(int64(row.VisitCount)))
			entry.Set("share_percent", pyjson.NewInt(int64(row.SharePercent)))
			list = append(list, entry)
			resolved += int64(row.VisitCount)
		}
		unknown, err := socialmap.UnknownAreaCount(rows)
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("areas", list)
		body.Set("resolved_checkins", pyjson.NewInt(resolved))
		body.Set("unknown_area_count", pyjson.NewInt(int64(unknown)))
		body.Set("scanned_checkins", pyjson.NewInt(int64(len(rows))))
		body.Set("truncated", pyjson.Bool(truncated))
		return endpoint.Reply{Body: body}, nil
	}}
}
