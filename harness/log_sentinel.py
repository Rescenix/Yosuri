"""日志自检模块 —— harness 自己盯自己的日志。

后台线程 tail logs/harness.log：发现 ERROR 级别日志行（排除哨兵自身的 [SENTINEL] 行）
→ 记录事件到 logs/incidents.log（JSON 行）
→ 执行预设动作（写 last_incident_action.json + 打 [SENTINEL] 处置日志）。

/status 暴露事件计数与最近事件，供外部监控拉取。
"""

import json
import logging
import threading
import time
from datetime import datetime
from pathlib import Path

logger = logging.getLogger("harness.sentinel")

BASE_DIR = Path(__file__).resolve().parent
LOGS_DIR = BASE_DIR / "logs"
HARNESS_LOG = LOGS_DIR / "harness.log"
INCIDENTS_LOG = LOGS_DIR / "incidents.log"
ACTION_FILE = LOGS_DIR / "last_incident_action.json"

_SELF_MARK = "[SENTINEL]"


class LogSentinel:
    def __init__(self, poll_interval: float = 0.5):
        self.poll_interval = poll_interval
        self.incidents = []  # 内存中的事件列表（供 /status）
        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._thread = None

    def start(self):
        LOGS_DIR.mkdir(exist_ok=True)
        HARNESS_LOG.touch(exist_ok=True)
        self._thread = threading.Thread(target=self._watch_loop, daemon=True, name="log-sentinel")
        self._thread.start()
        logger.info("%s 日志自检已启动，监视 %s", _SELF_MARK, HARNESS_LOG)

    def stop(self):
        self._stop.set()

    def _watch_loop(self):
        with open(HARNESS_LOG, "r", encoding="utf-8", errors="replace") as f:
            f.seek(0, 2)  # 只看启动之后的新日志
            while not self._stop.is_set():
                line = f.readline()
                if not line:
                    time.sleep(self.poll_interval)
                    continue
                line = line.rstrip("\n")
                if " ERROR " in line and _SELF_MARK not in line:
                    self._handle_incident(line)

    def _handle_incident(self, line: str):
        incident = {
            "detected_at": datetime.now().isoformat(timespec="milliseconds"),
            "log_line": line,
            "action": "preset:record_and_mark",
        }
        with self._lock:
            self.incidents.append(incident)
        # 记录
        with open(INCIDENTS_LOG, "a", encoding="utf-8") as f:
            f.write(json.dumps(incident, ensure_ascii=False) + "\n")
        # 预设动作：落一个自愈标记文件（真实场景可换成重启子系统/告警推送）
        ACTION_FILE.write_text(json.dumps(incident, ensure_ascii=False, indent=2), encoding="utf-8")
        logger.info("%s 检测到 ERROR → 已记录 incidents.log 并执行预设动作(record_and_mark): %s",
                    _SELF_MARK, line)

    def snapshot(self):
        with self._lock:
            return {"count": len(self.incidents), "recent": self.incidents[-5:]}
