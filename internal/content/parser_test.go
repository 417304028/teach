package content

import "testing"

func TestParseSpringPath(t *testing.T) {
	m := ParseMaterialPath("春季课/人教版/第10讲 动能定理的应用/讲义/动能定理的应用(教师版).pdf")
	if m.Season != "春季课" || m.Edition != "人教版" || m.LessonNo != 10 || m.LessonTitle != "动能定理的应用" {
		t.Fatalf("unexpected metadata: %+v", m)
	}
	if m.MaterialKind != "讲义" || m.Version != "教师版" {
		t.Fatalf("unexpected type/version: %+v", m)
	}
}

func TestParseAutumnTrack(t *testing.T) {
	m := ParseMaterialPath("秋季课/人教版/新授/第10讲 受力分析与共点力的平衡/题集/题集：受力分析与共点力平衡(答案版).pdf")
	if m.Track != "新授" || m.MaterialKind != "题集" || m.Version != "答案版" {
		t.Fatalf("unexpected metadata: %+v", m)
	}
}
