package util

import "testing"

func TestArchiveSignatureBindsEnterpriseAndPayload(t *testing.T){
	want:=ArchiveRequestSignature("key","100","GET","/messages","ext_corp_id=a&tenant_id=1",nil)
	for _,query:=range []string{"ext_corp_id=b&tenant_id=1","ext_corp_id=a&tenant_id=2"}{
		if ArchiveRequestSignature("key","100","GET","/messages",query,nil)==want{t.Fatal("archive identity is not signed")}
	}
	if ArchiveRequestSignature("key","100","POST","/messages","ext_corp_id=a&tenant_id=1",nil)==want{t.Fatal("method is not signed")}
	if ArchiveRequestSignature("key","100","GET","/other","ext_corp_id=a&tenant_id=1",nil)==want{t.Fatal("path is not signed")}
	if ArchiveRequestSignature("key","100","GET","/messages","ext_corp_id=a&tenant_id=1",[]byte("changed"))==want{t.Fatal("body is not signed")}
}
