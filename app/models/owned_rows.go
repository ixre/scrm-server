package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ownedRows describes a child collection owned by one aggregate. Callers must
// hold the parent row lock and run all writes in the parent's transaction.
// Column names are internal constants, never request parameters.
type ownedRows struct {
	parentColumn string
	parentID     string
	corpID       string
}

func (o ownedRows) scope(db *gorm.DB) *gorm.DB {
	return db.Where(o.parentColumn+" = ? AND ext_corp_id = ?", o.parentID, o.corpID)
}

func (o ownedRows) validateIDs(db *gorm.DB, model any, ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			return fmt.Errorf("child IDs must be nonempty and unique")
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return nil
	}
	var invalid int64
	err := db.Unscoped().Model(model).Where("id IN ?", ids).
		Where("("+o.parentColumn+" IS DISTINCT FROM ? OR ext_corp_id IS DISTINCT FROM ? OR deleted_at IS NOT NULL)", o.parentID, o.corpID).
		Count(&invalid).Error
	if err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("child record belongs to another parent or is deleted")
	}
	return nil
}

// writeOwnedRows explicitly upserts permitted fields and optionally removes
// missing children. It never nulls a parent ID or overwrites creation metadata.
func writeOwnedRows[T any](db *gorm.DB, owner ownedRows, rows []T, ids []string, columns []string, replace bool) error {
	var model T
	if err := owner.validateIDs(db, &model, ids); err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return fmt.Errorf("child row/ID count mismatch")
	}
	if len(rows) > 0 {
		assignments := clause.AssignmentColumns(columns)
		for i := range assignments {
			if assignments[i].Column.Name == "updated_at" {
				assignments[i].Value = db.NowFunc()
			}
		}
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: assignments,
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: owner.parentColumn}, Value: owner.parentID},
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "ext_corp_id"}, Value: owner.corpID},
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "deleted_at"}, Value: nil},
			}},
		}).CreateInBatches(&rows, 200)
		if result.Error != nil {
			return result.Error
		}
		// A conflicting ID inserted by another transaction must not be silently ignored.
		if result.RowsAffected != int64(len(rows)) {
			return fmt.Errorf("child record ownership changed")
		}
	}
	if !replace {
		return nil
	}
	query := owner.scope(db)
	if len(ids) > 0 {
		query = query.Where("id NOT IN ?", ids)
	}
	return query.Delete(&model).Error
}

// loadCorpChildren uses one query per child table, independent of list length.
// Both enterprise and parent ID participate in the match.
func loadCorpChildren[T any](db *gorm.DB, parentColumn string, parents []ExtCorpModel, order string) ([]T, error) {
	rows := make([]T, 0)
	if len(parents) == 0 {
		return rows, nil
	}
	keys := make([][]any, 0, len(parents))
	for _, parent := range parents {
		keys = append(keys, []any{parent.ExtCorpID, parent.ID})
	}
	query := db.Where("(ext_corp_id, "+parentColumn+") IN ?", keys)
	if order != "" {
		query = query.Order(order)
	}
	err := query.Find(&rows).Error
	return rows, err
}

type parentKey struct{ corpID, id string }

// PostgreSQL char(n) values are padded on reads, while request IDs are not.
func aggregateKey(corpID, id string) parentKey {
	return parentKey{strings.TrimSpace(corpID), id}
}
