package workwx

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"core/pkg/easywework/internal/lowlevel/encryptor"
	"core/pkg/easywework/internal/lowlevel/signature"
)

func TestCallbackVerifiesEncryptedReceiver(t *testing.T) {
	key:=strings.Repeat("a",43)
	enc,err:=encryptor.NewWorkWXEncryptor(key);if err!=nil{t.Fatal(err)}
	plain:=[]byte(`<xml><SuiteId>suite-a</SuiteId><InfoType>suite_ticket</InfoType></xml>`)
	encrypted,err:=enc.Encrypt(&encryptor.WorkWXPayload{Msg:plain,ReceiveID:[]byte("suite-a")});if err!=nil{t.Fatal(err)}
	sig:=signature.MakeDevMsgSignature("token","1","nonce",encrypted)
	for _,receiver:=range []string{"suite-a","suite-b"}{
		cb,err:=NewCBHandlerForReceiver("token",key,receiver);if err!=nil{t.Fatal(err)}
		req:=httptest.NewRequest("POST","/callback?timestamp=1&nonce=nonce&msg_signature="+sig,strings.NewReader(fmt.Sprintf("<xml><Encrypt><![CDATA[%s]]></Encrypt></xml>",encrypted)))
		data,err:=cb.GetCallBackXML(req)
		if receiver=="suite-a" {if err!=nil||string(data)!=string(plain){t.Fatal("valid callback rejected",err)}} else if err==nil {t.Fatal("callback intended for another receiver accepted")}
	}
}

func TestAuthorizedTokensAreEnterpriseLocal(t *testing.T){
	for _,corpID:=range []string{"corp-a","corp-b"}{
		calls:=0
		app:=New(corpID).WithApp("",10).UseAccessTokenProvider(func()(string,int64,error){calls++;return corpID+"-token",7200,nil})
		if app.accessToken.getToken()!=corpID+"-token"||app.accessToken.getToken()!=corpID+"-token"||calls!=1{t.Fatal("enterprise token cache mixed or repeatedly refreshed")}
	}
}
