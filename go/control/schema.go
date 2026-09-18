// SQL copied from the pinned Memento control/db.py schema.
package control

const SchemaVersion = "10"

var migrationsV1 = []string{
	`
    CREATE TABLE IF NOT EXISTS operations (
        op_id TEXT PRIMARY KEY,
        idempotency_key TEXT NOT NULL,
        principal TEXT NOT NULL,
        client_instance_id TEXT,
        mcp_session_id TEXT,
        source_chat TEXT,
        tool_name TEXT NOT NULL,
        request_hash TEXT NOT NULL,
        base_revision TEXT,
        result_revision TEXT,
        state TEXT NOT NULL,
        request_json TEXT NOT NULL,
        result_json TEXT,
        error_class TEXT,
        error_message TEXT,
        created_at TEXT NOT NULL,
        started_at TEXT,
        finished_at TEXT,
        UNIQUE(principal, idempotency_key)
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS proposals (
        proposal_id TEXT PRIMARY KEY,
        author_principal TEXT NOT NULL,
        client_instance_id TEXT,
        base_revision TEXT NOT NULL,
        intent TEXT NOT NULL,
        rationale TEXT,
        patch_json TEXT NOT NULL,
        patch_hash TEXT NOT NULL,
        status TEXT NOT NULL,
        reviewed_by TEXT,
        review_comment TEXT,
        applied_operation_id TEXT,
        applied_revision TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        expires_at TEXT,
        FOREIGN KEY(applied_operation_id) REFERENCES operations(op_id)
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS scheduler_runs (
        run_id TEXT PRIMARY KEY,
        job_name TEXT NOT NULL,
        window_key TEXT NOT NULL,
        base_revision TEXT,
        end_revision TEXT,
        state TEXT NOT NULL,
        signal_count INTEGER NOT NULL DEFAULT 0,
        proposal_count INTEGER NOT NULL DEFAULT 0,
        model_chain_json TEXT,
        started_at TEXT NOT NULL,
        finished_at TEXT,
        error_message TEXT,
        UNIQUE(job_name, window_key)
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS service_state (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS dream_signals (
        signal_id TEXT PRIMARY KEY,
        signal_type TEXT NOT NULL,
        entity_refs_json TEXT NOT NULL,
        severity TEXT NOT NULL,
        repo_revision TEXT NOT NULL,
        dedupe_key TEXT NOT NULL UNIQUE,
        status TEXT NOT NULL,
        evidence_hash TEXT NOT NULL,
        evidence_json TEXT NOT NULL,
        first_detected_at TEXT NOT NULL,
        last_detected_at TEXT NOT NULL,
        resolved_revision TEXT
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_dream_signals_status ON dream_signals(status, signal_type)`,
}

var migrationsV6 = []string{
	`
    CREATE TABLE IF NOT EXISTS proposal_assets (
        proposal_id TEXT NOT NULL,
        asset_id TEXT NOT NULL,
        concept_path TEXT NOT NULL,
        asset_kind TEXT NOT NULL,
        version TEXT NOT NULL,
        media_type TEXT NOT NULL,
        sha256 TEXT NOT NULL,
        blob_bytes BLOB NOT NULL,
        manifest_json TEXT NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY (proposal_id, asset_id),
        FOREIGN KEY(proposal_id) REFERENCES proposals(proposal_id) ON DELETE CASCADE
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_proposal_assets_concept ON proposal_assets(concept_path, asset_kind, version)`,
	`CREATE INDEX IF NOT EXISTS idx_proposal_assets_created ON proposal_assets(created_at, proposal_id)`,
}

var migrationsV7 = []string{
	`
    CREATE TABLE IF NOT EXISTS access_meta (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS access_principals (
        name TEXT PRIMARY KEY,
        roles_json TEXT NOT NULL,
        read_prefixes_json TEXT NOT NULL,
        write_prefixes_json TEXT NOT NULL,
        enabled INTEGER NOT NULL,
        revoked INTEGER NOT NULL,
        deleted INTEGER NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS access_credentials (
        principal_name TEXT PRIMARY KEY,
        token_digest TEXT NOT NULL UNIQUE,
        created_at TEXT NOT NULL,
        revoked_at TEXT,
        FOREIGN KEY(principal_name) REFERENCES access_principals(name) ON UPDATE CASCADE
    )
    `,
	`
    CREATE TABLE IF NOT EXISTS access_audit (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        actor TEXT NOT NULL,
        action TEXT NOT NULL,
        target TEXT NOT NULL,
        details_json TEXT NOT NULL,
        created_at TEXT NOT NULL
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_access_audit_created ON access_audit(created_at DESC, id DESC)`,
	`
    CREATE TABLE IF NOT EXISTS access_idempotency (
        actor TEXT NOT NULL,
        idempotency_key TEXT NOT NULL,
        action TEXT NOT NULL,
        target TEXT NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY(actor, idempotency_key)
    )
    `,
}

var migrationsV8 = []string{
	`
    CREATE TABLE IF NOT EXISTS staged_assets (
        staged_asset_id TEXT PRIMARY KEY,
        principal TEXT NOT NULL,
        idempotency_key TEXT NOT NULL,
        asset_kind TEXT NOT NULL,
        version TEXT NOT NULL,
        media_type TEXT NOT NULL,
        sha256 TEXT NOT NULL,
        blob_bytes BLOB NOT NULL,
        manifest_json TEXT NOT NULL,
        state TEXT NOT NULL,
        proposal_id TEXT,
        created_at TEXT NOT NULL,
        expires_at TEXT NOT NULL,
        consumed_at TEXT,
        UNIQUE(principal, idempotency_key),
        FOREIGN KEY(proposal_id) REFERENCES proposals(proposal_id)
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_staged_assets_expiry ON staged_assets(state, expires_at)`,
}

var migrationsV9 = []string{
	`
    CREATE TABLE IF NOT EXISTS asset_upload_tickets (
        token_digest TEXT PRIMARY KEY,
        principal TEXT NOT NULL,
        idempotency_key TEXT NOT NULL,
        asset_kind TEXT NOT NULL,
        version TEXT NOT NULL,
        created_at TEXT NOT NULL,
        expires_at TEXT NOT NULL,
        staged_asset_id TEXT,
        consumed_at TEXT,
        UNIQUE(principal, idempotency_key),
        FOREIGN KEY(staged_asset_id) REFERENCES staged_assets(staged_asset_id)
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_asset_upload_ticket_expiry ON asset_upload_tickets(expires_at)`,
}

var migrationsV10 = []string{
	`
    CREATE TABLE IF NOT EXISTS proposal_events (
        event_id INTEGER PRIMARY KEY AUTOINCREMENT,
        proposal_id TEXT NOT NULL REFERENCES proposals(proposal_id),
        actor TEXT NOT NULL,
        action TEXT NOT NULL,
        from_status TEXT NOT NULL,
        to_status TEXT NOT NULL,
        base_revision TEXT NOT NULL,
        repo_revision TEXT NOT NULL,
        details_json TEXT NOT NULL,
        created_at TEXT NOT NULL
    )
    `,
	`CREATE INDEX IF NOT EXISTS idx_proposal_events ON proposal_events(proposal_id, event_id)`,
}
