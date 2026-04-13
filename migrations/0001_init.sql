-- +goose Up
CREATE TABLE repos (
    id           SERIAL PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    root_path    TEXT NOT NULL,
    indexed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE documents (
    id            SERIAL PRIMARY KEY,
    repo_id       INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    relative_path TEXT NOT NULL,
    language      TEXT NOT NULL,
    UNIQUE (repo_id, relative_path)
);

CREATE TABLE symbols (
    id             SERIAL PRIMARY KEY,
    repo_id        INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    scip_symbol    TEXT NOT NULL,
    display_name   TEXT NOT NULL,
    kind           TEXT NOT NULL,
    signature      TEXT,
    documentation  TEXT,
    UNIQUE (repo_id, scip_symbol)
);
CREATE INDEX idx_symbols_display_name ON symbols (display_name);

CREATE TABLE occurrences (
    id          SERIAL PRIMARY KEY,
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    symbol_id   INTEGER NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    start_line  INTEGER NOT NULL,
    start_col   INTEGER NOT NULL,
    end_line    INTEGER NOT NULL,
    end_col     INTEGER NOT NULL,
    role        TEXT NOT NULL
);
CREATE INDEX idx_occurrences_symbol_id ON occurrences (symbol_id);
CREATE INDEX idx_occurrences_document_id ON occurrences (document_id);

-- +goose Down
DROP TABLE IF EXISTS occurrences;
DROP TABLE IF EXISTS symbols;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS repos;
