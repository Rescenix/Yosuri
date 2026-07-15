"""Watchdog —— harness 主进程的守护者，nssm 实际托管的入口。

拉起 app.py 子进程并监视：子进程一旦退出（崩溃/被 kill），
在 RESTART_DELAY 秒内自动重启，全程写 logs/watchdog.log。
收到 SIGTERM/SIGINT（nssm stop / Ctrl+C）时终止子进程后退出。
"""

import logging
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent
LOGS_DIR = BASE_DIR / "logs"
LOGS_DIR.mkdir(exist_ok=True)

RESTART_DELAY = float(os.environ.get("HARNESS_RESTART_DELAY", "2"))

_fmt = logging.Formatter("%(asctime)s %(levelname)s [WATCHDOG] %(message)s")
_fh = logging.FileHandler(LOGS_DIR / "watchdog.log", encoding="utf-8")
_fh.setFormatter(_fmt)
_ch = logging.StreamHandler(sys.stdout)
_ch.setFormatter(_fmt)
logger = logging.getLogger("watchdog")
logger.setLevel(logging.INFO)
logger.addHandler(_fh)
logger.addHandler(_ch)

_shutting_down = False
_child = None


def _handle_signal(signum, frame):
    global _shutting_down
    _shutting_down = True
    logger.info("收到信号 %s，终止子进程并退出", signum)
    if _child and _child.poll() is None:
        _child.terminate()


signal.signal(signal.SIGTERM, _handle_signal)
signal.signal(signal.SIGINT, _handle_signal)


def main():
    global _child
    restarts = 0
    logger.info("watchdog 启动 pid=%s restart_delay=%ss", os.getpid(), RESTART_DELAY)
    while not _shutting_down:
        _child = subprocess.Popen([sys.executable, str(BASE_DIR / "app.py")], cwd=str(BASE_DIR))
        logger.info("harness 子进程已启动 pid=%s（第 %d 次启动）", _child.pid, restarts + 1)
        code = _child.wait()
        if _shutting_down:
            logger.info("正常停机，子进程退出码=%s", code)
            break
        restarts += 1
        logger.warning("harness 子进程退出（code=%s）！%.1f 秒后自动重启（累计重启 %d 次）",
                       code, RESTART_DELAY, restarts)
        time.sleep(RESTART_DELAY)
    logger.info("watchdog 退出")


if __name__ == "__main__":
    main()
