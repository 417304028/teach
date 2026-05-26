package store

import (
	"testing"

	"hermesclaw/internal/model"
)

func TestMaterialWhere_Empty(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{}, 1)
	if where != "" {
		t.Errorf("expected empty where clause, got %q", where)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestMaterialWhere_Season(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{Season: "春季课"}, 1)
	if !containsSubstring(where, "m.season") {
		t.Errorf("where clause missing season filter: %q", where)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
	if args[0] != "春季课" {
		t.Errorf("arg = %v", args[0])
	}
}

func TestMaterialWhere_Multiple(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{
		Season:   "春季课",
		LessonNo: 10,
		Track:    "新授",
	}, 1)
	if !containsSubstring(where, "m.season") {
		t.Error("missing season")
	}
	if !containsSubstring(where, "m.lesson_no") {
		t.Error("missing lesson_no")
	}
	if !containsSubstring(where, "m.track") {
		t.Error("missing track")
	}
	if !containsSubstring(where, " and ") {
		t.Error("filters should be joined with AND")
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestMaterialWhere_CustomStartIndex(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{Season: "春季课"}, 3)
	if !containsSubstring(where, "$3") {
		t.Errorf("expected $3 placeholder, got %q", where)
	}
	if args[0] != "春季课" {
		t.Errorf("arg = %v", args[0])
	}
}

func TestMaterialWhere_Version(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{Version: "教师版"}, 1)
	if !containsSubstring(where, "m.version") {
		t.Errorf("missing version filter")
	}
	if args[0] != "教师版" {
		t.Errorf("arg = %v", args[0])
	}
}

func TestMaterialWhere_MaterialKind(t *testing.T) {
	where, args := materialWhere(model.SearchFilters{MaterialKind: "讲义"}, 1)
	if !containsSubstring(where, "m.material_kind") {
		t.Errorf("missing material_kind filter")
	}
	if args[0] != "讲义" {
		t.Errorf("arg = %v", args[0])
	}
}

func TestVectorLiteral(t *testing.T) {
	vec := []float64{0.1, -0.2, 0.3}
	lit := vectorLiteral(vec)
	expected := "[0.1,-0.2,0.3]"
	if lit != expected {
		t.Errorf("vectorLiteral = %q, want %q", lit, expected)
	}
}

func TestVectorLiteral_Empty(t *testing.T) {
	vec := []float64{}
	lit := vectorLiteral(vec)
	if lit != "[]" {
		t.Errorf("vectorLiteral = %q", lit)
	}
}

func TestVectorLiteral_SingleElement(t *testing.T) {
	vec := []float64{1.0}
	lit := vectorLiteral(vec)
	if lit != "[1]" {
		t.Errorf("vectorLiteral = %q", lit)
	}
}

func TestNewID(t *testing.T) {
	id := NewID("mat")
	if id == "" {
		t.Error("NewID returned empty string")
	}
	if len(id) < 3 {
		t.Errorf("NewID too short: %q", id)
	}
}

func TestNewID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewID("test")
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestLexicalScore(t *testing.T) {
	material := model.Material{
		LessonTitle: "动能定理的应用",
		Season:      "春季课",
		Edition:     "人教版",
	}
	score := lexicalScore("动能定理", "动能定理的应用", material)
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}
}

func TestLexicalScore_NoMatch(t *testing.T) {
	material := model.Material{
		LessonTitle: "光的折射",
		Season:      "春季课",
	}
	score := lexicalScore("动能定理", "光的折射", material)
	if score > 0 {
		t.Errorf("expected zero score for unrelated query, got %f", score)
	}
}

func TestLexicalScore_SeasonMatch(t *testing.T) {
	material := model.Material{
		LessonTitle: "动能定理",
		Season:      "春季课",
	}
	score := lexicalScore("春季", "动能定理", material)
	if score <= 0 {
		t.Errorf("expected positive score for season match, got %f", score)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
