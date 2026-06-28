package main

// import (
// 	"context"
// 	"encoding/binary"
// 	"encoding/hex"
// 	"fmt"
// 	"os"

// 	"github.com/tetratelabs/wazero"
// )

// func main() {
// 	wasmBytes, err := os.ReadFile("ds_hash_no_alloc.wasm")
// 	if err != nil {
// 		panic(err)
// 	}

// 	ctx := context.Background()
// 	r := wazero.NewRuntime(ctx)
// 	defer r.Close(ctx)

// 	compiled, err := r.CompileModule(ctx, wasmBytes)
// 	if err != nil {
// 		panic(err)
// 	}

// 	mod, err := r.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
// 	if err != nil {
// 		panic(err)
// 	}

// 	mem := mod.ExportedMemory("memory")
// 	if mem == nil {
// 		panic("no memory")
// 	}

// 	hashFunc := mod.ExportedFunction("wasm_deepseek_hash_v1")
// 	if hashFunc == nil {
// 		panic("no hash")
// 	}

// 	challenge := "b6301256d3355dd6470b2a831f97f9e1acc9fe10b04db8b709b1234c37d2499e"
// 	salt := "4155c185ce433003d3bc"
// 	targetPath := "/api/v0/chat/completion"
// 	answer := "36400"
// 	expected := "17613ad322f2ad923a10e00854bcaaf46ac69878fd307a7c75a51f3edb96b151"

// 	chBytes, _ := hex.DecodeString(challenge) // 32 bytes
// 	sBytes, _ := hex.DecodeString(salt)       // 10 bytes

// 	type combo struct {
// 		name string
// 		data []byte
// 	}

// 	// 预构造所有组合数据，避免闭包内的运行时错误导致整个程序崩溃
// 	var combos []combo

// 	// 安全的长度前缀拼接辅助函数
// 	makeLenPrefix := func(parts ...[]byte) []byte {
// 		var total int
// 		for _, p := range parts {
// 			total += len(p)
// 		}
// 		buf := make([]byte, 0, total)
// 		for _, p := range parts {
// 			buf = append(buf, p...)
// 		}
// 		return buf
// 	}

// 	// 添加纯文本组合
// 	combos = append(combos,
// 		combo{"path+ch+sa+ans", []byte(targetPath + challenge + salt + answer)},
// 		combo{"path\\nch\\nsa\\nans", []byte(targetPath + "\n" + challenge + "\n" + salt + "\n" + answer)},
// 		combo{"ch+sa+ans", []byte(challenge + salt + answer)},
// 		combo{"ch\\nsa\\nans", []byte(challenge + "\n" + salt + "\n" + answer)},
// 		combo{"ans+ch+sa+path", []byte(answer + challenge + salt + targetPath)},
// 		combo{"ans\\nch\\nsa\\npath", []byte(answer + "\n" + challenge + "\n" + salt + "\n" + targetPath)},
// 	)

// 	// hex 解码后组合
// 	combos = append(combos,
// 		combo{"path+chBytes+saBytes+ans", makeLenPrefix([]byte(targetPath), chBytes, sBytes, []byte(answer))},
// 		combo{"path\\nchBytes+saBytes+ans", makeLenPrefix([]byte(targetPath+"\n"), chBytes, sBytes, []byte(answer))},
// 		combo{"chBytes+saBytes+ans", makeLenPrefix(chBytes, sBytes, []byte(answer))},
// 		combo{"chBytes+saBytes+ans+path", makeLenPrefix(chBytes, sBytes, []byte(answer), []byte(targetPath))},
// 		combo{"path+chBytes+saBytes+ans+path", makeLenPrefix([]byte(targetPath), chBytes, sBytes, []byte(answer), []byte(targetPath))},
// 		combo{"chBytes+saBytes+ans (空格分隔)", makeLenPrefix(chBytes, []byte(" "), sBytes, []byte(" "+answer))},
// 		combo{"chBytes+saBytes+ans (逗号分隔)", makeLenPrefix(chBytes, []byte(","), sBytes, []byte(","+answer))},
// 	)

// 	// 长度前缀格式
// 	lenCh := uint32(len(chBytes))
// 	lenSa := uint32(len(sBytes))
// 	lenAns := uint32(len(answer))
// 	bufLenCh := make([]byte, 4)
// 	binary.LittleEndian.PutUint32(bufLenCh, lenCh)
// 	bufLenSa := make([]byte, 4)
// 	binary.LittleEndian.PutUint32(bufLenSa, lenSa)
// 	bufLenAns := make([]byte, 4)
// 	binary.LittleEndian.PutUint32(bufLenAns, lenAns)

