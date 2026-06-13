#include "prism_store.h"
#include <iostream>
#include <vector>

int main() {
    const char* path = "test_memory.dat";
    
    // 清理旧文件
    remove(path);
    
    PrismStore* store = prism_open(path);
    if (!store) {
        std::cerr << "Failed to open store" << std::endl;
        return 1;
    }
    
    // 准备两条向量（2维，便于验证）
    std::vector<float> emb1 = {1.0f, 0.0f};
    std::vector<float> emb2 = {0.0f, 1.0f};
    std::vector<float> query = {0.6f, 0.8f};
    
    uint64_t id1 = prism_insert(store, "user", "hello", "", emb1.data(), 2);
    uint64_t id2 = prism_insert(store, "assistant", "world", "", emb2.data(), 2);
    
    std::cout << "Inserted: " << id1 << ", " << id2 << std::endl;
    std::cout << "Total count: " << prism_count(store) << std::endl;
    
    PrismResult results[2];
    int n = prism_search(store, query.data(), 2, 2, results, 0.0f);
    
    for (int i = 0; i < n; ++i) {
        std::cout << "Result " << i << ": id=" << results[i].id 
                  << " score=" << results[i].score 
                  << " role=" << results[i].role 
                  << " content=" << results[i].content << std::endl;
    }
    
    prism_free_results(results, n);
    prism_close(store);
    
    // 重新打开，验证持久化
    PrismStore* store2 = prism_open(path);
    std::cout << "After reopen, count: " << prism_count(store2) << std::endl;
    prism_close(store2);
    
    return 0;
}