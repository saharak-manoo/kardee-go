package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func lineServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("id_token") != "tok" || r.Form.Get("client_id") != "12345" {
			t.Errorf("bad request form: %v %v", err, r.Form)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLINEVerifier_OK(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	srv := lineServer(t, 200, `{"sub":"U1","name":"Ann","aud":"12345","exp":`+jsonInt(exp)+`}`)
	v := &LINEVerifier{ChannelID: "12345", Endpoint: srv.URL}
	id, err := v.Verify(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if id.Provider != ProviderLINE || id.Subject != "U1" || id.Name != "Ann" {
		t.Errorf("%+v", id)
	}
}

func TestLINEVerifier_Rejects(t *testing.T) {
	exp := jsonInt(time.Now().Add(time.Hour).Unix())
	past := jsonInt(time.Now().Add(-time.Hour).Unix())
	cases := map[string]struct {
		status int
		body   string
	}{
		"line says invalid": {400, `{"error":"invalid_request","error_description":"Invalid IdToken."}`},
		"other channel":     {200, `{"sub":"U1","aud":"999","exp":` + exp + `}`},
		"expired":           {200, `{"sub":"U1","aud":"12345","exp":` + past + `}`},
		"no subject":        {200, `{"aud":"12345","exp":` + exp + `}`},
	}
	for name, tc := range cases {
		srv := lineServer(t, tc.status, tc.body)
		v := &LINEVerifier{ChannelID: "12345", Endpoint: srv.URL}
		if _, err := v.Verify(context.Background(), "tok"); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func TestNewToken_HashMatches(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) < 40 || hash != HashToken(tok) || hash == tok {
		t.Errorf("token %q hash %q", tok, hash)
	}
	tok2, _, _ := NewToken()
	if tok2 == tok {
		t.Error("tokens repeat")
	}
}

func jsonInt(n int64) string { return strconv.FormatInt(n, 10) }
