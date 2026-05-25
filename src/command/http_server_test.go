package command

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRedirectInstallRouteMiddlewareRedirectsInstallRoutes(t *testing.T) {
	e := echo.New()
	e.Use(redirectInstallRouteMiddleware)
	e.GET("/sign-in", func(c echo.Context) error {
		return c.String(http.StatusOK, "sign-in")
	})
	e.GET("/*", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	tests := []string{
		"/install",
		"/install/",
		"/install/step-two",
	}

	for _, path := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("%s status = %d, want 302", path, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "/sign-in" {
			t.Fatalf("%s Location = %q, want /sign-in", path, got)
		}
	}
}

func TestRedirectInstallRouteMiddlewareDoesNotRedirectOtherRoutes(t *testing.T) {
	e := echo.New()
	e.Use(redirectInstallRouteMiddleware)
	e.GET("/sign-in", func(c echo.Context) error {
		return c.String(http.StatusOK, "sign-in")
	})

	req := httptest.NewRequest(http.MethodGet, "/sign-in", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "sign-in" {
		t.Fatalf("body = %q, want sign-in", body)
	}
}
