-- name: ListClients :many
SELECT id, name, surname, email, phone, comment, alert_note, rfid_tag, created_at
FROM clients
ORDER BY surname, name
LIMIT ?1 OFFSET ?2;

-- name: CountClients :one
SELECT COUNT(*) FROM clients;

-- name: GetClient :one
SELECT id, name, surname, email, phone, comment, alert_note, rfid_tag, created_at
FROM clients
WHERE id = ?1;

-- name: SearchClients :many
SELECT id, name, surname, email, phone, comment, alert_note, rfid_tag, created_at
FROM clients
WHERE (name || ' ' || surname) LIKE '%' || @query || '%'
   OR phone LIKE '%' || @query || '%'
   OR email LIKE '%' || @query || '%'
ORDER BY surname, name
LIMIT 50;

-- name: CreateClient :one
INSERT INTO clients (name, surname, email, phone, comment, alert_note)
VALUES (?1, ?2, ?3, ?4, ?5, ?6)
RETURNING *;

-- name: UpdateClient :one
UPDATE clients
SET name = ?2, surname = ?3, email = ?4, phone = ?5, comment = ?6, alert_note = ?7
WHERE id = ?1
RETURNING *;

-- name: DeleteClient :exec
DELETE FROM clients WHERE id = ?1;

-- name: GetClientByRFID :one
SELECT id, name, surname, email, phone, comment, alert_note, rfid_tag, created_at
FROM clients
WHERE rfid_tag = ?1;

-- name: UpdateClientRFID :exec
UPDATE clients SET rfid_tag = ?2 WHERE id = ?1;

-- name: ClearClientAlert :exec
UPDATE clients SET alert_note = '' WHERE id = ?1;
