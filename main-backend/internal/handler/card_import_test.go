package handler

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

// makeTextPNG 造一张带 tEXt 块的最小 PNG（解析器不校验 CRC，只要块结构合法）。
func makeTextPNG(keyword, text string) []byte {
	var buf bytes.Buffer
	buf.Write(pngSignature)
	chunk := func(typ string, body []byte) {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(body)))
		buf.WriteString(typ)
		buf.Write(body)
		buf.Write([]byte{0, 0, 0, 0}) // CRC 占位
	}
	// IHDR：1x1 RGB
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 1)
	binary.BigEndian.PutUint32(ihdr[4:], 1)
	ihdr[8] = 8
	ihdr[9] = 2
	chunk("IHDR", ihdr)
	chunk("tEXt", []byte(keyword+"\x00"+text))
	chunk("IEND", nil)
	return buf.Bytes()
}

const v2JSON = `{
  "spec": "chara_card_v2",
  "spec_version": "2.0",
  "data": {
    "name": "小雪",
    "description": "17 岁的图书管理员。",
    "personality": "安静，偶尔嘴硬。",
    "scenario": "深夜图书馆。",
    "first_mes": "……你还没走啊。",
    "mes_example": "<START>\n{{user}}: 还不下班？\n{{char}}: 书还没放回去。",
    "alternate_greetings": ["又来借书？", "灯要关了哦。"],
    "tags": ["原创"],
    "creator": "tester",
    "character_book": {
      "entries": [
        {"keys": ["图书馆", "开门"], "content": "图书馆早八点到晚十点。", "enabled": true, "insertion_order": 3},
        {"keys": ["小雪"], "content": "小雪住在图书馆后面的旧楼。", "enabled": false, "constant": true}
      ]
    }
  }
}`

func TestParseV2JSON(t *testing.T) {
	c, err := parseCardJSON([]byte(v2JSON))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if c.Spec != "v2" {
		t.Fatalf("spec 应为 v2，实际 %q", c.Spec)
	}
	if c.Name != "小雪" {
		t.Fatalf("name 错误: %q", c.Name)
	}
	for _, want := range []string{"【人设】", "17 岁的图书管理员", "【性格】", "安静，偶尔嘴硬", "【场景】", "深夜图书馆", "【示例对话】"} {
		if !strings.Contains(c.Persona, want) {
			t.Fatalf("persona 缺少 %q:\n%s", want, c.Persona)
		}
	}
	if c.Greeting != "……你还没走啊。" {
		t.Fatalf("first_mes 没映射: %q", c.Greeting)
	}
	if len(c.AltGreetings) != 2 {
		t.Fatalf("alternate_greetings 应 2 条，实际 %d", len(c.AltGreetings))
	}
	if len(c.WorldBook) != 2 {
		t.Fatalf("世界书应 2 条，实际 %d", len(c.WorldBook))
	}
	if c.WorldBook[0].Order != 3 || !c.WorldBook[0].Enabled {
		t.Fatalf("条目 0 的 order/enabled 映射错误: %+v", c.WorldBook[0])
	}
	if c.WorldBook[1].Enabled {
		t.Fatalf("条目 1 enabled=false 没映射")
	}
	if !c.WorldBook[1].Constant {
		t.Fatalf("条目 1 constant 没映射")
	}
	if c.WorldBook[0].Position != "before" {
		t.Fatalf("默认 position 应为 before")
	}
}

func TestParseV1FlatJSON(t *testing.T) {
	v1 := `{"name":"老张","description":"门卫。","personality":"话少。","scenario":"传达室。","first_mes":"登记一下。","mes_example":""}`
	c, err := parseCardJSON([]byte(v1))
	if err != nil {
		t.Fatalf("V1 解析失败: %v", err)
	}
	if c.Spec != "v1" || c.Name != "老张" || c.Greeting != "登记一下。" {
		t.Fatalf("V1 字段错误: %+v", c)
	}
	if strings.Contains(c.Persona, "【示例对话】") {
		t.Fatalf("空 mes_example 不该产生段标题")
	}
}

