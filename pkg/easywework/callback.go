package workwx

import (
	"core/pkg/easywework/internal/lowlevel/encryptor"
	"core/pkg/easywework/internal/lowlevel/envelope"
	"core/pkg/easywework/internal/lowlevel/httpapi"
	"core/pkg/easywework/internal/lowlevel/signature"
	"fmt"
	"io/ioutil"
	"net/http"
)

type CallBackHandler struct {
	token      string
	receiverID string
	encryptor  *encryptor.WorkWXEncryptor
	ep         *envelope.Processor
}

// NewCBHandlerForReceiver verifies the encrypted envelope's intended recipient.
func NewCBHandlerForReceiver(token, encodingAESKey, receiverID string) (*CallBackHandler, error) {
	cb, err := NewCBHandler(token, encodingAESKey)
	if err != nil {
		return nil, err
	}
	cb.receiverID = receiverID
	return cb, nil
}

func NewCBHandler(token string, encodingAESKey string) (*CallBackHandler, error) {
	enc, err := encryptor.NewWorkWXEncryptor(encodingAESKey)
	if err != nil {
		return nil, err
	}

	ep, err := envelope.NewProcessor(token, encodingAESKey)
	if err != nil {
		return nil, err
	}

	return &CallBackHandler{token: token, encryptor: enc, ep: ep}, nil
}

func (cb *CallBackHandler) GetCallBackXML(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		//rw.WriteHeader(http.StatusInternalServerError)
		return nil, err
	}

	// 验签
	// 解析Xml
	ev, err := cb.ep.HandleIncomingMsg(r.URL, body)
	if err != nil {
		return nil, err
	}
	if cb.receiverID != "" && string(ev.ReceiveID) != cb.receiverID {
		return nil, fmt.Errorf("callback receiver mismatch")
	}

	return ev.Msg, nil
}

func (cb *CallBackHandler) GetCallBackMsg(r *http.Request) (*RxMessage, error) {
	data, err := cb.GetCallBackXML(r)
	if err != nil {
		return nil, err
	}
	return ParseCallbackMessage(data)
}

func ParseCallbackMessage(data []byte) (*RxMessage, error) { return fromEnvelope(data) }

// EchoTestHandler
// wx后台配置服务器ip，回显
func (cb *CallBackHandler) EchoTestHandler(rw http.ResponseWriter, r *http.Request) {
	url := r.URL

	if !signature.VerifyHTTPRequestSignature(cb.token, url, "") {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	adapter := httpapi.URLValuesForEchoTestAPI(url.Query())
	args, err := adapter.ToEchoTestAPIArgs()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	payload, err := cb.encryptor.Decrypt([]byte(args.EchoStr))
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	if cb.receiverID != "" && string(payload.ReceiveID) != cb.receiverID {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(payload.Msg)
}
