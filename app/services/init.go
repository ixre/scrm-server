package services

import (
 "context"
 "errors"
 "core/common/log"
)

// Syncs synchronizes every enabled enterprise, independently of other failures.
func Syncs() {
 err := EachWecomCorp(context.Background(), func(ctx context.Context, corpID string) error {
  return errors.Join(
   NewDepartment().Sync(ctx, corpID),
   NewStaffService().Sync(ctx, corpID),
   NewGroupChatService().SyncAll(ctx, corpID),
   NewCustomer().Sync(ctx, corpID),
   NewTag().Sync(corpID),
  )
 })
 if err != nil { log.Sugar.Errorw("sync WeCom enterprises failed", "err", err) }
}
