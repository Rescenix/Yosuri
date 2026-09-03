package handler

import (
	"archive/zip"
	"bytes"
	"testing"
)

// TestGenPptxDesign 验证新设计版 PPT：zip 结构合法，slide1.xml 含亮蓝白主题（背景/标题条/装饰条/
// 圆角卡片/页脚）与页码。
func TestGenPptxDesign(t *testing.T) {
	data, err := genPptx("路演", []officeSlide{{Title: "产品", Bullets: []string{"要点一", "要点二", "要点三"}}})
	if err != nil {
		t.Fatalf("genPptx 失败: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("无法解压 zip: %v", err)
	}
	var slide []byte
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			sb := &bytes.Buffer{}
			_, _ = sb.ReadFrom(rc)
			rc.Close()
			slide = sb.Bytes()
		}
	}
	if len(slide) == 0 {
		t.Fatalf("未找到 slide1.xml")
	}
	for _, want := range []string{"FFFFFF", "1950BE", "•", "footer", "第 1 页", "要点一", "要点二"} {
		if !bytes.Contains(slide, []byte(want)) {
			t.Fatalf("slide 缺设计元素 %q", want)
		}
	}
	t.Logf("slide1.xml 校验通过：亮蓝白主题 + 卡片 + 页脚 + 页码齐")
}

// TestGenPptxLongTitle 长标题（带时间戳项目名）自动降字号 + 条内换行，不溢出截断。
func TestGenPptxLongTitle(t *testing.T) {
	longTitle := "001-做一个学生专注冲刺台：番茄钟功能,要能运行,含计时器与待办分组2-0903-203827"
	data, err := genPptx(longTitle, []officeSlide{{Title: longTitle, Bullets: []string{"要点"}}})
	if err != nil {
		t.Fatalf("genPptx 失败: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	var slide []byte
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			sb := &bytes.Buffer{}
			_, _ = sb.ReadFrom(rc)
			rc.Close()
			slide = sb.Bytes()
		}
	}
	// 长标题应降字号（sz=2000），且含 wrap="square" 换行 + anchor="ctr" 垂直居中
	if !bytes.Contains(slide, []byte(`sz="2000"`)) {
		t.Fatalf("长标题未降字号（应 sz=2000）")
	}
	if !bytes.Contains(slide, []byte(`wrap="square"`)) || !bytes.Contains(slide, []byte(`anchor="ctr"`)) {
		t.Fatalf("标题未启用条内换行/垂直居中")
	}
	t.Logf("长标题降字号 + 换行居中校验通过")
}
