package routeops

import (
	"errors"
	"strings"
	"testing"
)

type fakeRoute struct {
	name string
	err  error
}

func TestDeleteRoutesContinueAggregatesErrors(t *testing.T) {
	routes := []fakeRoute{
		{name: "ok-1"},
		{name: "bad-1", err: errors.New("boom-1")},
		{name: "bad-2", err: errors.New("boom-2")},
	}

	partialErrs, err := DeleteRoutes(
		routes,
		func(route fakeRoute) error { return route.err },
		func(route fakeRoute) string { return route.name },
		ErrorActionContinue,
	)
	if err != nil {
		t.Fatalf("expected nil fatal error, got %v", err)
	}
	if len(partialErrs) != 2 {
		t.Fatalf("expected 2 partial errors, got %d", len(partialErrs))
	}
	if !strings.Contains(partialErrs[0].Error(), "bad-1") {
		t.Fatalf("expected first partial error to include route name, got %q", partialErrs[0])
	}
	if !strings.Contains(partialErrs[1].Error(), "boom-2") {
		t.Fatalf("expected second partial error to include underlying error, got %q", partialErrs[1])
	}
}

func TestDeleteRoutesStopReturnsFirstError(t *testing.T) {
	routes := []fakeRoute{
		{name: "ok-1"},
		{name: "bad-1", err: errors.New("boom-1")},
		{name: "bad-2", err: errors.New("boom-2")},
	}

	var deleted []string
	partialErrs, err := DeleteRoutes(
		routes,
		func(route fakeRoute) error {
			deleted = append(deleted, route.name)
			return route.err
		},
		func(route fakeRoute) string { return route.name },
		ErrorActionStop,
	)
	if partialErrs != nil {
		t.Fatalf("expected nil partial errors in stop mode, got %v", partialErrs)
	}
	if err == nil {
		t.Fatal("expected fatal error, got nil")
	}
	if !strings.Contains(err.Error(), "bad-1") {
		t.Fatalf("expected fatal error to include first failing route, got %q", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected deletion to stop after second route, got %d attempts", len(deleted))
	}
}
