package services

import (
	"core/app/constants"
	"core/app/entities"
	"core/app/models"
	"core/app/requests"
	"core/common/app"
	"core/common/ecode"
	"core/common/id_generator"
	"core/common/storage"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	url2 "net/url"
)

type QuickReply struct {
	QuickReplyGroup  models.QuickReplyGroup
	QuickReply       models.QuickReply
	QuickReplyDetail models.QuickReplyDetail
	httpClient       *resty.Client
}

func NewQuickReply() *QuickReply {
	return &QuickReply{
		QuickReplyGroup:  models.QuickReplyGroup{},
		QuickReply:       models.QuickReply{},
		httpClient:       resty.New(),
		QuickReplyDetail: models.QuickReplyDetail{},
	}
}

func (r QuickReply) Create(req entities.CreateQuickReplyReq, staff models.Staff) (*models.QuickReply, error) {
	quickReply := &models.QuickReply{
		ExtCorpModel:   models.ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: staff.ExtCorpID, ExtCreatorID: staff.ExtID},
		Name:           req.Name,
		GroupID:        req.GroupID,
		ExtStaffId:     staff.ExtID,
		StaffName:      staff.Name,
		SearchableText: []string{req.Name},
		QuickReplyType: req.QuickReplyDetails[0].ContentType,
	}
	if len(req.QuickReplyDetails) > 0 {
		quickReply.QuickReplyType = constants.QuickReplyTypeCollection
	}
	for _, replyDetail := range req.QuickReplyDetails {
		contentType, err := r.GenDetailType(replyDetail.ContentType)
		if err != nil {
			err = errors.WithStack(err)
			return nil, err
		}
		replyDetail.QuickReplyContent.MsgType = contentType
		detail := models.QuickReplyDetail{
			ExtCorpModel: models.ExtCorpModel{
				ID: id_generator.StringID(), ExtCorpID: staff.ExtCorpID, ExtCreatorID: staff.ExtID},
			QuickReplyID:      quickReply.ID,
			QuickReplyContent: replyDetail.QuickReplyContent,
			ContentType:       replyDetail.ContentType,
		}

		keyword, err := r.CreateKeyword(detail)
		if err != nil {
			err = errors.WithStack(err)
			return nil, err
		}

		quickReply.SearchableText = append(quickReply.SearchableText, keyword)
		quickReply.ReplyDetails = append(quickReply.ReplyDetails, detail)
	}

	return quickReply, r.QuickReply.Create(quickReply)
}

func (r QuickReply) QueryQuickReply(
	req requests.QueryQuickReplyReq, extCorpID string, sorter *app.Sorter, pager *app.Pager) (items []models.QuickReplyWithAvatar, total int64, err error) {

	items = make([]models.QuickReplyWithAvatar, 0)
	items, total, err = r.QuickReply.Query(req, extCorpID, sorter, pager)
	if err != nil {
		err = errors.WithStack(err)
		return nil, 0, err
	}

	return items, total, err
}

func (r QuickReply) Delete(ids []string, extCorpID string) (int64, error) {
	return r.QuickReply.Delete(ids, extCorpID)
}

func (r QuickReply) Update(req entities.UpdateQuickReplyReq, staff models.Staff) (*models.QuickReply, error) {
	reply := models.QuickReply{
		ExtCorpModel: models.ExtCorpModel{ID: req.ID, ExtCorpID: staff.ExtCorpID, ExtCreatorID: staff.ExtID},
		Name:         req.Name, GroupID: req.GroupID,
	}
	for _, item := range req.QuickReplyDetails {
		contentType, err := r.GenDetailType(item.ContentType)
		if err != nil {
			return nil, err
		}
		detail := models.QuickReplyDetail{
			ExtCorpModel: models.ExtCorpModel{ID: item.ID, ExtCorpID: staff.ExtCorpID, ExtCreatorID: staff.ExtID},
			ContentType:  item.ContentType, QuickReplyContent: item.QuickReplyContent,
		}
		if detail.ID == "" {
			detail.ID = id_generator.StringID()
		}
		detail.QuickReplyContent.MsgType = contentType
		reply.ReplyDetails = append(reply.ReplyDetails, detail)
	}
	if len(reply.ReplyDetails) > 0 {
		reply.QuickReplyType = constants.QuickReplyTypeCollection
	}
	result, err := r.QuickReply.UpdateWithDetails(reply, req.DeletedIDs)
	return &result, err
}

// IsFileExists 根据传地址生成文件地址，确定文件是否存在
func (r QuickReply) IsFileExists(fileUrl string) (isExists bool, err error) {
	url, err := url2.Parse(fileUrl)
	if err != nil {
		err = ecode.ParseFileUrlErr
		return
	}
	filename := url.Path
	return storage.FileStorage.IsExist(filename)
}

func (r QuickReply) CreateKeyword(detail models.QuickReplyDetail) (keyword string, err error) {
	switch detail.ContentType {
	case constants.QuickReplyTypeText:
		keyword = detail.QuickReplyContent.Text.Content
	case constants.QuickReplyTypePic:
		keyword = detail.QuickReplyContent.Image.Title
	case constants.QuickReplyTypePDF:
		keyword = detail.QuickReplyContent.Pdf.Title
	case constants.QuickReplyTypeNews:
		keyword = detail.QuickReplyContent.Link.Title
	case constants.QuickReplyTypeVideo:
		keyword = detail.QuickReplyContent.Video.Title
	default:
		return "", errors.New("unknown type")
	}
	return
}

// GenDetailType 2-文字 3-图片 4-网页 5-pdf 6-视频
func (r QuickReply) GenDetailType(contentType constants.QuickReplyType) (replyType string, err error) {
	switch contentType {
	case constants.QuickReplyTypeText:
		replyType = "text"
	case constants.QuickReplyTypePic:
		replyType = "image"
	case constants.QuickReplyTypeNews:
		replyType = "link"
	case constants.QuickReplyTypeVideo:
		replyType = "video"
	case constants.QuickReplyTypePDF:
		replyType = "pdf"
	default:
		err = errors.New("unknown type")
	}
	return
}
