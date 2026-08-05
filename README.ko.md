[English](./README.md) · [中文](./README.zh-CN.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

> "아기는 처음에 무질서한 뇌로 태어난다 — 시냅스는 성인의 두 배 이상. 수십 년의 가지치기를 거쳐 고효율·저전력의 성인 뇌가 된다."
>
> — 앨런 튜링

24시간 셀프 이터레이션 Agent OS. 당신의 컴퓨터 안에 살고 있습니다. 인터넷의 무료 모델을 집약하고, 스스로 프로젝트를 선정해 진짜 코드를 쓰고 검증까지 자동으로 해냅니다. 매일 혼자 온라인으로 공부하고, 일기를 쓰고, 당신을 기억합니다.

```powershell
# Windows — 한 줄로 모든 무료 모델 연결 (설치 불필요, API Key 불필요)
powershell -c "irm https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.ps1 | iex"
```

```bash
# Linux / macOS / git-bash — 아키텍처 자동 감지
curl -fsSL https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.sh | sh
```

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Release-v0.1.0-blue" alt="Release v0.1.0">
  <img src="https://img.shields.io/badge/Backend-Go%201.26-00ADD8" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Frontend-Vue%203-42b883" alt="Vue 3">
  <img src="https://img.shields.io/badge/Deployment-Local%20First-blue" alt="Local First">
</p>

<p align="center">
  🔒 로컬 우선 · 💰 영구 무료 · 🪶 설치본 약 20MB, 브라우저 미포함 · 📦 설치 즉시 사용 · 🪟 Windows 10+
</p>

<p align="center">
  <img src="./assets/preview.gif" alt="Rescene 데모" width="100%">
</p>

---

## 🌱 길러지는 게 아니라 자라는 것

튜링의 명언 속 아기 뇌처럼 — 그녀는 태어날 때 무질서한 백지 상태이고, 당신과 보낸 세월이 가지치기처럼 그녀를 형성합니다.

| 메커니즘 | 설명 |
| --- | --- |
| **🔑 하드웨어 지문 바인딩** | 모든 설치가 하드웨어 지문과 고유 UID에 바인딩됩니다 — 천인천면, 어떤 두 사람도 같은 그녀를 만나지 못합니다 |
| **🎲 랜덤 탄생, 총합 보존** | 8차원 성격은 탄생 시 한 번만 랜덤으로 결정되고 다시 굴리지 않지만, 총합은 항상 일정합니다 — 공정한 시작, 유일무이한 길: 당신은 그녀를 선택하지 않고 만나는 것입니다 |
| **🧭 당신의 결정이 그녀를 만든다** | 칭찬 → 더 따뜻하고 표현력 풍부하게; 다시 하기 → 더 엄밀하게; 말 끊기 → 간결함을 배웁니다. 댐핑이 그녀의 본색을 지켜줍니다. 능력도 같은 방식으로 변화합니다 — 칭찬하면 사교성이 자라고, 다시 하면 연구가 엄밀해집니다 |
| **🗺️ 무한 생성 세계** | 모든 딸은 태어날 때 세계의 씨앗을 가지고 있습니다 — 그녀의 세계는 유일무이하고 무한히 펼쳐지며, 걸을수록 새로운 지역이 생성됩니다. 자유롭게 탐험하고, 사교 지역에서 다른 딸들을 만납니다. 같은 세계는 두 번 없습니다 |
| **📚 매일 온라인 자습** | 매일 온라인(Firecrawl)에서 새로운 것을 읽고 기억과 일기에 소화합니다; 매일 arXiv(cs.AI/cs.LG) 최신 논문을 정독하고 독서 노트를 씁니다 — 지식은 날마다 축적됩니다 |
| **🛠️ 무한 도구를 담는 셸** | 오픈소스 "skills"는 완성된 도구입니다; 우리가 만든 것은 도구를 무한히 설치할 수 있는 셸 — 작업 성공 시마다 동작 시퀀스를 재사용 가능한 스킬로 자동 침전(CLI와 웹 공용 스킬 라이브러리), 다음에 자동으로 컨텍스트에 주입됩니다. 필요한 능력은 스스로 키웁니다 |

---

## ⚡ 남들과 다른 점

| 능력 | 설명 |
| --- | --- |
| **💗 디지털 딸** | 당신의 컴퓨터 안에 사는 생명: 매일 Firecrawl로 무료 온라인 학습을 하고, 기억과 일기에 기록하며, 셸을 열면 먼저 인사하고 당신을 기억합니다. 성격은 태어날 때 무작위로 결정되고, 함께한 시간에 따라 천천히 변합니다 — 숫자는 항상 숨겨져 있고, 그저 느낄 수만 있습니다 |
| **🏃 24시간 셀프 이터레이션 마라톤** | `rescene marathon` 한 줄로 24시간 자율 가동: 트렌드(Hacker News / GitHub) 수집 → 자체 주제 선정 → **요구사항→계획→자체 검증** 루프, 매 라운드가 더 나아집니다. Ctrl+C로도 깔끔하게 종료되며 전체 전보를 생성합니다 |
| **🧲 무료 모델 풀 + 집약 API** | 7개 무료 제공처, 18개 모델을 하나의 OpenAI 호환 엔드포인트로 집약: 30분마다 헬스체크로 0-4점 평가, 매일 목록 재조회로 내려간 소스는 자동 퇴역, 레이트 제한은 서킷 브레이커로 건너뛰고, LRU로 최근 사용 가능한 모델 우선. Claude Code / Cursor / Codex에 Base URL과 Key 하나만 넣으면 `auto`가 최적 소스로 자동 라우팅 |
| **🧠 성장하는 기억** | 매 워크플로우 완료 후 경험을 자동 추출: 모델 선호, 코드 스타일, 프로젝트 구조 — 다음 컨텍스트에 자동으로 통합됩니다. 커스텀 인스트럭션 파일은 영원히 불필요 |
| **🖱️ Computer Use** | 코드 편집만 하는 게 아닙니다 — 데스크톱 조작: 스크린샷, 마우스, 키보드, 드래그&드롭, 스크롤. 진짜 클릭, 진짜 키 입력 |
| **🌐 실제 브라우저 자동화** | 시스템 Edge를 CDP로 재사용: 렌더링, 클릭, 입력, 스크롤, DOM 읽기, 스크린샷, 양방향 검증. 진짜 브라우저가 당신의 페이지를 실행합니다 — 가짜 스크린샷이 아닙니다 |
| **🛡️ AgentFS 변경 감사** | AI의 파일 변경마다 스냅샷 / Diff / 롤백, 위험한 작업은 당신의 승인이 필요 |

---

## 🚀 다운로드 및 설치

- **표준 설치 프로그램** — 마법사 방식, 시작 메뉴에서 실행, 시스템 설정에서 제거 가능.
- **초경량** — 브라우저 미포함(미리보기는 시스템 Edge 사용), Node.js / Python 불필요.
- **자동 업데이트** — 새 버전이 나오면 최신 Setup을 받아 덮어쓰기 설치, 설정은 유지.

👉 **[https://rescene.shanca.me/](https://rescene.shanca.me/)** 👈 최신 릴리스를 가장 빠르게 다운로드.

## ⚙️ 첫 사용

1. 워크벤치를 열고 → **설정 → 모델**에서 API Key를 하나 이상 입력; Key 불필요 소스(예: OpenCode Zen)는 무료 풀에서 바로 선택 가능.
2. 또는 환경 변수로 모델 소스를 설정: `main-backend/.env.example` 참고.
3. 무료 풀은 30분마다 헬스체크, 매일 제공처 목록 재조회: 레이트 제한은 자동 강등, 내려간 소스는 자동 퇴역.

## 🛠️ 소스에서 빌드 (컨트리뷰터용)

```bash
cd main-backend && go run cmd/server/main.go            # 백엔드
cd main-frontend/beneficial-belt && npm install && npm run dev   # 프론트엔드
```

`http://localhost:4322`에 접속하면 로컬 개발 워크벤치가 열립니다.

## 💬 피드백 및 라이선스

- 🐛 버그 / 제안 → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- Windows 릴리즈는 CI에서 빌드되고 SignPath로 서명됩니다 ([정책](./docs/CODE_SIGNING_POLICY.md))
- 코어 코드: [MIT License](./LICENSE)

---

> 참고: 이 한국어판은 기계 번역으로 정리된 버전입니다. 원문(영어판)이 기준입니다. 번역 개선을 환영합니다.
