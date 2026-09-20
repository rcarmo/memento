package derived

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS concepts (
        id TEXT PRIMARY KEY,
        path TEXT NOT NULL UNIQUE,
        type TEXT NOT NULL,
        title TEXT NOT NULL,
        description TEXT,
        status TEXT NOT NULL,
        tags_json TEXT NOT NULL,
        aliases_json TEXT NOT NULL,
        body TEXT NOT NULL,
        content_hash TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        repo_revision TEXT NOT NULL
    )`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS concept_fts USING fts5(
        concept_id UNINDEXED,
        title,
        description,
        aliases,
        tags,
        body,
        path,
        tokenize='unicode61'
    )`,
	`CREATE TABLE IF NOT EXISTS links (
        source_id TEXT NOT NULL,
        target_id TEXT,
        raw_target TEXT NOT NULL,
        target_path TEXT,
        anchor TEXT,
        link_kind TEXT NOT NULL,
        resolution_state TEXT NOT NULL,
        first_seen_revision TEXT NOT NULL,
        last_checked_revision TEXT NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS graph_metrics (
        concept_id TEXT PRIMARY KEY,
        inbound_degree INTEGER NOT NULL,
        outbound_degree INTEGER NOT NULL,
        broken_link_count INTEGER NOT NULL,
        orphan_flag INTEGER NOT NULL
    )`,
	`CREATE TABLE IF NOT EXISTS index_state (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )`,
	"CREATE INDEX IF NOT EXISTS idx_concepts_path ON concepts(path)",
	"CREATE INDEX IF NOT EXISTS idx_concepts_type ON concepts(type)",
	"CREATE INDEX IF NOT EXISTS idx_concepts_status ON concepts(status)",
	"CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_id)",
	"CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_id)",
	`CREATE TABLE IF NOT EXISTS concept_embeddings (
        concept_id TEXT PRIMARY KEY,
        path TEXT NOT NULL,
        embedding_text_hash TEXT NOT NULL,
        model_id TEXT NOT NULL,
        dimensions INTEGER NOT NULL,
        embedding_revision TEXT NOT NULL,
        status TEXT NOT NULL,
        model_revision TEXT NOT NULL,
        embedding_blob BLOB,
        embedding_norm REAL,
        updated_at TEXT NOT NULL,
        error_message TEXT
    )`,
	"CREATE INDEX IF NOT EXISTS idx_concept_embeddings_path ON concept_embeddings(path)",
	"CREATE INDEX IF NOT EXISTS idx_concept_embeddings_status ON concept_embeddings(status)",
}
