package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authhttp "github.com/enterprise-cicd-platform/auth-service/internal/transport/http"
	"github.com/enterprise-cicd-platform/auth-service/internal/usecase"
)

// newTestHandlers wires all five use cases against the fakes in
// fakes_test.go. Refresh and Logout are passed as nil use cases — none of
// this file's tests call those two handlers' success paths, only
// Register/Login/Verify, which is exactly the set this session has a
// confirmed usecase signature for.
func newTestHandlers() *authhttp.Handlers {
	deps := usecase.Deps{
		Credentials:   newFakeCredentialRepo(),
		RefreshTokens: newFakeRefreshTokenRepo(),
		RefreshCache:  newFakeRefreshCache(),
		RateLimiter:   fakeRateLimiter{},
		Tokens:        fakeTokenIssuer{},
		Hasher:        fakeHasher{},
		Events:        fakeEventPublisher{},
		Now:           time.Now,
	}
	cfg := usecase.DefaultConfig()

	return authhttp.NewHandlers(
		usecase.NewRegister(deps),
		usecase.NewLogin(deps, cfg),
		nil,
		nil,
		usecase.NewVerifyToken(deps),
	)
}

func doJSON(t *testing.T, handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reqBody = bytes.NewReader(raw)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response body: %v (body: %s)", err, rec.Body.String())
	}
	return decoded
}

func TestRegister_Success(t *testing.T) {
	h := newTestHandlers()

	rec := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", map[string]string{
		"login_identifier": "alice@example.com",
		"password":         "hunter2-hunter2",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	body := decodeEnvelope(t, rec)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field missing or wrong shape: %#v", body["data"])
	}
	if data["user_id"] == "" || data["user_id"] == nil {
		t.Error("data.user_id is empty, want a UUID string")
	}
}

func TestRegister_DuplicateIdentifier(t *testing.T) {
	h := newTestHandlers()
	payload := map[string]string{"login_identifier": "bob@example.com", "password": "hunter2-hunter2"}

	first := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", payload)
	if first.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, want %d", first.Code, http.StatusCreated)
	}

	second := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", payload)
	if second.Code != http.StatusConflict {
		t.Fatalf("second register status = %d, want %d (body: %s)", second.Code, http.StatusConflict, second.Body.String())
	}
	body := decodeEnvelope(t, second)
	errBody := body["error"].(map[string]any)
	if errBody["code"] != "ACCOUNT_ALREADY_EXISTS" {
		t.Errorf("code = %v, want ACCOUNT_ALREADY_EXISTS", errBody["code"])
	}
}

func TestRegister_MissingFields(t *testing.T) {
	h := newTestHandlers()

	rec := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", map[string]string{"login_identifier": "onlyidentifier@example.com"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := decodeEnvelope(t, rec)
	errBody := body["error"].(map[string]any)
	if errBody["code"] != "VALIDATION_FAILED" {
		t.Errorf("code = %v, want VALIDATION_FAILED", errBody["code"])
	}
}

func TestRegister_MalformedJSON(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader([]byte("{not json")))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginAndVerify_RoundTrip(t *testing.T) {
	h := newTestHandlers()
	payload := map[string]string{"login_identifier": "carol@example.com", "password": "hunter2-hunter2"}

	registerRec := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", payload)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d (body: %s)", registerRec.Code, http.StatusCreated, registerRec.Body.String())
	}

	loginRec := doJSON(t, h.Login, http.MethodPost, "/v1/auth/login", payload)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d (body: %s)", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	loginBody := decodeEnvelope(t, loginRec)
	loginData := loginBody["data"].(map[string]any)
	accessToken, _ := loginData["access_token"].(string)
	if accessToken == "" {
		t.Fatal("login response missing access_token")
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+accessToken)
	verifyRec := httptest.NewRecorder()
	h.Verify(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want %d (body: %s)", verifyRec.Code, http.StatusOK, verifyRec.Body.String())
	}
	verifyBody := decodeEnvelope(t, verifyRec)
	verifyData := verifyBody["data"].(map[string]any)
	if verifyData["user_id"] != loginData["user_id"] {
		t.Errorf("verify user_id = %v, want %v (login's)", verifyData["user_id"], loginData["user_id"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newTestHandlers()
	registerPayload := map[string]string{"login_identifier": "dave@example.com", "password": "correct-password"}
	if rec := doJSON(t, h.Register, http.MethodPost, "/v1/auth/register", registerPayload); rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", rec.Code, http.StatusCreated)
	}

	loginRec := doJSON(t, h.Login, http.MethodPost, "/v1/auth/login", map[string]string{
		"login_identifier": "dave@example.com",
		"password":         "wrong-password",
	})

	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", loginRec.Code, http.StatusUnauthorized, loginRec.Body.String())
	}
	body := decodeEnvelope(t, loginRec)
	errBody := body["error"].(map[string]any)
	if errBody["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("code = %v, want INVALID_CREDENTIALS", errBody["code"])
	}
}

func TestVerify_MissingBearerToken(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()

	h.Verify(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// The following two tests exercise only the validation path of
// Refresh/Logout (decode + required-field checks), not their success
// path — see the ASSUMPTION FLAG comments in refresh_handler.go and
// logout_handler.go for why. A nil *usecase.Refresh/*usecase.Logout is
// safe here because these requests never reach h.refresh.Execute/
// h.logout.Execute — they're rejected by validation first.

func TestRefresh_MissingField(t *testing.T) {
	h := newTestHandlers()

	rec := doJSON(t, h.Refresh, http.MethodPost, "/v1/auth/refresh", map[string]string{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLogout_MissingBearerToken(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", bytes.NewReader([]byte(`{"refresh_token_id":"x"}`)))
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
