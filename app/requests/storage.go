package requests

type GetUploadURLReq struct {
	// 文件名
	Filename string `json:"file_name" validate:"required"`
}

// GetUploadURLResp 上传文件地址
type GetUploadURLResp struct {
	// 上传地址
	UploadURL string `json:"upload_url"`
	// 下载地址
	DownloadURL string `json:"download_url"`
}

type LocalStorageFileReq struct {
	// ExpireAt 过期时间
	ExpireAt int64 `form:"expire_at" json:"expire_at" validate:"required"`
	// Signature hmac签名
	Signature string `form:"signature" json:"signature" validate:"required,alphanum"`
}

type GetSignedURLReq struct {
	ObjectKey    string `json:"object_key" validate:"required"`
	Method       string `json:"method" validate:"required,oneof=GET PUT"`
	ExpiredInSec int64  `json:"expired_in_sec" validate:"required"`
}
