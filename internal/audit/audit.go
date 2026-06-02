package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"gym-manager-v2/internal/store"
)

// Log writes an audit log entry.
func Log(ctx context.Context, q *store.Queries, userID int64, action string, details any) {
	var d sql.NullString
	if details != nil {
		b, _ := json.Marshal(details)
		d = sql.NullString{String: string(b), Valid: true}
	}
	uid := sql.NullInt64{}
	if userID > 0 {
		uid = sql.NullInt64{Int64: userID, Valid: true}
	}
	err := q.CreateAuditLog(ctx, store.CreateAuditLogParams{
		UserID:  uid,
		Action:  action,
		Details: d,
	})
	if err != nil {
		log.Printf("audit.Log error: %v", err)
	}
}
