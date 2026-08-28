package callback

import (
	"context"
	"core/app/services"
	"core/common/identity"
	workwx "core/pkg/easywework"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
)

func handleVerifiedWecomEvent(tenantID string, msg *workwx.RxMessage, data []byte) error {
	var target struct {
		Customer string `xml:"ExternalUserID"`
		User     string `xml:"UserID"`
		Chat     string `xml:"ChatId"`
		ID       string `xml:"Id"`
	}
	if err := xml.Unmarshal(data, &target); err != nil {
		return err
	}
	entity := target.Customer
	if entity == "" {
		entity = target.Chat
	}
	if entity == "" {
		entity = target.User
	}
	if entity == "" {
		entity = target.ID
	}
	key := msg.ToUserID + ":" + string(msg.Event) + ":" + entity
	sum := sha256.Sum256(append([]byte(msg.ToUserID+":"), data...))
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	return services.RunWecomEvent(ctx, key, hex.EncodeToString(sum[:]), func() error { return dispatchWeComEvent(tenantID, msg) })
}
