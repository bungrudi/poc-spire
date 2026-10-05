package control

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestControlPeerAndProtocolCannotBecomeArbitraryRequest(t *testing.T) {
	called := 0
	handler := endpoint(10002, func(ctx context.Context, id string) Result { called++; return Result{RequestID: id} })
	for _, test := range []struct {
		uid    uint32
		body   string
		status int
	}{
		{1000, `{"request_id":"ok"}`, 403},
		{10002, `{"request_id":"ok","url":"http://attacker"}`, 400},
		{10002, `{"request_id":"ok","amount_minor":200}`, 400},
		{10002, `{"request_id":"ok"} {}`, 400},
		{10002, `{"request_id":"ok"}`, 200},
	} {
		r := httptest.NewRequest("POST", "/attempt", strings.NewReader(test.body))
		r = r.WithContext(context.WithValue(r.Context(), uidKey{}, test.uid))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Error(w.Code, test.status)
		}
	}
	if called != 1 {
		t.Fatal("invalid command reached caller", called)
	}
}
