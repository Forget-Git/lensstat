package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// makeJPEG 生成一个最小 JPEG（无 EXIF）。
func makeJPEG(t *testing.T, path string) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 25), uint8(y * 25), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRunFlow(t *testing.T) {
	dir := t.TempDir()
	makeJPEG(t, filepath.Join(dir, "a.jpg"))
	makeJPEG(t, filepath.Join(dir, "b.JPEG"))
	// 非 jpg 文件，不应被统计
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := collectJpgFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("期望 2 个 jpg，实际 %d", len(files))
	}

	photos := []PhotoInfo{readPhoto(files[0])}
	if photos[0].Err == "" {
		t.Fatal("无 EXIF 文件应在备注列记录错误")
	}
	if photos[0].FileName != "a.jpg" {
		t.Fatalf("文件名错误: %s", photos[0].FileName)
	}

	// 焦段统计：无焦距应返回空
	if got := statFocalLength(photos); len(got) != 0 {
		t.Fatalf("无焦距不应统计，实际 %d", len(got))
	}

	out := filepath.Join(dir, "test.xlsx")
	saved := outputXlsx
	outputXlsx = out
	defer func() { outputXlsx = saved }()

	if err := writeExcel(photos); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("Excel 未生成: %v", err)
	}
}
