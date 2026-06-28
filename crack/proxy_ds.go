package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"

	"github.com/tetratelabs/wazero"
)

// 从浏览器复制的最新认证信息
var dsHeaders = map[string]string{
	"authorization":            "Bearer O3gI2bnaFVQcJ25N8DJ1WNR56qUDo6oozeGVA0oHQJlfzd6ZdnHg/luC8Wa+UM78",
	"cookie":                   "HWWAFSESTIME=1782645556287; HWWAFSESID=1c33e12b8b0462797b06; ds_session_id=1154be51b1924f19ad34ee12d0dc8d1c; smidV2=20260628191919c45d95ced5beb22d1d1cedfe642bc6f100d46df4604025920; .thumbcache_6b2e5483f9d858d7c661c5e276b6a6ae=jMJlxnW+csMmSzDotZa0yk3Sdyphe2PrZXKDmwOC01tVP4WsEBdK2S3Ot8BlaG6kFj1mIWeU5kG1e9YkN2/ySg==",
	"user-agent":               "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
	"origin":                   "https://chat.deepseek.com",
	"referer":                  "https://chat.deepseek.com/",
	"x-app-version":            "2.0.0",
	"x-client-locale":          "zh_CN",
	"x-client-platform":        "web",
	"x-client-version":         "2.0.0",
	"x-client-timezone-offset": "28800",
}

type PowChallenge struct {
	Algorithm  string `json:"algorithm"`
	Challenge  string `json:"challenge"`
	Salt       string `json:"salt"`
	Signature  string `json:"signature"`
	TargetPath string `json:"target_path"`
}

type PoWResponse struct {
	Algorithm  string `json:"algorithm"`
	Challenge  string `json:"challenge"`
	Salt       string `json:"salt"`
	Signature  string `json:"signature"`
	Answer     int    `json:"answer"`
	TargetPath string `json:"target_path"`
}

var (
	wasmRuntime wazero.Runtime
	wasmCtx     context.Context
)

func loadWasm() (func([]byte) ([]byte, error), error) {
	wasmBytes, err := os.ReadFile("ds_hash_no_alloc.wasm")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)

	compiled, err := r.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, err
	}

	mod, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, err
	}

	mem := mod.ExportedMemory("memory")
	if mem == nil {
		return nil, fmt.Errorf("no memory")
	}

	hf := mod.ExportedFunction("wasm_deepseek_hash_v1")
	if hf == nil {
		return nil, fmt.Errorf("no hash function")
	}

	wasmRuntime = r
	wasmCtx = ctx

	return func(input []byte) ([]byte, error) {
		dataLen := len(input)
		dataPtr := uint32(1000)
		outDescPtr := uint32(3000)

		if !mem.Write(dataPtr, input) {
			return nil, fmt.Errorf("write input failed")
		}
		if !mem.Write(outDescPtr, make([]byte, 8)) {
			return nil, fmt.Errorf("write outdesc failed")
		}

		_, err := hf.Call(ctx, uint64(outDescPtr), uint64(dataPtr), uint64(dataLen))
		if err != nil {
			return nil, err
		}

		descBytes, ok := mem.Read(outDescPtr, 8)
		if !ok {
			return nil, fmt.Errorf("read desc failed")
		}
		strPtr := binary.LittleEndian.Uint32(descBytes[0:4])
		strLen := binary.LittleEndian.Uint32(descBytes[4:8])
		if strPtr == 0 || strLen == 0 {
			return nil, fmt.Errorf("empty output")
		}

		hexStr, ok := mem.Read(strPtr, strLen)
		if !ok {
			return nil, fmt.Errorf("read hex string failed")
		}

		rawHash, err := hex.DecodeString(string(hexStr))
		if err != nil {
			return nil, fmt.Errorf("decode hex output: %v", err)
		}
		return rawHash, nil
	}, nil
}

// 根据WAT源码的逆向分析，构造传给哈希函数的二进制输入
func buildHashInput(targetPath string, challenge, salt []byte, answer int) []byte {
	// 固定头部336字节 + target_path + answer字符串
	answerStr := fmt.Sprintf("%d", answer)

	// 336字节的固定结构
	fixedHeader := make([]byte, 336)

	// 偏移0x00: target_path长度 (4字节)
	binary.LittleEndian.PutUint32(fixedHeader[0:4], uint32(len(targetPath)))

	// 偏移0x08: challenge长度 (32字节 hex)
	copy(fixedHeader[8:40], challenge)

	// 偏移0x28: salt长度 (10字节 hex)
	copy(fixedHeader[40:50], salt)

	// 偏移0x30: difficulty (8字节，来自全局常量)
	// 144000 = 0x00023280
	binary.LittleEndian.PutUint64(fixedHeader[0x30:0x38], 144000)

	// 拼接: fixedHeader + target_path + answerStr
	buf := make([]byte, 0, 336+len(targetPath)+len(answerStr))
	buf = append(buf, fixedHeader...)
	buf = append(buf, []byte(targetPath)...)
	buf = append(buf, []byte(answerStr)...)
	return buf
}

