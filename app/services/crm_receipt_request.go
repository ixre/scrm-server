package services

import (
	"core/app/models"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var receiptRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func validateReceiptRequestID(key string) error {
	if key != "" && !receiptRequestIDPattern.MatchString(key) {
		return errors.New("登记请求标识须为8至64位字母、数字、下划线或短横线")
	}
	return nil
}
func receiptRequestDigest(r models.CrmReceivables, customer, number string) (string, error) {
	cents, err := moneyCents(r.Amount)
	if err != nil {
		return "", err
	}
	var date *time.Time
	if r.ReceivedAt != nil {
		utc := r.ReceivedAt.UTC()
		date = &utc
	}
	attachments := []string(r.Attachments)
	if attachments == nil {
		attachments = []string{}
	}
	payload := struct {
		Order, Customer, Plan            string
		Cents                            int64
		Date                             *time.Time
		Method, Account, Voucher         string
		Attachments                      []string
		Status, Approval, Remark, Number string
	}{r.OrderID, customer, r.PlanID, cents, date, r.PayMethod, r.PayAccount, r.VoucherNo, attachments, r.Status, r.ApproveStatus, r.Remark, number}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
