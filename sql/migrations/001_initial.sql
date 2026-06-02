-- +goose Up
CREATE TABLE users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    login           TEXT UNIQUE NOT NULL,
    name            TEXT NOT NULL,
    surname         TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    password_hash   TEXT NOT NULL,
    created_at      TEXT DEFAULT (datetime('now'))
);

CREATE TABLE membership_types (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    description     TEXT,
    duration_value  INTEGER NOT NULL,
    duration_unit   TEXT NOT NULL CHECK (duration_unit IN ('day', 'month', 'year')),
    is_contract     INTEGER DEFAULT 0,
    price           REAL NOT NULL,
    is_active       INTEGER DEFAULT 1,
    max_freeze_days INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT DEFAULT (datetime('now'))
);

CREATE TABLE clients (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    surname         TEXT NOT NULL,
    email           TEXT,
    phone           TEXT,
    comment         TEXT,
    alert_note      TEXT NOT NULL DEFAULT '',
    rfid_tag        TEXT UNIQUE,
    created_at      TEXT DEFAULT (datetime('now'))
);

CREATE TABLE memberships (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id           INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    type_id             INTEGER NOT NULL REFERENCES membership_types(id),
    starts_at           TEXT NOT NULL,
    ends_at             TEXT NOT NULL,
    is_active           INTEGER DEFAULT 1,
    frozen_at           TEXT,
    frozen_until        TEXT,
    total_frozen_days   INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT DEFAULT (datetime('now'))
);

CREATE TABLE payments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    membership_id   INTEGER NOT NULL REFERENCES memberships(id) ON DELETE CASCADE,
    due_date        TEXT NOT NULL,
    paid_at         TEXT,
    amount          REAL NOT NULL
);

CREATE TABLE entries (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id       INTEGER NOT NULL REFERENCES clients(id),
    recorded_by     INTEGER REFERENCES users(id),
    method          TEXT DEFAULT 'rfid',
    created_at      TEXT DEFAULT (datetime('now'))
);

CREATE TABLE audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER REFERENCES users(id),
    action          TEXT NOT NULL,
    details         TEXT,
    created_at      TEXT DEFAULT (datetime('now'))
);

-- Indexes
CREATE INDEX idx_clients_rfid ON clients(rfid_tag);
CREATE INDEX idx_memberships_client ON memberships(client_id);
CREATE INDEX idx_entries_created ON entries(created_at);
CREATE INDEX idx_entries_client ON entries(client_id);
CREATE INDEX idx_payments_membership ON payments(membership_id);
CREATE INDEX idx_audit_created ON audit_log(created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS membership_types;
DROP TABLE IF EXISTS users;
