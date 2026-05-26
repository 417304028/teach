package generate

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestBuildDOCX(t *testing.T) {
	data, err := BuildDOCX(Doc{Title: "动能定理", Sections: []DocSection{{Heading: "练习", Lines: []string{"第一题"}}}})
	if err != nil {
		t.Fatal(err)
	}
	assertZipContains(t, data, "word/document.xml")
}

func TestBuildPPTX(t *testing.T) {
	data, err := BuildPPTX([]Slide{{Title: "动能定理", Lines: []string{"目标", "例题"}}})
	if err != nil {
		t.Fatal(err)
	}
	assertZipContains(t, data, "ppt/slides/slide1.xml")
}

func assertZipContains(t *testing.T, data []byte, name string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name == name {
			return
		}
	}
	t.Fatalf("zip did not contain %s", name)
}
