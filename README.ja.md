[English](./README.en.md) · [中文](./README.md) · [正体中文](./README.zhtw.md) · [日本語](./README.ja.md) · [Tiếng Việt](./README.vi.md) · [தமிழ்](./README.ta.md)

# ResceneAgent

> フロントエンド特化のマルチエージェント作戦プラットフォーム —— IDE・ターミナル・ブラウザ・AIチームをひとつのチャットボックスに詰め込む。デジタル生命「Aurora」を中核とし、要件分解 → コード実装 → 実行検証までをチャット内で完結させる。

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?logo=go&logoColor=white" alt="Go >= 1.26">
  <img src="https://img.shields.io/badge/Node-%3E%3D22-339933?logo=nodedotjs&logoColor=white" alt="Node >= 22">
  <img src="https://img.shields.io/badge/LLM-Multi--Provider-ff69b4" alt="Multi-Provider LLM">
</p>

**語るだけでなく、書き、走らせ、検証する。**

---

## なぜ違うのか

一般的なコーディングアシスタントはコードについて「語る」ことはできる。ResceneAgent は「書く・走らせる・検証する」をガード付きの閉ループに作り上げた。二つの差別化された設計がプラットフォームの堀（もat）である：

### AgentFS：AI の書き込みに「トランザクション」を

AI が十個のファイルを連鎖編集し、五個目でクラッシュして最初の四個がすでにディスクに汚染されている —— これは従来の AI コーディングが抱える最も致命的な弱点だ。ResceneAgent はファイル書き込みを**隔離され、監査可能で、ロールバック可能なトランザクション層**へと再構築する：

- **メモリ・スナップショット隔離** —— 各変更はまず隔離スナップショットに落ち、プロジェクトを即座に汚染しない；
- **アトミック・コミット / ロールバック** —— コンパイル・テスト・人間の確認を経て初めてディスクへ書き出す；失敗時は全体が巻き戻り、プロジェクトはそのまま；
- **ピクセル単位の監査タイムライン** —— 各行の変更に出所が付き、承認 / 拒否を一行ずつ；
- **タイムトラベル** —— スナップショット・タイムラインを遡り、「どの版がまだコンパイルできたか」を特定。

思想的ルーツは VFS・Git・データベース・トランザクションといった成熟したパラダイムにあり；真の違いはこの能力を**体系的に「AI Agent 向けの独立した書き込みトランザクション層」として作り上げた**点にある。

### 記憶 / 状態の二軌：Agent を失業させない

毎回の新セッションがゼロから始まる？ResceneAgent はクロスセッション記憶とクロスプロジェクト状態を**Agent 向けの独立した記憶層**として実装する：

- **グローバル `MEMORY.md` + プロジェクト単位 `workdir.md`** の二軌、物理的に `~/rescene_data/` に隔離され、リポジトリを汚染しない；
- **Agent が能動的に書き込む** —— ツールを通じて「何を覚えておく価値があるか」を Agent が決定；ユーザーが覚えると選んだものだけが保存される；
- **セッション開始時に自動注入** —— ワークフロー起動時に無条件でシステムプロンプトに結合され、Agent は口を開く前にプロジェクト履歴を理解する。

---

## 機能

### 差別化された能力

- **AgentFS Trace：セッション単位の Git 跡木** —— サイドバーに絶えず伸びるスナップショット木、一セッション一軌跡、ノードをクリックでガラス質感の Diff カードを表示；すべての軌跡は隔離シャドウ Git リポジトリ由来で、メインリポジトリに余計なコミットを書かない。
- **Harness Flow：リアルタイム・ワークフロー構成図** —— チャット右側に内包されたキャンバスが、Gateway / Memory / LLM / Tools / Reply と Trace / Eval / Release 段階を本物のイベント駆動フロー図として繋ぎ、現在のリンクをハイライト。
- **Agent 主導 TODO** —— 複雑なタスク開始時に `pending / doing / done` 構造化リストを発行、SSE で入力欄上部へ押し出し、コンテキスト圧縮を跨いでも計画を失わず、ブレイクポイントから再開可能。
- **能動的ユーザー質問（Human-in-the-loop）** —— `ask_user` がワークフローの現場で構造化された意思決定（単選 / 複選 / 自由入力）を起こし、回答は正式なコンテキストとしてその場で再開、決して勝手に推測しない。
- **ブレイクポイント再開** —— 各ラウンドで自動スナップショット（メッセージ履歴・ツール・TODO・トークン計数）、再起動や切断後はフロントエンドが復元バーを表示し、ブレイクポイント・ラウンドから再再生。
- **リアルタイム描画・検証ブラウザ** —— Agent がフロントエンドファイルを編集後、本物の Chromium（CDP）が自動描画しパネルへ Screencast で戻す、iframe ではない；ナビゲーション / モバイル視口 / 外部で開く に対応。
- **スクリーンショット工件** —— ページ・スクリーンショットがツール呼び出し順にチャット流へ挿入され、既定折りたたみ・必要に応じ展開、Agent が自らページ証拠を納品。
- **エンタープライズ級の安全と検証** —— ① 不可逆操作（ファイル / ディレクトリ削除・移動）は一切の例外なし承認、YOLO モードでも通さない；② 終了時の強制ビルド + スクリーンショット検証ゲート（`go build` / `npm run build` + 本物の描画スクリーンショット）を、加点項目としてフロントエンドへ押し返す。

