package intent

import (
	"context"
	"testing"

	"hermesclaw/internal/model"
)

func TestClassifyPPT(t *testing.T) {
	result := New(nil).Classify(context.Background(), "用春季课第11讲生成12页PPT")
	if result.Intent != model.IntentPPT {
		t.Fatalf("intent = %s", result.Intent)
	}
	if result.Season != "春季课" || result.LessonNo != 11 || result.Pages != 12 {
		t.Fatalf("bad params: %+v", result)
	}
}

func TestClassifyExercises(t *testing.T) {
	result := New(nil).Classify(context.Background(), "出一套机械能守恒10道选择题带答案")
	if result.Intent != model.IntentExercises {
		t.Fatalf("intent = %s", result.Intent)
	}
	if result.Count != 10 {
		t.Fatalf("count = %d", result.Count)
	}
}

func TestClassifyOutline(t *testing.T) {
	result := New(nil).Classify(context.Background(), "帮我整理第3讲教学大纲")
	if result.Intent != model.IntentOutline {
		t.Fatalf("intent = %s", result.Intent)
	}
	if result.LessonNo != 3 {
		t.Fatalf("lesson = %d", result.LessonNo)
	}
}

func TestClassifyGenerateAll_题目做得不好(t *testing.T) {
	result := New(nil).Classify(context.Background(), "某个学生平抛运动的题目做的不好")
	if result.Intent != model.IntentGenerateAll {
		t.Fatalf("「题目做的不好」应触发 generate_all，实际 intent = %s，confidence = %.2f", result.Intent, result.Confidence)
	}
	if result.Confidence < 0.9 {
		t.Fatalf("confidence 应 >= 0.9, 实际 = %.2f", result.Confidence)
	}
}

func TestClassifyGenerateAll_出题(t *testing.T) {
	result := New(nil).Classify(context.Background(), "平抛运动这部分题目不好，需要出题")
	if result.Intent != model.IntentGenerateAll {
		t.Fatalf("「出题」+ 包含「不好」应触发 generate_all，实际 intent = %s", result.Intent)
	}
}

func TestClassifyGenerateAll_生成题目(t *testing.T) {
	result := New(nil).Classify(context.Background(), "学生动能定理做的不好，帮我生成题目")
	if result.Intent != model.IntentGenerateAll {
		t.Fatalf("「生成题目」+「做的不好」应触发 generate_all，实际 intent = %s", result.Intent)
	}
}

func TestClassifyGenerateAll_力学综合(t *testing.T) {
	result := New(nil).Classify(context.Background(), "春季课力学综合题做的差，需要生成题目和答案")
	if result.Intent != model.IntentGenerateAll {
		t.Fatalf("含「春季课」和「生成题目」场景应触发 generate_all，实际 intent = %s", result.Intent)
	}
}

func TestClassifyMindmap(t *testing.T) {
	result := New(nil).Classify(context.Background(), "生成牛顿运动定律的思维导图")
	if result.Intent != model.IntentMindmap {
		t.Fatalf("intent = %s", result.Intent)
	}
}

func TestClassifySearch(t *testing.T) {
	result := New(nil).Classify(context.Background(), "搜索资料库里的万有引力讲义")
	if result.Intent != model.IntentSearch {
		t.Fatalf("intent = %s", result.Intent)
	}
}

func TestExtractTopic_平抛运动(t *testing.T) {
	topic := extractTopic("平抛运动这部分题目做的不好")
	if topic == "" {
		t.Error("应从输入中提取出主题")
	}
}

func TestExtractLessonNo(t *testing.T) {
	n := extractLessonNo("秋季课第15讲 平抛运动的推论")
	if n != 15 {
		t.Fatalf("lesson no = %d", n)
	}
}
