package services

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strconv"

	"core/app/models"
	"core/common/util"
)

func applyArchiveConfig(cfg *models.CrmChannelConfig, req ChannelConfigReq) error {
	if req.MsgArchEnabled != 0 {
		if req.MsgArchEnabled != 1 && req.MsgArchEnabled != 2 {
			return errors.New("会话存档启用状态无效")
		}
		cfg.MsgArchEnabled = req.MsgArchEnabled
	}
	if req.MsgArchPrivateKeys != "" {
		if len(req.MsgArchPrivateKeys) > 128<<10 {
			return errors.New("存档私钥配置过大")
		}
		var keys map[string]string
		if err := json.Unmarshal([]byte(req.MsgArchPrivateKeys), &keys); err != nil || len(keys) == 0 {
			return errors.New("存档私钥须为版本号到 PEM 私钥的 JSON 对象")
		}
		for version, value := range keys {
			v, err := strconv.ParseUint(version, 10, 32)
			if err != nil || v == 0 {
				return errors.New("存档私钥版本号无效")
			}
			block, _ := pem.Decode([]byte(value))
			if block == nil {
				return errors.New("存档私钥 PEM 格式无效")
			}
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
				if err != nil {
					return errors.New("存档私钥格式无效")
				}
				var ok bool
				key, ok = parsed.(*rsa.PrivateKey)
				if !ok {
					return errors.New("存档私钥须为 RSA 私钥")
				}
			}
			if key.Validate() != nil {
				return errors.New("存档 RSA 私钥无效")
			}
		}
		encoded, err := util.AESEncode(req.MsgArchPrivateKeys)
		if err != nil {
			return err
		}
		cfg.MsgArchPrivateKeys = encoded
	}
	if req.MsgArchSecret != "" {
		encoded, err := util.AESEncode(req.MsgArchSecret)
		if err != nil {
			return err
		}
		cfg.MsgArchSecret = encoded
	}
	if cfg.MsgArchEnabled == 1 && (cfg.Channel != "wecom" || cfg.MsgArchSecret == "" || cfg.MsgArchPrivateKeys == "") {
		return errors.New("启用企微会话存档须配置存档 Secret 和版本私钥")
	}
	return nil
}
