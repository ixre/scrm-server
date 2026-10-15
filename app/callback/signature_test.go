package callback

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestCallbacksRejectUnsignedRequests(t *testing.T) {
	for name, handler := range map[string]gin.HandlerFunc{"xhs": xhsCallback, "gzh": gzhCallback, "douyin": douyinCallback} {
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			r.POST("/callback/:app_id", handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/callback/test-app", nil))
			if w.Code != 401 {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
}
