package models

import "time"

type WecomSuiteTicket struct {
	SuiteID   string `gorm:"primaryKey;type:varchar(64)"`
	Ticket    string `gorm:"type:text" json:"-"`
	Timestamp int64
}

type WecomAuthState struct {
	StateHash   string `gorm:"primaryKey;type:varchar(64)" json:"-"`
	TenantID    string `gorm:"type:bigint"`
	UserID      string `gorm:"type:bigint"`
	SuiteID     string
	ExpiresAt   time.Time
	AuthCode    string `gorm:"type:text" json:"-"`
	CorpID      string
	CompletedAt *time.Time
	Attempts    int
	LastError   string
}

type WecomEventReceipt struct {
	ReceiptKey string `gorm:"primaryKey;type:varchar(64)"`
	TenantID   string `gorm:"type:bigint"`
	CreatedAt  time.Time
}
