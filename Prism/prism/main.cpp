#include "prism_store.h"
#include <cstdio>
#include <cstring>
#include <vector>
#include <string>
#include <chrono>
#include <map>
#include <iostream>
#include <sstream>

static int display_width(const std::string& s) {
    int w = 0;
    for (size_t i = 0; i < s.size(); ) {
        unsigned char c = s[i];
        if (c < 0x80) { w++; i++; }
        else if (c < 0xE0) { w += 2; i += 2; }
        else if (c < 0xF0) { w += 2; i += 3; }
        else { w += 2; i += 4; }
    }
    return w;
}

static std::string pad_content(const std::string& s, int target_width) {
    int dw = display_width(s);
    if (dw >= target_width) return s;
    return s + std::string(target_width - dw, ' ');
}

int main() {
    printf("=== Prism 混沌语义引擎演示 ===\n");
    printf("加载记忆库...\n");
    fflush(stdout);

    // 读记忆文本元数据
    std::vector<std::pair<std::string, std::string>> mem_texts;
    FILE* meta = fopen("memories_meta.txt", "r");
    if (meta) {
        char line[512];
        while (fgets(line, sizeof(line), meta)) {
            line[strcspn(line, "\n")] = 0;
            std::string s(line);
            size_t sep = s.find('|');
            if (sep != std::string::npos) {
                mem_texts.push_back({s.substr(0, sep), s.substr(sep+1)});
            }
        }
        fclose(meta);
    }

    // 打开存储
    remove("chaos_memory.dat");
    PrismStore* store = prism_open("chaos_memory.dat");
    if (!store) {
        printf("存储引擎启动失败\n");
        return -1;
    }

    // 读取记忆向量并插入
    FILE* vf = fopen("memories.bin", "rb");
    if (!vf) { printf("缺少 memories.bin\n"); return -1; }
    float vec[1024];
    const int dim = 1024;
    int total = 0;
    while (fread(vec, sizeof(float), dim, vf) == dim && total < (int)mem_texts.size()) {
        prism_insert(store, mem_texts[total].first.c_str(),
                     mem_texts[total].second.c_str(), "", vec, dim);
        total++;
    }
    fclose(vf);
    printf("已加载 %d 条记忆。\n", total);

    // 预加载查询向量
    std::map<std::string, std::vector<float>> query_vecs;
    const char* query_names[] = {"机器学习","天气","忆阻器","混沌","编程语言","你好"};
    for (auto& name : query_names) {
        char fname[256];
        snprintf(fname, sizeof(fname), "query_%s.bin", name);
        FILE* qf = fopen(fname, "rb");
        if (qf) {
            std::vector<float> q(dim);
            fread(q.data(), sizeof(float), dim, qf);
            fclose(qf);
            query_vecs[name] = q;
        }
    }

    printf("\n可用查询：");
    for (auto& kv : query_vecs) printf("%s ", kv.first.c_str());
    printf("\n输入查询词（或 :q 退出）:\n");

    std::string line;
    while (true) {
        printf("\nprism> ");
        fflush(stdout);
        std::getline(std::cin, line);
        if (line == ":q" || line == ":quit") break;
        if (line.empty()) continue;

        // 混沌演示模式
        if (line == ":chaos") {
            if (query_vecs.empty()) { printf("无预加载查询\n"); continue; }
            auto& test_q = query_vecs.begin()->second;
            printf("混沌测试：对 '%s' 连续查询 5 次...\n", query_vecs.begin()->first.c_str());
            for (int run = 0; run < 5; ++run) {
                PrismResult results[10];
                int n = prism_search(store, test_q.data(), dim, 5, results, 0.0f);
                printf("Run %d: Top -> %s (%.4f)\n", run, results[0].content, results[0].score);
                prism_free_results(results, n);
            }
            continue;
        }

        // 查找查询向量
        auto it = query_vecs.find(line);
        if (it == query_vecs.end()) {
            printf("无此查询，可用: ");
            for (auto& kv : query_vecs) printf("%s ", kv.first.c_str());
            printf("\n");
            continue;
        }
        std::vector<float>& qvec = it->second;

        // 获取检索前状态
        std::vector<MemristorInfo> before(total);
        prism_get_all_states(store, before.data(), total);

        // 性能测量（关闭演化，纯检索）
        prism_set_evolution(store, 0);
        const int LOOPS = 10000;
        volatile int dummy = 0;
        auto t1 = std::chrono::high_resolution_clock::now();
        for (int i = 0; i < LOOPS; ++i) {
            PrismResult tmp[10];
            int n = prism_search(store, qvec.data(), dim, 10, tmp, 0.0f);
            dummy += n;
            prism_free_results(tmp, n);
        }
        auto t2 = std::chrono::high_resolution_clock::now();
        prism_set_evolution(store, 1);
        auto total_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(t2 - t1).count();
        double avg_ns = (double)total_ns / LOOPS;

        // 正式检索（触发混沌演化）
        PrismResult results[10];
        int n = prism_search(store, qvec.data(), dim, 3, results, 0.0f);

        // 获取检索后状态
        std::vector<MemristorInfo> after(total);
        prism_get_all_states(store, after.data(), total);

        // 打印结果
        printf("\n查询: %s  (Top 3)\n", line.c_str());
        printf("----------------------------------------------\n");
        for (int i = 0; i < n; ++i) {
            printf("[%llu] %s (score=%.4f)\n", results[i].id, results[i].content, results[i].score);
        }
        printf("----------------------------------------------\n");
        printf("平均延迟 (10000次): %.1f ns\n", avg_ns);

        // 打印状态变化表
        printf("\n混沌状态变化:\n");
        printf("+----+-----------------------------------+------------------+------------------+------------------+\n");
        printf("| ID | CONTENT                           | CONDUCT          | FLUX             | CHAOS            |\n");
        printf("+----+-----------------------------------+------------------+------------------+------------------+\n");
        for (int i = 0; i < total; ++i) {
            uint64_t id = before[i].id;
            float bc = before[i].conductance, bf = before[i].correlation_flux, bh = before[i].chaos;
            float ac = after[i].conductance,  af = after[i].correlation_flux, ah = after[i].chaos;

            std::string content = "mem#" + std::to_string(id);
            for (int j = 0; j < n; ++j) {
                if (results[j].id == id) {
                    content = results[j].content ? results[j].content : content;
                    break;
                }
            }

            char cond_str[32], flux_str[32], chaos_str[32];
            snprintf(cond_str, sizeof(cond_str), "%.3f -> %.3f", bc, ac);
            snprintf(flux_str, sizeof(flux_str), "%.1f -> %.1f", bf, af);
            snprintf(chaos_str, sizeof(chaos_str), "%.3f -> %.3f", bh, ah);

            printf("| %-2llu | %-33s | %-16s | %-16s | %-16s |\n",
                   id, pad_content(content, 33).c_str(), cond_str, flux_str, chaos_str);
        }
        printf("+----+-----------------------------------+------------------+------------------+------------------+\n");

        prism_free_results(results, n);
    }

    prism_close(store);
    printf("演示结束。\n");
    return 0;
}