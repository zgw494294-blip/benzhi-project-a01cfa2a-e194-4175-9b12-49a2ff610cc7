package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func writeProjections(ctx context.Context, tx *sql.Tx, c *domain.InspectionCase) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM findings WHERE case_id=?`, c.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM revisions WHERE case_id=?`, c.ID); err != nil {
		return err
	}
	for _, rev := range c.Revisions {
		encoded, err := json.Marshal(rev)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO revisions(id,case_id,revision_number,content_digest,storage_key,size_bytes,data_json) VALUES(?,?,?,?,?,?,?)`, rev.ID, c.ID, rev.RevisionNumber, rev.ContentDigest, rev.StorageKey, rev.SizeBytes, encoded); err != nil {
			return err
		}
	}
	for _, finding := range c.Findings {
		encoded, err := json.Marshal(finding)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO findings(id,case_id,revision_id,source,severity,status,data_json) VALUES(?,?,?,?,?,?,?)`, finding.ID, c.ID, nullable(finding.RevisionID), finding.Source, finding.Severity, finding.Status, encoded); err != nil {
			return err
		}
	}
	if c.Frozen != nil {
		encoded, err := json.Marshal(c.Frozen)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO frozen_snapshots(case_id,frozen_version,evidence_digest,data_json) VALUES(?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET frozen_version=excluded.frozen_version,evidence_digest=excluded.evidence_digest,data_json=excluded.data_json`, c.ID, c.Frozen.FrozenVersion, c.Frozen.EvidenceDigest, encoded); err != nil {
			return err
		}
	}
	if c.Credential != nil {
		encoded, err := json.Marshal(c.Credential)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO credentials(credential_number,case_id,verification_digest,data_json) VALUES(?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET credential_number=excluded.credential_number,verification_digest=excluded.verification_digest,data_json=excluded.data_json`, c.Credential.CredentialNumber, c.ID, c.Credential.VerificationDigest, encoded); err != nil {
			return err
		}
	}
	return nil
}

func insertEventAndResult(ctx context.Context, tx *sql.Tx, event domain.AuditEvent, key string, result []byte) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,case_id,action,actor,role,case_version,occurred_at,data_json) VALUES(?,?,?,?,?,?,?,?)`, event.ID, event.CaseID, event.Action, event.Actor, event.Role, event.Version, event.At.Format(time.RFC3339Nano), encoded)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO idempotency_results(idempotency_key,operation,result_json,created_at) VALUES(?,?,?,?)`, key, operationForAction(event.Action), result, event.At.Format(time.RFC3339Nano))
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
