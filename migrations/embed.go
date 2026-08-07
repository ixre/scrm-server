// Package migrations 嵌入手工迁移 SQL（*.sql），供 cmd/migrate 与 internal/migrate 使用。
package migrations

import "embed"

// Files 嵌入 migrations 目录下所有 .sql，运行时无需依赖 cwd 位置。
//
//go:embed *.sql
var Files embed.FS
