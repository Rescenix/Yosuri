# Prism — 轻量级向量记忆存储库

## 概述

**Prism** 是一个用 C++17 编写的轻量级向量记忆存储引擎。它将文本记忆（包含角色、内容、关键词）与浮点向量（embedding）一起持久化到二进制文件中，并提供基于余弦相似度的暴力检索能力。

Prism 以动态链接库（DLL）形式提供纯 C API，可被任何支持 C ABI 的语言（C/C++、Python、Go 等）调用。

## 特性

- **纯 C API**：导出 `extern "C"` 函数，跨语言调用友好
- **二进制持久化**：每条记录以紧凑的二进制格式追加写入文件，重启后自动恢复
- **向量检索**：基于点积的暴力搜索（假设向量已归一化），返回余弦相似度最高的 top-k 结果
- **增量写入**：插入即落盘（`fflush`），数据不丢失
- **内存缓存**：全量记录缓存在内存中，搜索零磁盘 I/O
- **轻量无依赖**：仅依赖 C/C++ 标准库，无需第三方库

## 构建

### 环境要求

- CMake ≥ 3.12
- 支持 C++17 的编译器（MSVC、GCC、Clang）

### 构建步骤

```bash
cd Prism/prism
mkdir build && cd build
cmake ..
cmake --build .
```

构建产物：

| 文件 | 说明 |
|------|------|
| `prism_store.dll` / `libprism_store.so` | 共享库 |
| `test_prism.exe` | 测试程序 |

## API 参考

所有函数在 `prism_store.h` 中声明。

### `prism_open`

```c
PrismStore* prism_open(const char* data_path);
```

打开（或创建）一个持久化存储文件。返回不透明指针，后续操作均通过该指针进行。

### `prism_close`

```c
void prism_close(PrismStore* store);
```

关闭存储，释放所有内存。

### `prism_insert`

```c
uint64_t prism_insert(
    PrismStore* store,
    const char* role,       // 角色（如 "user" / "assistant"）
    const char* content,    // 记忆内容
    const char* keywords,   // 关键词（可选，可传空字符串）
    const float* embedding, // 浮点向量数组
    int embedding_dim       // 向量维度
);
```

插入一条记忆记录，返回自增的唯一 ID。

### `prism_search`

```c
int prism_search(
    PrismStore* store,
    const float* query_vec,  // 查询向量
    int dim,                 // 向量维度
    int top_k,               // 返回前 k 个结果
    PrismResult* results,    // 由调用方预分配的缓冲区（至少 top_k 个）
    float min_score          // 最低相似度阈值，0.0 表示不限制
);
```

暴力搜索与查询向量最相似的 top-k 条记忆。返回实际命中数，结果按相似度降序排列。

**注意**：搜索基于点积运算，假设所有 embedding 向量已归一化（`||v|| = 1`），此时点积 = 余弦相似度。

### `prism_count`

```c
int prism_count(PrismStore* store);
```

返回存储中总的记忆条数。

### `prism_free_results`

```c
void prism_free_results(PrismResult* results, int count);
```

释放 `prism_search` 返回结果中的动态分配字符串。

## 二进制存储格式

每条记录在文件中的布局如下（小端序）：

| 偏移 | 类型 | 字段 |
|------|------|------|
| 0 | `uint64_t` | id |
| 8 | `uint32_t` | role 长度 (N) |
| 12 | `char[N]` | role 数据 |
| 12+N | `uint32_t` | content 长度 (M) |
| 16+N | `char[M]` | content 数据 |
| 16+N+M | `uint32_t` | keywords 长度 (K) |
| 20+N+M | `char[K]` | keywords 数据 |
| 20+N+M+K | `uint32_t` | 向量维度 (D) |
| 24+N+M+K | `float[D]` | 向量数据 |

文件以追加方式写入，每次插入只写入新记录，不做碎片整理。

## 使用示例

参见 `main.cpp`：

```cpp
PrismStore* store = prism_open("memory.dat");

float emb[] = {1.0f, 0.0f};
uint64_t id = prism_insert(store, "user", "hello", "", emb, 2);

float query[] = {0.6f, 0.8f};
PrismResult results[5];
int n = prism_search(store, query, 2, 5, results, 0.0f);

prism_free_results(results, n);
prism_close(store);
```

## 项目结构

```
Prism/
└── prism/
    ├── CMakeLists.txt      # CMake 构建配置
    ├── prism_store.h       # 公共头文件（C API 声明）
    ├── prism_store.cpp     # 核心实现
    ├── main.cpp            # 测试/演示程序
    ├── prism_store.dll     # 预编译共享库
    ├── test_prism.exe      # 预编译测试程序
    ├── test_memory.dat     # 测试生成的数据文件
    └── build/              # CMake 构建输出目录
```

## 已知局限

- **暴力搜索**：未使用近似最近邻（ANN）索引，大规模数据（>10 万条）时延迟较高
- **全量内存加载**：启动时将所有记录读入内存，不适合超大规模持久化场景
- **无并发控制**：未做线程安全处理，多线程同时操作需自行加锁
- **无删除/更新**：当前仅支持追加插入，不支持删除或修改已有记录
- **向量需归一化**：搜索直接使用点积，不会自动对查询向量做归一化，需调用方保证

## 许可证

本项目为内部工具，未指定开源许可证。