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
