#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <iostream>
#include <string>
#include <vector>
#include <cstdint>
#include <cstdio>
#include <sstream>

#pragma comment(lib, "ws2_32.lib")

std::string recv_all(SOCKET sock) {
    std::string result;
    char buf[4096];
    int n;
    while ((n = recv(sock, buf, sizeof(buf) - 1, 0)) > 0) {
        buf[n] = '\0';
        result += buf;
    }
    return result;
}

int main() {
    WSADATA wsa;
    WSAStartup(MAKEWORD(2, 2), &wsa);

    printf("⚡ Prism 混沌记忆终端 (5666)\n");
    printf("输入命令: LOOM <query>, ENGRAM <role> <content>, STATS, DRIFT\n");
    printf("输入 :q 退出\n\n");

    std::string line;
    while (true) {
        printf("prism> ");
        std::getline(std::cin, line);
        if (line == ":q") break;
        if (line.empty()) continue;

        SOCKET sock = socket(AF_INET, SOCK_STREAM, 0);
        sockaddr_in addr;
        addr.sin_family = AF_INET;
        addr.sin_port = htons(5666);
        addr.sin_addr.s_addr = inet_addr("127.0.0.1");

        if (connect(sock, (sockaddr*)&addr, sizeof(addr)) != 0) {
            printf("无法连接到 PrismD\n");
            closesocket(sock);
            continue;
        }

        // 安全解析命令：提取命令词，剩余部分作为参数
        std::istringstream iss(line);
        std::string cmd;
        iss >> cmd;
        for (auto& c : cmd) c = toupper(c);

        // 获取命令后的剩余文本（参数）
        std::string rest;
        std::getline(iss, rest);
        // 去掉前导和尾部空白
        size_t first = rest.find_first_not_of(" \t");
        size_t last  = rest.find_last_not_of(" \t\r\n");
        if (first != std::string::npos && last != std::string::npos)
            rest = rest.substr(first, last - first + 1);
        else
            rest = "";

        // 拼接最终发送的字符串：命令 + (可能的空格 + 参数) + 换行
        std::string full_cmd = cmd;
        if (!rest.empty())
            full_cmd += " " + rest;
        full_cmd += "\n";

        send(sock, full_cmd.c_str(), full_cmd.size(), 0);
        shutdown(sock, SD_SEND);

        std::string resp = recv_all(sock);
        printf("%s\n", resp.c_str());

        closesocket(sock);
    }

    WSACleanup();
    return 0;
}