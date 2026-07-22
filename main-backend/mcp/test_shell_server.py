#!/usr/bin/env python3
# shell_server.py 的测试：真实 JSON-RPC stdio 协议（initialize -> tools/list ->
# tools/call），与 Go 后端 mcp_client.go 握手同构。
#
# 运行：python test_shell_server.py

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

SERVER = str(Path(__file__).with_name("shell_server.py"))


class Server:
    def __init__(self, root):
        env = dict(os.environ, MCP_ROOT=root)
        self.p = subprocess.Popen(
            [sys.executable, SERVER],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            text=True, env=env, bufsize=1,
        )

    def call(self, mid, method, params=None):
        req = {"jsonrpc": "2.0", "id": mid, "method": method}
        if params is not None:
            req["params"] = params
        self.p.stdin.write(json.dumps(req) + "\n")
        self.p.stdin.flush()
        return json.loads(self.p.stdout.readline())

    def close(self):
        self.p.stdin.close()
        self.p.terminate()


PASS, FAIL = 0, 0


def check(cond, msg):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  ✓ {msg}")
    else:
        FAIL += 1
        print(f"  ✗ {msg}")


def run(srv, mid, command):
    resp = srv.call(mid, "tools/call", {"name": "run", "arguments": {"command": command}})
    res = resp.get("result", {})
    text = "".join(b.get("text", "") for b in res.get("content", []))
    return text, res.get("isError", False)


def main():
    root = tempfile.mkdtemp()
    # 放一个已知文件，验证命令确实在 MCP_ROOT 下执行
    (Path(root) / "marker.txt").write_text("hello", encoding="utf-8")

    srv = Server(root)
    try:
        init = srv.call(1, "initialize", {"protocolVersion": "2024-11-05"})
        check(init.get("result", {}).get("serverInfo", {}).get("name") == "re0-shell", "initialize 握手")

        tools = srv.call(2, "tools/list").get("result", {}).get("tools", [])
        check({t["name"] for t in tools} == {"run"}, "tools/list 暴露 run")

        # 成功命令：退出码 0、输出里带命令与 exit 标注
        text, err = run(srv, 3, "echo re0test")
        check(not err, "正常命令不报错")
        check("re0test" in text, "拿到 stdout")
        check("exit=0" in text, "标注退出码 0")

        # cwd 确实是 MCP_ROOT：列目录应看到 marker.txt
        listcmd = "Get-ChildItem -Name" if os.name == "nt" else "ls"
        text, err = run(srv, 4, listcmd)
        check("marker.txt" in text, "cwd 锁在 MCP_ROOT（列到 marker.txt）")

        # 失败命令：非 0 退出码要标 isError
        badcmd = "exit 3" if os.name == "nt" else "exit 3"
        text, err = run(srv, 5, badcmd)
        check(err, "非 0 退出码标记为 error")
        check("exit=3" in text, "退出码如实反映（3）")

        # 空命令：明确报错，不静默
        text, err = run(srv, 6, "   ")
        check(err and "需要 command" in text, "空命令报错")

        # stderr 也被并回来（写到 stderr 的内容不能丢）
        if os.name == "nt":
            errcmd = "[Console]::Error.WriteLine('to_stderr')"
        else:
            errcmd = "echo to_stderr 1>&2"
        text, err = run(srv, 7, errcmd)
        check("to_stderr" in text, "stderr 内容被合并回来")
    finally:
        srv.close()

    print(f"\n{PASS} passed, {FAIL} failed")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
