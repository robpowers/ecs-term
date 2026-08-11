package views

import (
	"strings"
	"testing"
	"time"

	"github.com/robpowers/ecs-term/internal/domain"
)

func TestServiceMatches(t *testing.T) {
	svc := domain.ECSService{
		Name:         "checkout-api",
		Status:       "ACTIVE",
		DesiredCount: 3,
		RunningCount: 2,
		PendingCount: 1,
	}
	cases := []struct {
		query string
		want  bool
	}{
		{"checkout", true},
		{"ACTIVE", true},
		{"active", true}, // case-insensitive
		{"3", true},      // desired count
		{"billing", false},
	}
	for _, c := range cases {
		if got := serviceMatches(svc, strings.ToLower(c.query)); got != c.want {
			t.Errorf("serviceMatches(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestFilterServicesEmptyQueryReturnsAll(t *testing.T) {
	items := []domain.ECSService{{Name: "a"}, {Name: "b"}}
	got := filterServices(items, "")
	if len(got) != 2 {
		t.Fatalf("filterServices with empty query should return all items, got %d", len(got))
	}
}

func TestSortServicesInPlace(t *testing.T) {
	base := []domain.ECSService{
		{Name: "zeta", Status: "ACTIVE", RunningCount: 1},
		{Name: "alpha", Status: "PENDING", RunningCount: 3},
		{Name: "mid", Status: "STOPPED", RunningCount: 2},
	}

	t.Run("name ascending", func(t *testing.T) {
		items := append([]domain.ECSService(nil), base...)
		sortServicesInPlace(items, "N", true)
		want := []string{"alpha", "mid", "zeta"}
		for i, w := range want {
			if items[i].Name != w {
				t.Fatalf("index %d: got %q, want %q", i, items[i].Name, w)
			}
		}
	})

	t.Run("name descending", func(t *testing.T) {
		items := append([]domain.ECSService(nil), base...)
		sortServicesInPlace(items, "N", false)
		want := []string{"zeta", "mid", "alpha"}
		for i, w := range want {
			if items[i].Name != w {
				t.Fatalf("index %d: got %q, want %q", i, items[i].Name, w)
			}
		}
	})

	t.Run("running count ascending", func(t *testing.T) {
		items := append([]domain.ECSService(nil), base...)
		sortServicesInPlace(items, "R", true)
		want := []int32{1, 2, 3}
		for i, w := range want {
			if items[i].RunningCount != w {
				t.Fatalf("index %d: got %d, want %d", i, items[i].RunningCount, w)
			}
		}
	})

	t.Run("unknown key is a no-op", func(t *testing.T) {
		items := append([]domain.ECSService(nil), base...)
		sortServicesInPlace(items, "", true)
		for i, s := range base {
			if items[i].Name != s.Name {
				t.Fatalf("expected original order preserved, index %d got %q", i, items[i].Name)
			}
		}
	})
}

func TestTaskMatches(t *testing.T) {
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task := domain.ECSTask{
		ShortID:    "abc123",
		LastStatus: "RUNNING",
		StartedAt:  &started,
		Containers: []domain.ContainerSummary{{Name: "web"}, {Name: "sidecar"}},
	}
	startedDate := started.Local().Format("2006-01-02")
	cases := []struct {
		query string
		want  bool
	}{
		{"abc123", true},
		{"running", true},
		{"web", true},
		{startedDate, true},
		{"nope", false},
	}
	for _, c := range cases {
		if got := taskMatches(task, strings.ToLower(c.query)); got != c.want {
			t.Errorf("taskMatches(%q) = %v, want %v", c.query, got, c.want)
		}
	}
}

func TestSortTasksInPlaceByAgeHandlesNilStartedAt(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	items := []domain.ECSTask{
		{ShortID: "no-start", StartedAt: nil},
		{ShortID: "newer", StartedAt: &newer},
		{ShortID: "older", StartedAt: &older},
	}
	sortTasksInPlace(items, "A", true)
	want := []string{"no-start", "older", "newer"}
	for i, w := range want {
		if items[i].ShortID != w {
			t.Fatalf("index %d: got %q, want %q", i, items[i].ShortID, w)
		}
	}
}

func TestDeploymentMatches(t *testing.T) {
	d := domain.Deployment{ID: "ecs-svc/123", Status: "PRIMARY", RolloutState: "COMPLETED"}
	if !deploymentMatches(d, "primary") {
		t.Errorf("expected match on status")
	}
	if !deploymentMatches(d, "completed") {
		t.Errorf("expected match on rollout state")
	}
	if deploymentMatches(d, "failed") {
		t.Errorf("expected no match")
	}
}
