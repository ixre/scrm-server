package models

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func loadQuickReplyDetails(db *gorm.DB, replies []*QuickReply) error {
	parents := make([]ExtCorpModel, 0, len(replies))
	byID := make(map[parentKey]*QuickReply, len(replies))
	for _, reply := range replies {
		parents = append(parents, reply.ExtCorpModel)
		byID[aggregateKey(reply.ExtCorpID, reply.ID)] = reply
		reply.ReplyDetails = []QuickReplyDetail{}
	}
	details, err := loadCorpChildren[QuickReplyDetail](db, "quick_reply_id", parents, "id")
	if err != nil {
		return err
	}
	for _, detail := range details {
		p := byID[aggregateKey(detail.ExtCorpID, detail.QuickReplyID)]
		p.ReplyDetails = append(p.ReplyDetails, detail)
	}
	return nil
}

func loadQuickReplyGroups(db *gorm.DB, groups []QuickReplyGroup, replyIDs []string) error {
	parents := make([]ExtCorpModel, 0, len(groups))
	byID := make(map[parentKey]*QuickReplyGroup, len(groups))
	for i := range groups {
		p := &groups[i]
		parents = append(parents, p.ExtCorpModel)
		byID[aggregateKey(p.ExtCorpID, p.ID)] = p
		p.QuickReplies = []QuickReply{}
	}
	replyDB := db
	if replyIDs != nil {
		replyDB = db.Where("id IN ?", replyIDs)
	}
	replies, err := loadCorpChildren[QuickReply](replyDB, "group_id", parents, "id")
	if err != nil {
		return err
	}
	ptrs := make([]*QuickReply, len(replies))
	for i := range replies {
		ptrs[i] = &replies[i]
	}
	if err := loadQuickReplyDetails(db, ptrs); err != nil {
		return err
	}
	for _, reply := range replies {
		p := byID[aggregateKey(reply.ExtCorpID, reply.GroupID)]
		p.QuickReplies = append(p.QuickReplies, reply)
	}
	return nil
}

func writeQuickReplyDetails(tx *gorm.DB, reply *QuickReply) error {
	ids := make([]string, 0, len(reply.ReplyDetails))
	for i := range reply.ReplyDetails {
		d := &reply.ReplyDetails[i]
		d.QuickReplyID, d.ExtCorpID = reply.ID, reply.ExtCorpID
		if d.ExtCreatorID == "" {
			d.ExtCreatorID = reply.ExtCreatorID
		}
		ids = append(ids, d.ID)
	}
	return writeOwnedRows(tx, ownedRows{"quick_reply_id", reply.ID, reply.ExtCorpID}, reply.ReplyDetails, ids,
		[]string{"quick_reply_content", "content_type", "scope", "updated_at"}, false)
}

func createQuickReply(tx *gorm.DB, reply *QuickReply) error {
	if err := tx.Create(reply).Error; err != nil {
		return err
	}
	return writeQuickReplyDetails(tx, reply)
}

// UpdateWithDetails commits the parent, detail edits and explicit deletions as
// one operation. IDs supplied by clients cannot reparent another reply's detail.
func (q QuickReply) UpdateWithDetails(reply QuickReply, deletedIDs []string) (QuickReply, error) {
	var result QuickReply
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND ext_corp_id = ?", reply.ID, reply.ExtCorpID).First(&result).Error; err != nil {
			return err
		}
		owner := ownedRows{"quick_reply_id", result.ID, result.ExtCorpID}
		if err := owner.validateIDs(tx, &QuickReplyDetail{}, deletedIDs); err != nil {
			return err
		}
		if len(deletedIDs) > 0 {
			if err := owner.scope(tx).Where("id IN ?", deletedIDs).Delete(&QuickReplyDetail{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&result).Omit("ext_creator_id", "created_at").Updates(&reply).Error; err != nil {
			return err
		}
		if err := writeQuickReplyDetails(tx, &reply); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND ext_corp_id = ?", result.ID, result.ExtCorpID).First(&result).Error; err != nil {
			return err
		}
		return loadQuickReplyDetails(tx, []*QuickReply{&result})
	})
	return result, err
}

func createQuickReplyGroup(tx *gorm.DB, group *QuickReplyGroup) error {
	if err := tx.Create(group).Error; err != nil {
		return err
	}
	for i := range group.SubGroups {
		child := &group.SubGroups[i]
		child.ParentID, child.ExtCorpID = &group.ID, group.ExtCorpID
		if err := createQuickReplyGroup(tx, child); err != nil {
			return err
		}
	}
	for i := range group.QuickReplies {
		reply := &group.QuickReplies[i]
		reply.GroupID, reply.ExtCorpID = group.ID, group.ExtCorpID
		if err := createQuickReply(tx, reply); err != nil {
			return err
		}
	}
	return nil
}
