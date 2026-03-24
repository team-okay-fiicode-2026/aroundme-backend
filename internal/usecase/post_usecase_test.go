package usecase

import (
	"testing"

	"github.com/aroundme/aroundme-backend/internal/entity"
)

func TestNormalizePostCategoryAcceptsLegacyAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  entity.PostCategory
	}{
		{input: "resource", want: entity.PostCategoryRequest},
		{input: "skill", want: entity.PostCategoryRequest},
		{input: "event", want: entity.PostCategoryEvent},
		{input: "community", want: entity.PostCategoryEvent},
		{input: "uncategorized", want: entity.PostCategoryUncategorized},
		{input: "request", want: entity.PostCategoryRequest},
		{input: "offer", want: entity.PostCategoryOffer},
		{input: "item", want: entity.PostCategoryItem},
		{input: "emergency", want: entity.PostCategoryEmergency},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := normalizePostCategory(tc.input, false)
			if err != nil {
				t.Fatalf("normalizePostCategory(%q): %v", tc.input, err)
			}
			if *got != tc.want {
				t.Fatalf("normalizePostCategory(%q) = %q, want %q", tc.input, *got, tc.want)
			}
		})
	}
}

func TestNormalizePostCategoryIgnoresUnknownOptionalValue(t *testing.T) {
	t.Parallel()

	category, err := normalizePostCategory("help", true)
	if err != nil {
		t.Fatalf("normalize optional category: %v", err)
	}
	if category != nil {
		t.Fatalf("optional category = %v, want nil", *category)
	}
}
