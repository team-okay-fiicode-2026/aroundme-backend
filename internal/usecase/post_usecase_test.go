package usecase

import (
	"testing"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

func TestNormalizePostCategoryAcceptsLegacyAliases(t *testing.T) {
	t.Parallel()

	resourceCategory, err := normalizePostCategory("resource", false)
	if err != nil {
		t.Fatalf("normalize resource: %v", err)
	}
	if got, want := *resourceCategory, entity.PostCategorySkill; got != want {
		t.Fatalf("resource category = %q, want %q", got, want)
	}

	eventCategory, err := normalizePostCategory("event", false)
	if err != nil {
		t.Fatalf("normalize event: %v", err)
	}
	if got, want := *eventCategory, entity.PostCategoryCommunity; got != want {
		t.Fatalf("event category = %q, want %q", got, want)
	}

	uncategorizedCategory, err := normalizePostCategory("uncategorized", false)
	if err != nil {
		t.Fatalf("normalize uncategorized: %v", err)
	}
	if got, want := *uncategorizedCategory, entity.PostCategoryUncategorized; got != want {
		t.Fatalf("uncategorized category = %q, want %q", got, want)
	}
}