func setHeaders(req *http.Request) {
	for k, v := range dsHeaders {
		if k == "cookie" {
			req.Header.Set("Cookie", v)
		} else {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "*/*")
}

func FetchPoW() (*PowChallenge, error) {
	body := []byte(`{"target_path":"/api/v0/chat/completion"}`)
	req, _ := http.NewRequest("POST", "https://chat.deepseek.com/api/v0/chat/create_pow_challenge", bytes.NewReader(body))
	setHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求挑战失败: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	fmt.Println("PoW 挑战响应:", string(data))

	var result struct {
		Data struct {
			BizData struct {
				Challenge PowChallenge `json:"challenge"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析挑战响应失败: %w", err)
	}
	return &result.Data.BizData.Challenge, nil
}

func buildPoWJSON(challenge PowChallenge, answer int) string {
	powResp := PoWResponse{
		Algorithm:  challenge.Algorithm,
		Challenge:  challenge.Challenge,
		Salt:       challenge.Salt,
		Signature:  challenge.Signature,
		Answer:     answer,
		TargetPath: challenge.TargetPath,
	}
	jsonBytes, _ := json.Marshal(powResp)
	return string(jsonBytes)
}

func solveAnswer(challenge PowChallenge, hashFunc func([]byte) ([]byte, error)) (int, error) {
	chBytes, _ := hex.DecodeString(challenge.Challenge)
	saBytes, _ := hex.DecodeString(challenge.Salt)
	targetPath := challenge.TargetPath

	for answer := 0; answer < 1000000; answer++ {
		if answer%10000 == 0 {
			fmt.Printf("  尝试 answer=%d...\n", answer)
		}
		input := buildHashInput(targetPath, chBytes, saBytes, answer)
		hash, err := hashFunc(input)
		if err != nil {
			continue
		}
		// 难度条件：哈希值 < 144000
		hashInt := new(big.Int).SetBytes(hash)
		if hashInt.Cmp(big.NewInt(144000)) < 0 {
			fmt.Printf("找到 answer=%d, hashInt=%s\n", answer, hashInt.String())
			return answer, nil
		}
	}
	return 0, fmt.Errorf("未找到 answer")
}

func ChatCompletion(challenge PowChallenge, answer int, prompt string) (*http.Response, error) {
	powJSONStr := buildPoWJSON(challenge, answer)
	powBase64 := base64.StdEncoding.EncodeToString([]byte(powJSONStr))

	reqBody := map[string]interface{}{
		"chat_session_id":   "24c7ffea-159a-46e7-be16-e5ca6d045c81",
		"parent_message_id": nil,
		"model_type":        nil,
		"prompt":            prompt,
		"ref_file_ids":      []interface{}{},
		"thinking_enabled":  true,
		"search_enabled":    false,
		"action":            nil,
		"preempt":           false,
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://chat.deepseek.com/api/v0/chat/completion", bytes.NewReader(body))
	setHeaders(req)
	req.Header.Set("x-ds-pow-response", powBase64)

	return http.DefaultClient.Do(req)
}

func main() {
	hashFunc, err := loadWasm()
	if err != nil {
		fmt.Println("加载WASM失败:", err)
		return
	}
	defer wasmRuntime.Close(wasmCtx)

	powChallenge, err := FetchPoW()
	if err != nil {
		fmt.Println("获取PoW失败:", err)
		return
	}

	answer, err := solveAnswer(*powChallenge, hashFunc)
	if err != nil {
		fmt.Println("求解answer失败:", err)
		return
	}

	resp, err := ChatCompletion(*powChallenge, answer, "你好，请用一句简短的话回答我")
	if err != nil {
		fmt.Println("聊天请求失败:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("聊天响应状态:", resp.Status)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println("错误内容:", string(body))
		return
	}

	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	fmt.Printf("回复内容:\n%s\n", string(buf[:n]))
	io.Copy(io.Discard, resp.Body)
}