func TestParseV3FromPNGChunk(t *testing.T) {
	v3 := `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"阿岚","description":"剑客。","first_mes":"拔剑吧。","nickname":"岚","group_only_greetings":[]}}`
	png := makeTextPNG("ccv3", base64.StdEncoding.EncodeToString([]byte(v3)))
	c, err := parseCharacterCard(png)
	if err != nil {
		t.Fatalf("PNG V3 解析失败: %v", err)
	}
	if c.Spec != "v3" || c.Name != "阿岚" || c.Greeting != "拔剑吧。" {
		t.Fatalf("V3 字段错误: %+v", c)
	}
}

// ccv3 与 chara 同时存在时，必须用 ccv3。
func TestPNGPrefersCCV3Chunk(t *testing.T) {
	v2 := `{"spec":"chara_card_v2","data":{"name":"旧版","description":"x"}}`
	v3 := `{"spec":"chara_card_v3","data":{"name":"新版","description":"y"}}`
	var buf bytes.Buffer
	buf.Write(pngSignature)
	chunk := func(typ string, body []byte) {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(body)))
		buf.WriteString(typ)
		buf.Write(body)
		buf.Write([]byte{0, 0, 0, 0})
	}
	chunk("tEXt", []byte("chara\x00"+base64.StdEncoding.EncodeToString([]byte(v2))))
	chunk("tEXt", []byte("ccv3\x00"+base64.StdEncoding.EncodeToString([]byte(v3))))
	chunk("IEND", nil)

	c, err := parseCharacterCard(buf.Bytes())
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if c.Name != "新版" || c.Spec != "v3" {
		t.Fatalf("应优先 ccv3，实际 %s/%s", c.Spec, c.Name)
	}
}

// 转发丢了 tEXt 的 PNG：必须给出「去下原文件」的可操作提示，而不是笼统报错。
func TestPNGWithoutChunkGivesActionableError(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(pngSignature)
	_ = binary.Write(&buf, binary.BigEndian, uint32(13))
	buf.WriteString("IHDR")
	buf.Write(make([]byte, 13))
	buf.Write([]byte{0, 0, 0, 0})

	_, err := parseCharacterCard(buf.Bytes())
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "微信") {
		t.Fatalf("错误提示应指向转发丢数据，实际: %v", err)
	}
}

func TestParseCharx(t *testing.T) {
	card := `{"spec":"chara_card_v3","data":{"name":"打包卡","description":"来自 charx"}}`
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, err := zw.Create("card.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(card)); err != nil {
		t.Fatal(err)
	}
	zw.Close()

	c, err := parseCharacterCard(zbuf.Bytes())
	if err != nil {
		t.Fatalf("CHARX 解析失败: %v", err)
	}
	if c.Name != "打包卡" {
		t.Fatalf("name 错误: %q", c.Name)
	}
}

// 世界书 entries 用 map 形式导出的卡也要能吃下。
func TestLorebookObjectEntries(t *testing.T) {
	card := `{"spec":"chara_card_v2","data":{"name":"m","description":"d","character_book":{"entries":{"0":{"keys":["a"],"content":"A 的设定","enabled":true},"1":{"keys":["b"],"content":"B 的设定","enabled":true}}}}}`
	c, err := parseCardJSON([]byte(card))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(c.WorldBook) != 2 {
		t.Fatalf("map 形式应解析出 2 条，实际 %d", len(c.WorldBook))
	}
}

func TestNotACard(t *testing.T) {
	if _, err := parseCardJSON([]byte(`{"hello":"world"}`)); err == nil {
		t.Fatal("随便一个 JSON 不该被当角色卡")
	}
	if _, err := parseCardJSON([]byte("not json at all")); err == nil {
		t.Fatal("非 JSON 应报错")
	}
}

func TestDataURLPrefixAccepted(t *testing.T) {
	v2 := v2JSON
	png := makeTextPNG("chara", base64.StdEncoding.EncodeToString([]byte(v2)))
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	b64 := dataURL[strings.Index(dataURL, ",")+1:]
	raw, err := decodeCardBase64(b64)
	if err != nil || !bytes.HasPrefix(raw, pngSignature) {
		t.Fatalf("dataURL 解出的不是 PNG")
	}
	if _, err := parseCharacterCard(raw); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// JSON 兜底校验：确保测试用的 v2JSON 本身合法
	var probe map[string]any
	if err := json.Unmarshal([]byte(v2), &probe); err != nil {
		t.Fatalf("测试素材 JSON 非法: %v", err)
	}
}
