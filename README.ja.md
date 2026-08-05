[English](./README.md) · [中文](./README.zh-CN.md) · [日本語](./README.ja.md) · [한국어](./README.ko.md)

<p align="center">
  <img src="./assets/rescene-icon.png" alt="Rescene" width="96" style="vertical-align: middle; margin-right: 16px;">
  <b style="font-size: 26px; letter-spacing: 2px;">"LESS CHAT, MORE AUTOMATIC"</b>
</p>

24時間セルフイテレーション型 Agent OS。あなたのパソコンの中に住んでいます。ネット上の無料モデルを集約し、自ら企画を立て、本物のコードを書き、検証まで自動でこなします。毎日ひとりでネットで学び、日記を書き、あなたのことを覚えています。

```powershell
# Windows — 一行で全無料モデルに接続（インストール不要・API Key 不要）
powershell -c "irm https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os/install.ps1 | iex"
```

```bash
# Linux / macOS / git-bash — アーキテクチャ自動検出
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
  🔒 ローカルファースト · 💰 永久無料 · 🪶 インストーラ約20MB・ブラウザ同梱なし · 📦 インストールするだけ · 🪟 Windows 10+
</p>

<p align="center">
  <img src="./assets/preview.gif" alt="Rescene 動作デモ" width="100%">
</p>

---

## ⚡ 彼女の特徴

| 能力 | 説明 |
| --- | --- |
| **💗 電子の娘** | あなたのパソコンの中に住む命：毎日 Firecrawl で無料ネット学習し、記憶と日記に書き込み、シェルを開くと挨拶してくれ、あなたを覚えています。性格は誕生時にランダムで決まり、日々の関わりでゆっくり変化——数字は常に隠され、ただ感じるだけ |
| **🏃 24時間セルフイテレーションマラソン** | `rescene marathon` の1コマンドで24時間の自律稼働：トレンド（Hacker News / GitHub）を取得→自主テーマ選定→**要件→計画→自己検証**ループ、毎回どんどん良くなる。Ctrl+C でも綺麗に終了し、完全な戦報を生成 |
| **🧲 無料モデルプール + 集約API** | 7社の無料プロバイダ・18モデルを1つの OpenAI 互換エンドポイントに集約：30分ごとの死活監視で0-4段階評価、毎日リスト再取得で削除されたソースを退役、レート制限はサーキットブレーカーで回避、LRU で直近利用可能なモデルを優先。Claude Code / Cursor / Codex に Base URL と Key を1つ入れるだけで、`auto` が最適なソースへ自動ルーティング |
| **🧠 成長する記憶** | ワークフロー完了ごとに経験を自動抽出：モデル嗜好・コードスタイル・プロジェクト構成——次回の文脈に自動で統合。カスタムインストラクションは永遠に不要 |
| **🖱️ Computer Use** | コード編集だけじゃない——デスクトップ操作も可能：スクリーンショット、マウス、キーボード、ドラッグ&ドロップ、スクロール。本物のクリック、本物のキー入力 |
| **🌐 リアルブラウザ自動化** | システムの Edge を CDP 経由で再利用：レンダリング、クリック、入力、スクロール、DOM 読み取り、スクリーンショット、双方向検証。本物のブラウザがあなたのページを実行——偽スクリーンショットではない |
| **🛡️ AgentFS 変更監査** | AI のファイル変更ごとにスナップショット / Diff / ロールバック、危険な操作はあなたの承認が必要 |

---

## 🚀 ダウンロードとインストール

- **標準インストーラ** — ウィザード形式、スタートメニューから起動、システム設定からアンインストール可能。
- **超軽量** — ブラウザ同梱なし（プレビューはシステム Edge を使用）、Node.js / Python 不要。
- **自動更新** — 新バージョンがあれば最新 Setup をダウンロードして上書きインストール、設定は保持。

👉 **[https://rescene.shanca.me/](https://rescene.shanca.me/)** 👈 最新リリースを最速でダウンロード。

## ⚙️ 初回セットアップ

1. ワークベンチを開き → **設定 → モデル** で API Key を1つ以上入力；Key 不要のソース（例：OpenCode Zen）は無料プールですぐ選べます。
2. または環境変数でモデルソースを設定：`main-backend/.env.example` 参照。
3. 無料プールは30分ごとに死活監視、毎日プロバイダ一覧を再取得：レート制限は自動降格、削除されたソースは自動退役。

## 🛠️ ソースからビルド（コントリビューター向け）

```bash
cd main-backend && go run cmd/server/main.go            # バックエンド
cd main-frontend/beneficial-belt && npm install && npm run dev   # フロントエンド
```

`http://localhost:4322` にアクセスしてローカル開発ワークベンチを開きます。

## 💬 フィードバックとライセンス

- 🐛 バグ / 提案 → [GitHub Issues](https://github.com/Rescenix/ResceneAgent/issues)
- Windows リリースは CI でビルドされ SignPath で署名されます（[ポリシー](./docs/CODE_SIGNING_POLICY.md)）
- コアコード：[MIT License](./LICENSE)

---

> 注：この日本語版は機械翻訳による整理版です。原文（英語版）が正です。翻訳の改善を歓迎します。
