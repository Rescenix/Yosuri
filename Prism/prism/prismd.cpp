// prismd.cpp — Prism 混沌引擎守护进程 v1.3 (HNSW index, cosmic background)
#include "memory_store.h"
#include "prism_hnsw.h"
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <winhttp.h>
#include <string>
#include <vector>
#include <sstream>
#include <cstring>
#include <cstdlib>
#include <cstdio>
#include <cstdint>

#pragma comment(lib, "ws2_32.lib")
#pragma comment(lib, "winhttp.lib")

static PrismStore* g_store = nullptr;
static const int PORT = 5666;
static bool g_bge_ready = false;

// 声明 memory_store.cpp 中的 HNSW 句柄（外部链接）
extern PrismHNSWHandle* g_hnsw;

// ---------- BGE 向量服务 ----------
std::vector<float> get_embedding(const std::string& text) {
    std::vector<float> vec;
    if (!g_bge_ready) return vec;

    HINTERNET hSession = WinHttpOpen(L"PrismD/1.2",
                                     WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                                     NULL, NULL, 0);
    if (!hSession) return vec;
    HINTERNET hConnect = WinHttpConnect(hSession, L"localhost", 6752, 0);
    if (!hConnect) { WinHttpCloseHandle(hSession); return vec; }
    HINTERNET hRequest = WinHttpOpenRequest(hConnect, L"POST", L"/", NULL, NULL, NULL, 0);
    if (!hRequest) { WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession); return vec; }

    std::string payload = text;
    if (!WinHttpSendRequest(hRequest, L"Content-Type: text/plain\r\n", -1,
                            (LPVOID)payload.c_str(), payload.size(), payload.size(), 0)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return vec;
    }
    if (!WinHttpReceiveResponse(hRequest, NULL)) {
        WinHttpCloseHandle(hRequest); WinHttpCloseHandle(hConnect); WinHttpCloseHandle(hSession);
        return vec;
    }

    DWORD statusCode = 0;
    DWORD statusCodeSize = sizeof(statusCode);
    if (WinHttpQueryHeaders(hRequest,
                            WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                            WINHTTP_HEADER_NAME_BY_INDEX,
                            &statusCode, &statusCodeSize, WINHTTP_NO_HEADER_INDEX) &&
        statusCode != 200) {
        WinHttpCloseHandle(hRequest);
        WinHttpCloseHandle(hConnect);
        WinHttpCloseHandle(hSession);
        return vec;
    }

    DWORD bytesRead;
    char buf[4096];
    while (WinHttpReadData(hRequest, buf, sizeof(buf), &bytesRead)) {
        if (bytesRead == 0) break;
        for (DWORD i = 0; i < bytesRead; i += 4) {
            float val;
            memcpy(&val, buf + i, 4);
            vec.push_back(val);
        }
    }
    WinHttpCloseHandle(hRequest);
    WinHttpCloseHandle(hConnect);
    WinHttpCloseHandle(hSession);
    return vec;
}

// ---------- BGE 端口检测 ----------
bool bge_ping(const char* host, int port) {
    WSADATA wsa;
    WSAStartup(MAKEWORD(2,2), &wsa);
    SOCKET sock = socket(AF_INET, SOCK_STREAM, 0);
    sockaddr_in addr;
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    addr.sin_addr.s_addr = inet_addr(host);
    int res = connect(sock, (sockaddr*)&addr, sizeof(addr));
    closesocket(sock);
    WSACleanup();
    return res == 0;
}

// ---------- PrimQL 解析 ----------
std::string handle_query(const std::string& ql) {
    printf("[DEBUG] raw query: '%s'\n", ql.c_str());
    fflush(stdout);

    std::istringstream iss(ql);
    std::string cmd;
    iss >> cmd;
    for (auto& c : cmd) c = toupper(c);

    if (cmd == "LOOM") {
        // 快速判空：如果记忆场为空，直接返回，避免阻塞和超时
        if (prism_count(g_store) == 0) {
            return "OK\n";
        }
        std::string query_text;
        std::getline(iss, query_text);
        size_t first = query_text.find_first_not_of(" \t");
        size_t last = query_text.find_last_not_of(" \t\r\n");
        if (first != std::string::npos)
            query_text = query_text.substr(first, last - first + 1);
        else
            query_text = "";

        if (query_text.empty()) return "ERROR no query\n";

        auto qvec = get_embedding(query_text);
        if (qvec.empty()) return "ERROR BGE failed\n";

        uint64_t ids[10];
        float scores[10];
        int n = prism_search_raw(g_store, qvec.data(), (int)qvec.size(), 3, ids, scores);

        std::ostringstream resp;
        resp << "OK\n";
        for (int i = 0; i < n; i++) {
            char role[128] = {0};
            char content[4096] = {0};
            if (prism_get_content(g_store, ids[i], role, sizeof(role), content, sizeof(content)) == 0) {
                resp << ids[i] << "\t" << scores[i] << "\t"
                     << role << "\t" << content << "\n";
            }
        }
        return resp.str();
    }
    else if (cmd == "ENGRAM") {
        std::string role, content;
        iss >> role;
        std::getline(iss, content);
        size_t first = content.find_first_not_of(" \t");
        size_t last = content.find_last_not_of(" \t\r\n");
        if (first != std::string::npos)
            content = content.substr(first, last - first + 1);
        else
            content = "";

        if (role.empty() || content.empty()) return "ERROR role and content required\n";

        auto vec = get_embedding(content);
        if (vec.empty()) return "ERROR BGE failed\n";

        uint64_t id = prism_insert(g_store, role.c_str(), content.c_str(), "", vec.data(), (int)vec.size());
        if (id == 0) return "ERROR insert failed\n";
        return "OK " + std::to_string(id) + "\n";
    }
    else if (cmd == "DRIFT") {
        prism_set_evolution(g_store, 1);
        return "DRIFT OK\n";
    }
    else if (cmd == "RESONATE") {
        return "RESONATE OK\n";
    }
    else if (cmd == "STATS") {
        try {
            int total = prism_count(g_store);
            if (total < 0) total = 0;
            std::vector<MemristorInfo> infos(total);
            if (total > 0) prism_get_all_states(g_store, infos.data(), total);

            std::ostringstream resp;
            resp << "OK\n";
            resp << "ID\tCONTENT\tCONDUCT\tFLUX\tCHAOS\n";
            for (int i = 0; i < total; i++) {
                char role[128] = {0}, content[4096] = {0};
                if (prism_get_content(g_store, infos[i].id, role, sizeof(role), content, sizeof(content)) != 0)
                    continue;
                char fmt[64];
                snprintf(fmt, sizeof(fmt), "%.40s", content);
                resp << infos[i].id << "\t"
                     << fmt << "\t"
                     << infos[i].conductance << "\t"
                     << infos[i].correlation_flux << "\t"
                     << infos[i].chaos << "\n";
            }
            return resp.str();
        } catch (const std::exception& e) {
            return std::string("ERROR STATS: ") + e.what() + "\n";
        } catch (...) {
            return "ERROR STATS: unknown exception\n";
        }
    }
    return "UNKNOWN\n";
}

