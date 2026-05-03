package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service"
)

// mockAuthService implements AuthServicer for use in handler tests.
type mockAuthService struct {
	registerUser  *data.User
	registerToken string
	registerErr   error

	loginUser  *data.User
	loginToken string
	loginErr   error

	getUserByIDUser *data.User
	getUserByIDErr  error
}

func (m *mockAuthService) Register(_ context.Context, email, password string) (*data.User, string, error) {
	return m.registerUser, m.registerToken, m.registerErr
}
func (m *mockAuthService) Login(_ context.Context, email, password string) (*data.User, string, error) {
	return m.loginUser, m.loginToken, m.loginErr
}
func (m *mockAuthService) GetUserByID(_ context.Context, userID string) (*data.User, error) {
	return m.getUserByIDUser, m.getUserByIDErr
}
func (m *mockAuthService) VerifyEmail(_ context.Context, token string) error             { return nil }
func (m *mockAuthService) ResendVerificationEmail(_ context.Context, email string) error { return nil }
func (m *mockAuthService) LoginWithGoogle(_ context.Context, token string) (*data.User, string, error) {
	return nil, "", nil
}

// helpers

func devHandler(svc AuthServicer) *AccountHandler {
	return &AccountHandler{
		AuthService: svc,
		Config:      &config.Config{Environment: "development"},
	}
}

func fakeUser() *data.User {
	return &data.User{
		ID:        "user-1",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		Balance:   decimal.NewFromFloat(10000.0),
	}
}

func jsonBody(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// ---- Register ----

func TestRegister_InvalidJSON(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("not-json{"))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegister_MissingEmailPassword(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/register",
		jsonBody(t, RegisterRequest{Email: "", Password: ""}))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegister_MissingPassword(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/register",
		jsonBody(t, RegisterRequest{Email: "test@example.com", Password: ""}))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	h := devHandler(&mockAuthService{registerErr: &service.EmailExistsError{}})
	req := httptest.NewRequest(http.MethodPost, "/register",
		jsonBody(t, RegisterRequest{Email: "exists@example.com", Password: "Secret1!"}))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp AuthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestRegister_Success(t *testing.T) {
	h := devHandler(&mockAuthService{
		registerUser:  fakeUser(),
		registerToken: "jwt-abc",
	})
	req := httptest.NewRequest(http.MethodPost, "/register",
		jsonBody(t, RegisterRequest{Email: "new@example.com", Password: "Secret1!"}))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	// Cookie must be set
	var tokenCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "token" {
			tokenCookie = c
		}
	}
	if tokenCookie == nil {
		t.Fatal("expected token cookie")
	}
	if tokenCookie.Value != "jwt-abc" {
		t.Errorf("cookie value: got %q, want %q", tokenCookie.Value, "jwt-abc")
	}
}

// ---- Login ----

func TestLogin_MissingBody(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	h := devHandler(&mockAuthService{loginErr: &service.InvalidCredentialsError{}})
	req := httptest.NewRequest(http.MethodPost, "/login",
		jsonBody(t, LoginRequest{Email: "test@example.com", Password: "wrong"}))
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	var resp AuthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestLogin_Success(t *testing.T) {
	h := devHandler(&mockAuthService{
		loginUser:  fakeUser(),
		loginToken: "jwt-xyz",
	})
	req := httptest.NewRequest(http.MethodPost, "/login",
		jsonBody(t, LoginRequest{Email: "test@example.com", Password: "Secret1!"}))
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var tokenCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "token" {
			tokenCookie = c
		}
	}
	if tokenCookie == nil {
		t.Fatal("expected token cookie")
	}
	if tokenCookie.Value != "jwt-xyz" {
		t.Errorf("cookie value: got %q, want %q", tokenCookie.Value, "jwt-xyz")
	}
}

// ---- Logout ----

func TestLogout_AlwaysOK(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()
	h.Logout(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// Cookie must be present and cleared (empty value / expired)
	var tokenCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "token" {
			tokenCookie = c
		}
	}
	if tokenCookie == nil {
		t.Fatal("expected token cookie to be set (cleared)")
	}
	if tokenCookie.Value != "" {
		t.Errorf("cookie value should be empty, got %q", tokenCookie.Value)
	}
}

// ---- GetProfile ----

func TestGetProfile_MissingUserID(t *testing.T) {
	h := devHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()
	h.GetProfile(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetProfile_UserNotFound(t *testing.T) {
	h := devHandler(&mockAuthService{getUserByIDErr: errors.New("user not found")})
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("X-User-ID", "missing-user")
	w := httptest.NewRecorder()
	h.GetProfile(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetProfile_Success(t *testing.T) {
	h := devHandler(&mockAuthService{getUserByIDUser: fakeUser()})
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetProfile(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var user data.User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if user.ID != "user-1" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-1")
	}
}
