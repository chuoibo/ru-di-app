package repo

import (
	"context"
	"time"
)

// ReportInput is create_report's keyword arguments. Note is stored as given;
// the domain has already stripped it and turned blank into nil.
type ReportInput struct {
	ReporterID string
	TargetType string
	TargetID   string
	Reason     string
	Note       *string
	Now        time.Time
}

// Report is ReportRecord: the id and the time, never the note.
type Report struct {
	ID        string
	CreatedAt time.Time
}

// CreateReport is create_report: one INSERT with a client-side uuid4 and the
// caller's clock (the column's server default is never used). The returned
// created_at is the value handed in, at the microsecond precision a Python
// datetime and the stored row both have.
//
// Nothing checks the reporter first: a reporter with no people row fails the
// flush on fk_reports_reporter, and that *pgconn.PgError is returned as is,
// which the Python route turns into a 500. target_id has no foreign key.
func (r Repository) CreateReport(ctx context.Context, in ReportInput) (Report, error) {
	id, err := newUUID()
	if err != nil {
		return Report{}, err
	}
	created := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO reports (id, reporter_id, target_type, target_id, reason, note, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::VARCHAR, $4::UUID, $5::VARCHAR, $6::VARCHAR, $7::TIMESTAMP WITH TIME ZONE)`,
		id, in.ReporterID, in.TargetType, in.TargetID, in.Reason, in.Note, created); err != nil {
		return Report{}, err
	}
	return Report{ID: id, CreatedAt: created}, nil
}
