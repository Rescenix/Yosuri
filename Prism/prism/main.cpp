#include "prism_store.h"
#include <iostream>
#include <string>
#include <vector>
#include <chrono>
#include <iomanip>

int display_width(const std::string& s) {
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

std::string pad_content(const std::string& s, int target_width) {
    int dw = display_width(s);
    if (dw >= target_width) return s;
    return s + std::string(target_width - dw, ' ');
}

int main() {
    printf("Initializing Chrono-Memristor...\n");

    remove("memristor.dat");
    PrismStore* store = prism_open("memristor.dat");
    if (!store) { printf("ERROR: Cannot open database.\n"); system("pause"); return 1; }

    // 插入中文示例记忆，向量由 TF‑IDF 自动生成
    prism_insert(store, "user", "你好，很高兴见到你", "问候", nullptr, 0);
    prism_insert(store, "user", "今天的天气真不错", "天气", nullptr, 0);
    prism_insert(store, "assistant", "我喜欢写代码，特别是Go", "编程", nullptr, 0);
    prism_insert(store, "user", "机器学习真有趣", "ML", nullptr, 0);
    prism_insert(store, "assistant", "时序忆阻器是一种新的存储范式", "忆阻器", nullptr, 0);

    printf("Inserted 5 memories (auto‑vectorized).\n\n");
    printf("========================================\n");
    printf("  Chrono-Memristor -- Prism Terminal\n");
    printf("  Type Chinese keywords to search.\n");
    printf("  Type :q to quit.\n");
    printf("========================================\n");

    std::string line;
    while (true) {
        printf("\nprism> ");
        std::getline(std::cin, line);
        if (line == ":q" || line == ":quit") break;
        if (line.empty()) continue;

        // 获取演化前状态
        int total = prism_count(store);
        std::vector<MemristorInfo> before_infos(total);
        prism_get_all_states(store, before_infos.data(), total);

        // ====== 计时（关闭演化，测量真实搜索延迟） ======
        prism_set_evolution(store, 0);
        int dim = prism_vocab_size();
        std::vector<float> qvec(dim);
        prism_text_to_vector(line.c_str(), qvec.data(), dim);

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

        // ====== 正式查询（top_k=3，触发演化） ======
        PrismResult results[10];
        int n = prism_search(store, qvec.data(), dim, 3, results, 0.0f);

        std::vector<MemristorInfo> after_infos(total);
        prism_get_all_states(store, after_infos.data(), total);

        // 输出表格
        printf("+----+------------------------------------------------+----------------------------+----------------------------+\n");
        printf("| ID | CONTENT                                        | BEFORE (CONDUCT / FLUX)     | AFTER (CONDUCT / FLUX)      |\n");
        printf("+----+------------------------------------------------+----------------------------+----------------------------+\n");
        for (int i = 0; i < total; ++i) {
            uint64_t id = before_infos[i].id;
            float before_cond = before_infos[i].conductance, before_flux = before_infos[i].correlation_flux;
            float after_cond  = after_infos[i].conductance,  after_flux  = after_infos[i].correlation_flux;

            std::string content;
            for (int j = 0; j < n; ++j) {
                if (results[j].id == id) {
                    content = results[j].content ? results[j].content : "(null)";
                    break;
                }
            }
            if (content.empty()) content = "memory #" + std::to_string(id);

            char before_str[64], after_str[64];
            snprintf(before_str, sizeof(before_str), "%8.4f / %8.4f", before_cond, before_flux);
            snprintf(after_str, sizeof(after_str), "%8.4f / %8.4f", after_cond, after_flux);

            printf("| %-2llu | %-46s | %-26s | %-26s |\n",
                   id, pad_content(content, 46).c_str(), before_str, after_str);
        }
        printf("+----+------------------------------------------------+----------------------------+----------------------------+\n");

        printf("Avg latency (10000 runs): %.1f ns\n", avg_ns);
        printf("Total elapsed: %.2f ms\n", total_ns / 1000000.0);
        printf("Total records: %d\n", total);

        prism_free_results(results, n);
    }

    prism_close(store);
    printf("Goodbye, Sama.\n");
    system("pause");
    return 0;
}