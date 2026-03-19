package usecase

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aroundme/aroundme-backend/internal/entity"
	"github.com/aroundme/aroundme-backend/internal/model"
)

var matchLabelOrder = []string{
	"medical",
	"transport",
	"childcare",
	"pets",
	"food",
	"tools",
	"repair",
	"power",
	"internet",
	"shelter",
	"cleanup",
	"translation",
	"moving",
	"tech",
	"water",
	"clothing",
	"books",
	"garden",
	"sports",
}

var matchLabelAliases = map[string][]string{
	"medical":     {"medical", "doctor", "nurse", "medic", "medicine", "pharmacy", "first aid"},
	"transport":   {"transport", "ride", "rides", "driver", "driving", "pickup", "dropoff", "delivery", "car", "bike"},
	"childcare":   {"childcare", "babysit", "babysitting", "babysitter", "kids", "children", "toddler", "baby"},
	"pets":        {"pets", "pet", "dog", "dogs", "cat", "cats", "pet sitting", "pet sitter", "vet"},
	"food":        {"food", "meal", "meals", "grocery", "groceries", "cooking", "cook", "kitchen"},
	"tools":       {"tools", "tool", "drill", "ladder", "hammer", "saw", "wrench", "screwdriver", "pliers"},
	"repair":      {"repair", "fix", "fixing", "handyman", "plumber", "electrician", "mechanic"},
	"power":       {"power", "charger", "charging", "battery", "generator", "flashlight", "extension cord", "powerbank"},
	"internet":    {"internet", "wifi", "wi fi", "wi-fi", "hotspot", "router"},
	"shelter":     {"shelter", "room", "bed", "hosting", "host", "couch", "blanket", "blankets", "stay"},
	"cleanup":     {"cleanup", "cleaning", "clean", "snow", "shovel", "broom", "mop", "debris", "trash"},
	"translation": {"translation", "translate", "translator", "interpreter", "language"},
	"moving":      {"moving", "move", "boxes", "box", "lifting", "carry", "hauling"},
	"tech":        {"tech", "computer", "laptop", "phone", "printer", "tablet", "software"},
	"water":       {"water", "bottle", "bottled water", "hydration"},
	"clothing":    {"clothing", "clothes", "jacket", "coat", "shoes", "shirt"},
	"books":       {"books", "book", "textbook", "notebook"},
	"garden":      {"garden", "gardening", "plants", "plant", "soil", "watering"},
	"sports":      {"sports", "sport", "ball", "racket", "fitness", "exercise"},
}

var itemCategoryMatchLabels = map[string][]string{
	"tools":    {"tools", "repair"},
	"clothing": {"clothing"},
	"books":    {"books"},
	"sports":   {"sports"},
	"kitchen":  {"food"},
	"home":     {"shelter"},
	"garden":   {"garden", "cleanup"},
}

func normalizeSkillTags(rawTags []string) ([]string, error) {
	cleaned := make([]string, 0, len(rawTags))
	seen := make(map[string]struct{}, len(rawTags))

	for _, raw := range rawTags {
		tag := compactMatchValue(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 50 {
			return nil, model.ValidationError{Message: fmt.Sprintf("skill tag %q is too long", tag)}
		}

		derived := deriveMatchLabelsFromText(raw)
		if len(derived) == 0 {
			derived = []string{tag}
		}

		for _, next := range derived {
			if _, exists := seen[next]; exists {
				continue
			}
			seen[next] = struct{}{}
			cleaned = append(cleaned, next)
		}
	}

	return cleaned, nil
}

func deriveItemMatchTags(item entity.ProfileItem) []string {
	labels := make([]string, 0, 6)
	seen := make(map[string]struct{}, 6)

	for _, label := range itemCategoryMatchLabels[strings.ToLower(strings.TrimSpace(item.Category))] {
		labels = appendUniqueLabel(labels, seen, label)
	}

	for _, label := range deriveMatchLabelsFromText(item.Name, item.Description) {
		labels = appendUniqueLabel(labels, seen, label)
	}

	return labels
}

func buildPostMatchTags(rawTags []string, title, body string) ([]string, error) {
	if len(rawTags) > maxPostTags {
		return nil, model.ValidationError{Message: "a post can have at most 8 tags"}
	}

	tags := make([]string, 0, maxPostTags)
	seen := make(map[string]struct{}, maxPostTags)

	for _, raw := range rawTags {
		tag := compactMatchValue(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 40 {
			return nil, model.ValidationError{Message: fmt.Sprintf("tag %q is too long", tag)}
		}

		derived := deriveMatchLabelsFromText(raw)
		if len(derived) == 0 {
			derived = []string{tag}
		}

		for _, next := range derived {
			if len(tags) >= maxPostTags {
				return tags, nil
			}
			if _, exists := seen[next]; exists {
				continue
			}
			seen[next] = struct{}{}
			tags = append(tags, next)
		}
	}

	for _, next := range deriveMatchLabelsFromText(title, body) {
		if len(tags) >= maxPostTags {
			break
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		tags = append(tags, next)
	}

	return tags, nil
}

func deriveMatchLabelsFromText(parts ...string) []string {
	normalizedParts := make([]string, 0, len(parts))
	tokenSet := make(map[string]struct{}, 16)
	for _, part := range parts {
		normalized := normalizedSearchText(part)
		if normalized == "" {
			continue
		}
		normalizedParts = append(normalizedParts, normalized)
		for _, token := range strings.Fields(normalized) {
			tokenSet[token] = struct{}{}
		}
	}

	if len(normalizedParts) == 0 {
		return nil
	}

	searchText := " " + strings.Join(normalizedParts, " ") + " "
	labels := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)

	for _, label := range matchLabelOrder {
		aliases := matchLabelAliases[label]
		matched := false
		for _, alias := range aliases {
			normalizedAlias := compactMatchValue(alias)
			if normalizedAlias == "" {
				continue
			}
			if strings.Contains(normalizedAlias, " ") {
				if strings.Contains(searchText, " "+normalizedAlias+" ") {
					matched = true
					break
				}
				continue
			}
			if _, ok := tokenSet[normalizedAlias]; ok {
				matched = true
				break
			}
		}
		if matched {
			labels = appendUniqueLabel(labels, seen, label)
		}
	}

	return labels
}

func appendUniqueLabel(dst []string, seen map[string]struct{}, value string) []string {
	if value == "" {
		return dst
	}
	if _, exists := seen[value]; exists {
		return dst
	}
	seen[value] = struct{}{}
	return append(dst, value)
}

func compactMatchValue(value string) string {
	normalized := normalizedSearchText(value)
	if normalized == "" {
		return ""
	}
	return normalized
}

func normalizedSearchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	lastWasSpace := true
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if lastWasSpace {
			continue
		}
		builder.WriteByte(' ')
		lastWasSpace = true
	}

	return strings.TrimSpace(builder.String())
}
