package store

import (
	"context"
	"encoding/json"
	"fmt"

	"benzhi-project-a01cfa2a-e194-4175-9b12-49a2ff610cc7/internal/domain"
)

func (r *SQLiteRepository) CheckIntegrity(ctx context.Context) error {
	var result string
	if err := r.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite 完整性检查失败: %s", result)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT aggregate_json FROM cases WHERE status IN ('frozen','released')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return err
		}
		var c domain.InspectionCase
		if err := json.Unmarshal(encoded, &c); err != nil {
			return err
		}
		if c.Frozen == nil || !c.Frozen.Verify() {
			return fmt.Errorf("任务 %s 的冻结证据摘要无效", c.ID)
		}
		if c.Credential != nil && !c.Credential.Verify(c.Frozen) {
			return fmt.Errorf("任务 %s 的放行凭据摘要无效", c.ID)
		}
	}
	return rows.Err()
}
