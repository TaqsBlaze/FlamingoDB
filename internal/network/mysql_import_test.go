package network

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMySQLImportAccessAndValidation(t *testing.T) {
	s, err := NewServer(Config{DataDir: t.TempDir(), Username: "admin", Password: "pass"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.userStore.CreateUser("reader", "pass", false, "", Policy{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		user, pass, body string
		status           int
	}{
		{"", "", "{}", http.StatusUnauthorized},
		{"reader", "pass", "{}", http.StatusForbidden},
		{"admin", "pass", "{}", http.StatusBadRequest},
		{"admin", "pass", `{"host":"localhost","port":70000,"database":"db","username":"u"}`, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/import/mysql", strings.NewReader(tc.body))
		if tc.user != "" {
			req.SetBasicAuth(tc.user, tc.pass)
		}
		w := httptest.NewRecorder()
		s.handleMySQLImport(w, req)
		if w.Code != tc.status {
			t.Errorf("user %q: got %d, want %d", tc.user, w.Code, tc.status)
		}
	}
}
