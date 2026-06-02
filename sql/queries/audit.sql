-- name: CreateAuditLog :exec
INSERT INTO audit_log (user_id, action, details)
VALUES (?1, ?2, ?3);

-- name: ListAuditLogs :many
SELECT a.id, a.user_id, a.action, a.details, a.created_at,
       COALESCE(u.login, '—') AS user_login
FROM audit_log a
LEFT JOIN users u ON u.id = a.user_id
ORDER BY a.created_at DESC
LIMIT ?1 OFFSET ?2;

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_log;
