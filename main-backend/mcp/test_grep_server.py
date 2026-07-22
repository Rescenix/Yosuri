#!/usr/bin/env python3
# grep_server.py 的测试：直接跑真实的 JSON-RPC stdio 协议（initialize -> tools/list
# -> tools/call），跟 Go 后端 mcp_client.go 的握手同构，而不是只调内部函数——
# 这样连协议层（工具是否出现在 tools/list、参数解析）一起验到。
#
# 运行：python test_grep_server.py

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

SERVER = str(Path(__file__).with_name("grep_server.py"))


class Server:
    """把 grep_server 当子进程拉起，按行喂 JSON-RPC 请求、读响应。"""

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
        # notifications/initialized 无响应；有 id 的才等回包
        line = self.p.stdout.readline()
        return json.loads(line)

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


def call_tool(srv, mid, name, args):
    resp = srv.call(mid, "tools/call", {"name": name, "arguments": args})
    res = resp.get("result", {})
    text = "".join(b.get("text", "") for b in res.get("content", []))
    return text, res.get("isError", False)


def main():
    root = tempfile.mkdtemp()
    # 造一个 10 行的文件
    sample = Path(root) / "sample.txt"
    sample.write_text("\n".join(f"line{i}" for i in range(1, 11)) + "\n", encoding="utf-8")
    # 造一个 500 行的大文件，测行数上限
    big = Path(root) / "big.txt"
    big.write_text("\n".join(f"L{i}" for i in range(1, 501)) + "\n", encoding="utf-8")

    srv = Server(root)
    try:
        init = srv.call(1, "initialize", {"protocolVersion": "2024-11-05"})
        check(init.get("result", {}).get("serverInfo", {}).get("name") == "re0-grep", "initialize 握手")

        tools = srv.call(2, "tools/list").get("result", {}).get("tools", [])
        names = {t["name"] for t in tools}
        check("read_range" in names, "read_range 出现在 tools/list")

        # 中间任意行段：第 3-6 行（这是 head/tail 做不到的）
        text, err = call_tool(srv, 3, "read_range", {"path": "sample.txt", "start": 3, "end": 6})
        check(not err, "读中间行段不报错")
        check("3:line3" in text and "6:line6" in text, "含起止行且带行号前缀")
        check("2:line2" not in text and "7:line7" not in text, "区间外的行不出现")
        check("共 10 行" in text, "头部标注总行数")

        # 越界 end 自动裁剪到文件末行
        text, err = call_tool(srv, 4, "read_range", {"path": "sample.txt", "start": 8, "end": 999})
        check(not err and "10:line10" in text, "end 越界裁剪到末行")
        check("第 8-10 行" in text, "头部反映真实结束行")

        # start 超出文件范围：明确报错，不静默返回空
        text, err = call_tool(srv, 5, "read_range", {"path": "sample.txt", "start": 50})
        check(err and "超出文件范围" in text, "start 越界报错")

        # 行数上限：500 行文件从头读，最多 400 行，且提示续读
        text, err = call_tool(srv, 6, "read_range", {"path": "big.txt", "start": 1})
        lines = [l for l in text.split("\n") if l and l[0].isdigit()]
        check(len(lines) == 400, f"一次最多 400 行（实得 {len(lines)}）")
        check("start=401" in text, "提示用 start=401 续读")

        # 路径越界：不能读项目根外的文件
        text, err = call_tool(srv, 7, "read_range", {"path": "../../../etc/passwd", "start": 1})
        check(err and "越界" in text, "路径穿越被拒")

        # 缺省 end：从 start 读一屏
        text, err = call_tool(srv, 8, "read_range", {"path": "sample.txt", "start": 2})
        check(not err and "2:line2" in text and "10:line10" in text, "缺省 end 读到文件尾")

        # 不存在的文件
        text, err = call_tool(srv, 9, "read_range", {"path": "nope.txt", "start": 1})
        check(err and "不存在" in text, "读不存在的文件报错")

        # grep/glob 仍正常（没被改坏）
        text, err = call_tool(srv, 10, "grep", {"pattern": "line3", "path": "."})
        check(not err and "line3" in text, "grep 回归正常")
    finally:
        srv.close()

    print(f"\n{PASS} passed, {FAIL} failed")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
