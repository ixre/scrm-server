// Package dbutil 提供数据库错误判断的通用辅助函数，屏蔽底层驱动差异。
package dbutil

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgerrcode"
)

// IsUniqueViolation 判断 err 是否为 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
// 对应 MySQL 的 Error 1062 (ER_DUP_ENTRY)。
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.UniqueViolation
	}
	return false
}
