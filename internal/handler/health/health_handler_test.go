package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	healthconst "github.com/RijalArul/disbursement-race-condition/internal/constants/health"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
)

type fakePinger struct {
	err error
}

func (f *fakePinger) PingContext(context.Context) error { return f.err }

func newTestRouter(p Pinger) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", NewHandler(p).Check)
	return r
}

func doHealth(t *testing.T, p Pinger) (*httptest.ResponseRecorder, respconst.Envelope) {
	t.Helper()

	w := httptest.NewRecorder()
	newTestRouter(p).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	var env respconst.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not the standard envelope: %v (body: %s)", err, w.Body.String())
	}
	return w, env
}

func TestHealthReportsOKWhenDatabaseReachable(t *testing.T) {
	w, env := doHealth(t, &fakePinger{})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with a reachable database, got %d", w.Code)
	}
	if !env.Success {
		t.Error("expected success=true")
	}

	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected a data object, got %T", env.Data)
	}
	if data["status"] != healthconst.StatusOK || data["database"] != healthconst.DatabaseUp {
		t.Errorf("expected status=%q database=%q, got %v", healthconst.StatusOK, healthconst.DatabaseUp, data)
	}
}

func TestHealthReports503WhenDatabaseUnreachable(t *testing.T) {
	w, env := doHealth(t, &fakePinger{err: errors.New("dial tcp 127.0.0.1:5434: connect: connection refused")})

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when the database is unreachable, got %d", w.Code)
	}
	if env.Success {
		t.Error("expected success=false")
	}
	if env.Error == nil || env.Error.Code != healthconst.ErrCodeDatabaseUnavailable {
		t.Fatalf("expected error code %q, got %+v", healthconst.ErrCodeDatabaseUnavailable, env.Error)
	}
}

// The driver's error text can name hosts, ports, and credentials. It belongs in
// the server log, never in a response body an unauthenticated caller can read.
func TestHealthDoesNotLeakDriverErrorToClient(t *testing.T) {
	w, _ := doHealth(t, &fakePinger{err: errors.New("pq: password authentication failed for user \"postgres\"")})

	body := w.Body.String()
	for _, leak := range []string{"password", "postgres", "pq:"} {
		if strings.Contains(body, leak) {
			t.Errorf("driver detail %q leaked to the client: %s", leak, body)
		}
	}
}
