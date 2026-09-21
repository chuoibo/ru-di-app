package routes

import (
	"context"
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/domain/reports"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// createReport is POST /reports (routes/reports.py create_report,
// ApiService.create_report). The target is not looked up on purpose, and a
// reporter without a people row fails on the foreign key as Python does: an
// unhandled 500.
func createReport() Route {
	return Route{ID: "POST /reports", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		refused, err := service.RequirePermission("file_report", *call.Actor, service.Resource{})
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
		targetType, err := stringField(request, "target_type")
		if err != nil {
			return endpoint.Reply{}, err
		}
		targetID, err := uuidField(request, "target_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		reason, err := stringField(request, "reason")
		if err != nil {
			return endpoint.Reply{}, err
		}
		note, err := optionalStringField(request, "note")
		if err != nil {
			return endpoint.Reply{}, err
		}
		fields, err := reports.ValidateReport(targetType, reason, note)
		var invalid *reports.ReportError
		if errors.As(err, &invalid) {
			return endpoint.Reply{}, endpoint.Refuse(422, strings.ToLower(invalid.Code), "Báo cáo chưa hợp lệ.")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := repo.Repository{Q: tx}.CreateReport(ctx, repo.ReportInput{
			ReporterID: call.Actor.ID,
			TargetType: fields.TargetType,
			TargetID:   targetID,
			Reason:     fields.Reason,
			Note:       fields.Note,
			Now:        time.Now().UTC(),
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("id", pyjson.String(record.ID))
		// The database runs in UTC, so psycopg hands Python an aware UTC value
		// and pydantic writes "Z"; pgx may return local time, hence UTC().
		body.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
		return endpoint.Reply{Body: body}, nil
	}}
}
