package main

// import (
// 	"context"
// 	"encoding/binary"
// 	"encoding/hex"
// 	"fmt"
// 	"math/big"
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

// 	// 用已知合法的 answer=36400 对应的 challenge/salt 来验证难度条件
// 	challenge := "b6301256d3355dd6470b2a831f97f9e1acc9fe10b04db8b709b1234c37d2499e"
// 	salt := "4155c185ce433003d3bc"
// 	answer := 36400
// 	knownSignature := "17613ad322f2ad923a10e00854bcaaf46ac69878fd307a7c75a51f3edb96b151"

// 	// 尝试不同的拼接格式，找到正确的那个
// 	formats := []string{"targetPath_ch_sa_ans", "ch_sa_ans", "ch_sa_ans_nohex", "sa_ch_ans"}
// 	for _, fmtName := range formats {
// 		var input []byte
// 		switch fmtName {
// 		case "targetPath_ch_sa_ans":
// 			input = []byte("/api/v0/chat/completion" + challenge + salt + fmt.Sprintf("%d", answer))
// 		case "ch_sa_ans":
// 			input = []byte(challenge + salt + fmt.Sprintf("%d", answer))
// 		case "ch_sa_ans_nohex":
// 			chBytes, _ := hex.DecodeString(challenge)
// 			saBytes, _ := hex.DecodeString(salt)
// 			input = append(append(chBytes, saBytes...), []byte(fmt.Sprintf("%d", answer))...)
// 		case "sa_ch_ans":
// 			input = []byte(salt + challenge + fmt.Sprintf("%d", answer))
// 		}

// 		dataLen := len(input)
// 		dataPtr := uint32(1000)
// 		outDescPtr := uint32(3000)

// 		if !mem.Write(dataPtr, input) {
// 			continue
// 		}
// 		if !mem.Write(outDescPtr, make([]byte, 8)) {
// 			continue
// 		}

// 		_, err := hashFunc.Call(ctx, uint64(outDescPtr), uint64(dataPtr), uint64(dataLen))
// 		if err != nil {
// 			continue
// 		}

// 		descBytes, _ := mem.Read(outDescPtr, 8)
// 		strPtr := binary.LittleEndian.Uint32(descBytes[0:4])
// 		strLen := binary.LittleEndian.Uint32(descBytes[4:8])
// 		if strPtr == 0 || strLen == 0 {
// 			continue
// 		}

// 		hexStr, _ := mem.Read(strPtr, strLen)
// 		hashBytes, _ := hex.DecodeString(string(hexStr))
// 		if string(hexStr) == knownSignature {
// 			fmt.Printf("✅ 找到正确格式: %s\n", fmtName)
// 			// 验证难度条件：哈希值 < 144000 ？
// 			hashInt := new(big.Int).SetBytes(hashBytes)
// 			fmt.Printf("哈希值 = %s\n", hashInt.String())
// 			if hashInt.Cmp(big.NewInt(144000)) < 0 {
// 				fmt.Println("✅ 难度条件确认：哈希值 < 144000")
// 			} else {
// 				fmt.Println("❌ 哈希值 >= 144000，难度条件可能不是简单的数值比较")
// 				fmt.Println("可能需要哈希前几个字节为0")
// 			}
// 			return
// 		}
// 	}
// 	fmt.Println("❌ 所有格式均不匹配已知签名，需要调整格式")
// }