// ---------- main ----------
int main() {
    WSADATA wsa;
    WSAStartup(MAKEWORD(2,2), &wsa);

    printf("PrismD v1.3 (HNSW index, HTTP support, cosmic background)\n");

    if (bge_ping("127.0.0.1", 6752)) {
        g_bge_ready = true;
        printf("⚡ BGE 语义引擎已连接\n");
    } else {
        printf("⚠️ BGE 未启动，语义检索不可用（请手动执行 python embed_server.py）\n");
    }

    g_store = prism_open("prismd_memory.dat");
    if (!g_store) {
        printf("Failed to open Prism store.\n");
        return 1;
    }

    // 初始化 HNSW 索引
    g_hnsw = prism_hnsw_create(16, 200, 50);
    printf("HNSW 索引已初始化。\n");

    // 从已有记忆数据中全量重建索引（正常人类思维：启动即重建，无需额外文件）
    if (g_store && prism_count(g_store) > 0) {
        int total = prism_count(g_store);
        printf("正在从已有记忆重建 HNSW 索引 (%d 条)...\n", total);
        
        for (auto& rec : g_store->records) {
            if (!rec.embedding.empty()) {
                prism_hnsw_insert(g_hnsw, rec.id, rec.embedding.data(), (int)rec.embedding.size());
            }
        }
        printf("HNSW 索引重建完成。\n");
    }

    printf("PrismD listening on port %d...\n", PORT);

    SOCKET listen_sock = socket(AF_INET, SOCK_STREAM, 0);
    sockaddr_in addr;
    addr.sin_family = AF_INET;
    addr.sin_port = htons(PORT);
    addr.sin_addr.s_addr = INADDR_ANY;
    bind(listen_sock, (sockaddr*)&addr, sizeof(addr));
    listen(listen_sock, SOMAXCONN);

    while (true) {
        SOCKET client = accept(listen_sock, NULL, NULL);
        char buf[4096];
        int len = recv(client, buf, sizeof(buf)-1, 0);
        if (len > 0) {
            buf[len] = 0;
            std::string request(buf, len);
            
            std::string query;
            bool is_http = false;

            if (strncmp(buf, "POST ", 5) == 0) {
                is_http = true;
                size_t body_pos = request.find("\r\n\r\n");
                if (body_pos != std::string::npos) {
                    query = request.substr(body_pos + 4);
                    while (!query.empty() && (query.back() == '\n' || query.back() == '\r')) query.pop_back();
                }
            } else if (strncmp(buf, "GET ", 4) == 0) {
                std::string http_resp = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nAccess-Control-Allow-Origin: *\r\nConnection: close\r\n\r\nPrismD is running. Send POST with PrimQL.\n";
                send(client, http_resp.c_str(), http_resp.size(), 0);
                closesocket(client);
                background_radiation_tick(g_store);
                continue;
            } else {
                query = request;
                while (!query.empty() && (query.back() == '\n' || query.back() == '\r')) query.pop_back();
            }

            if (!query.empty()) {
                std::string response;
                try {
                    response = handle_query(query);
                } catch (const std::exception& e) {
                    response = std::string("FATAL: ") + e.what() + "\n";
                } catch (...) {
                    response = "FATAL: unknown exception\n";
                }

                if (is_http) {
                    std::string http_resp = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nAccess-Control-Allow-Origin: *\r\nConnection: close\r\n\r\n" + response;
                    send(client, http_resp.c_str(), http_resp.size(), 0);
                } else {
                    send(client, response.c_str(), response.size(), 0);
                }
            }
        }
        closesocket(client);
        background_radiation_tick(g_store);
    }

    closesocket(listen_sock);
    prism_close(g_store);
    prism_hnsw_destroy(g_hnsw);
    WSACleanup();
    return 0;
}