// 	combos = append(combos,
// 		combo{"len(ch)+ch+len(sa)+sa+ans", makeLenPrefix(bufLenCh, chBytes, bufLenSa, sBytes, []byte(answer))},
// 		combo{"len(ch)+ch+len(sa)+sa+len(ans)+ans", makeLenPrefix(bufLenCh, chBytes, bufLenSa, sBytes, bufLenAns, []byte(answer))},
// 	)

// 	// 特殊分隔符
// 	combos = append(combos,
// 		combo{"chBytes+':'+saBytes+':'+ans", makeLenPrefix(chBytes, []byte(":"), sBytes, []byte(":"+answer))},
// 		combo{"chBytes+'|'+saBytes+'|'+ans", makeLenPrefix(chBytes, []byte("|"), sBytes, []byte("|"+answer))},
// 		combo{"chBytes+0x00+saBytes+0x00+ans", makeLenPrefix(chBytes, []byte{0}, sBytes, []byte{0}, []byte(answer))},
// 	)

// 	// 反序
// 	combos = append(combos,
// 		combo{"sa+ch+ans", []byte(salt + challenge + answer)},
// 		combo{"saBytes+chBytes+ans", makeLenPrefix(sBytes, chBytes, []byte(answer))},
// 		combo{"ans+sa+ch+path", []byte(answer + salt + challenge + targetPath)},
// 		combo{"ans+saBytes+chBytes+path", makeLenPrefix([]byte(answer), sBytes, chBytes, []byte(targetPath))},
// 	)

// 	// 仅字节
// 	combos = append(combos,
// 		combo{"chBytes+saBytes (无ans)", makeLenPrefix(chBytes, sBytes)},
// 		combo{"chBytes+saBytes+ansBytes", makeLenPrefix(chBytes, sBytes, []byte(answer))},
// 		combo{"path+chBytes+saBytes (无ans)", makeLenPrefix([]byte(targetPath), chBytes, sBytes)},
// 	)

// 	// 更多尝试
// 	combos = append(combos,
// 		combo{"path+':'+ch+':'+sa+':'+ans", []byte(targetPath + ":" + challenge + ":" + salt + ":" + answer)},
// 		combo{"path+'\\n'+ch+'\\n'+sa+'\\n'+ans+'\\n'", []byte(targetPath + "\n" + challenge + "\n" + salt + "\n" + answer + "\n")},
// 		combo{"ch+sa+ans+ch+sa", []byte(challenge + salt + answer + challenge + salt)},
// 	)

// 	dataPtr := uint32(1000)
// 	outDescPtr := uint32(3000)

// 	for _, combo := range combos {
// 		dataLen := len(combo.data)
// 		if !mem.Write(dataPtr, combo.data) {
// 			fmt.Printf("[%s] 写入数据失败\n", combo.name)
// 			continue
// 		}
// 		if !mem.Write(outDescPtr, make([]byte, 8)) {
// 			fmt.Printf("[%s] 初始化输出描述符失败\n", combo.name)
// 			continue
// 		}

// 		_, err := hashFunc.Call(ctx, uint64(outDescPtr), uint64(dataPtr), uint64(dataLen))
// 		if err != nil {
// 			fmt.Printf("[%s] 调用失败: %v\n", combo.name, err)
// 			continue
// 		}

// 		descBytes, _ := mem.Read(outDescPtr, 8)
// 		strPtr := binary.LittleEndian.Uint32(descBytes[0:4])
// 		strLen := binary.LittleEndian.Uint32(descBytes[4:8])

// 		if strPtr == 0 || strLen == 0 {
// 			fmt.Printf("[%s] 输出为空\n", combo.name)
// 			continue
// 		}

// 		resultBytes, _ := mem.Read(strPtr, strLen)
// 		resultStr := string(resultBytes)

// 		if resultStr == expected {
// 			fmt.Printf("✅ [%s] 匹配成功！\n", combo.name)
// 			fmt.Printf("   输入长度: %d\n", dataLen)
// 			fmt.Printf("   签名: %s\n", resultStr)
// 			return
// 		} else {
// 			fmt.Printf("[%s] 哈希: %s... (不匹配)\n", combo.name, resultStr[:16])
// 		}
// 	}

// 	fmt.Println("❌ 所有组合均不匹配，需要更多尝试")
// }