### 共通 IDE 体験（チャットパネルに統合）

内包された本物の PowerShell ターミナル（スニペット・パネル）、Monaco エディタ + 再帰ファイル木、VS Code 風 Diff プレビュー、メッセージ流式グラデーション・アニメーション、および全 UI を覆う二次元スキン・システム（Live2D 看板娘付き）。さらにブログ / CMS・電子書籍・図床・TTS・統計ダッシュボード等のモジュールを内蔵。

---

## システム構成

```
┌──────────────────────────────────────────────────────────────┐
│   beneficial-belt (Astro + Vue 3 + Naive UI)                    │
│   チャット・エディタ・ファイル木・ターミナル・プレビュー・Diff・スキン  │
│   └─ 設定画面：ユーザーが LLM 提供元 / API Key / モデルを自由入力（非ハードコード） │
├──────────────────────────────────────────────────────────────┤
│   main-backend (Go / Gin :8080)                                 │
│   四状態機ワークフロー・マルチAgent・リアルタイムTODO・主動質問・MCP・記憶 │
│   └─ マルチ提供元モデルルーティング：設定に応じ動的転送、内蔵/ハードコードLLMバックエンドなし │
├──────────────────────────────────────────────────────────────┤
│   AgentFS：セッション単位スナップショット木・シャドウGit・Diff監査・タイムトラベル │
├──────────────────────────────────────────────────────────────┤
│   記憶層：MEMORY.md(全局) + workdir.md(プロジェクト単位)、単一ファイル保存 │
├──────────────────────────────────────────────────────────────┤
│   外部 LLM クラウド API（ユーザー選択：Ollama / DeepSeek / Gemini / …） │
└──────────────────────────────────────────────────────────────┘
```

> LLM はプラットフォーム・バックエンドの一部ではない：提供元・API Key・具体的モデルはすべて**フロントエンド設定画面でユーザーが自由に設定**し、バックエンドは動的ルーティングとフェイルオーバーのみを行い、いかなるベンダーも内蔵・ハードコードしない。

## リポジトリ構成

```
re0/
├── main-backend/          # Go バックエンド (:8080)
│   ├── internal/handler/  # ワークフロー / AgentFS / プレビュー / ターミナル / MCP / スキル / サブAgent
│   ├── skills/            # 学習済スキル（ローカル取得、リポジトリ非収録）
│   └── mcp/               # 自研 MCP server（grep/shell/memory…）
├── main-frontend/beneficial-belt/   # Astro + Vue 3 フロントエンド
├── harness/               # Python スクリプト（MCP/テスト/ツール）
└── docs/                  # ドキュメント資産
```

## クイックスタート

### 前提依存

- Go >= 1.26
- Node.js >= 22
- Ollama（ローカル LLM、任意）
- Docker（コード・サンドボックス、任意）

### バックエンド

```bash
cd main-backend
# .env を設定（ADMIN_PASSWORD, JWT_SECRET, DEEPSEEK_API_KEY 等）
go run cmd/server/main.go
```

### フロントエンド

```bash
cd main-frontend/beneficial-belt
npm install
npm run dev    # http://localhost:4322
```

記憶層はバックエンド起動と共に自動で有効化され、別途デプロイは不要。

## 環境変数

| 変数 | 説明 |
|------|------|
| `ADMIN_PASSWORD` | 管理者パスワード（SHA-256 ダイジェスト照合、平文保存なし、バックドアなし） |
| `JWT_SECRET` | JWT 署名鍵（ResceneCloud と共有） |
| `DEEPSEEK_API_KEY` | DeepSeek API Key |
| `RESCENE_CLOUD_URL` | ResceneCloud 認証サービス基址（私有、未設定時は localhost:8088 へフォールバック） |
| `MCP_CONFIG` | MCP server 設定ファイルパス（既定 `./mcp.json`） |
| `RESCENE_DATA_DIR` | 記憶 / AgentFS / セッション・データ根ディレクトリ（既定 `~/rescene_data`） |

## ライセンス

本プロジェクトは [MIT License](./LICENSE) の下でオープンソース。認証・課金・商業ループは私有サービス ResceneCloud に留保され、オープンソースの re0 はいかなる鍵や OAuth ロジックも保持しない。

---

## Star History

<a href="https://star-history.com/#Rescenix/ResceneAgent&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/star-history-dark.png" />
    <source media="(prefers-color-scheme: light)" srcset="assets/star-history-light.png" />
    <img alt="Star History Chart" src="assets/star-history-light.png" width="100%" />
  </picture>
</a>

<sub>[`scripts/gen_star_history.py`](scripts/gen_star_history.py) により生成、GitHub Actions で毎日自動更新；画像をクリックでライブデータを表示。</sub>

> 注：この日本語版は機械翻訳です。原文（中国語）の意味に従い、専門用語は原文表記を優先しています。表現の不自然さはご了承ください。
