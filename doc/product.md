# 製品概要

## 製品の目的
Gemini-CLI、Claude、Antigravity などの AI エージェントが、ローカル環境の各種ドキュメント（Excel, PowerPoint, PDF, Word, テキスト等）を高速かつ決定論的に操作・参照・検索できるようにするための **Go言語製 Agent-Native CLI ツール (`doctools-cli.exe`)**。

## コアコンセプト
- **"Agent-Native Dual Interface" (CLI & MCP)**: 
  - **CLI モード**: サブプロセス起動によるオーバーヘッドゼロ・Unix パイプ・決定論的 JSON 出力を提供する超高速実行モード。
  - **stdio MCP サーバーモード (`doctools-cli mcp`)**: エージェントの認知力学に基づき、Tool Definition Block に 1 級市民として常駐。`read_file` への安易な逃避や Python スクリプト自作の迷走を断ち切り、Office/PDF 操作を 1 ステップで直結発火。
- **統合ゲートウェイ (`doctools_run`)**:
  - ツールの乱立（Tool Overload）を防ぐため、Cloudflare 型の Dispatcher パターンを採用し、`read`（抽出）、`search`（単一ファイル内検索）、および `capture`（視覚化画像キャプチャ）のコアアクションに集約。拡張子に応じたエンジン（PDF/Excel/PPTX/Word 等）を自動判別。
- **Smart Dynamic Response (動的閾値ハンドリング)**:
  - 抽出結果が一定サイズ（例: 4,000文字以下）の場合は直接 Markdown を返却して 1 ターンで完結。
  - 閾値を超える長大ドキュメントの場合は、自動的にローカル Markdown ファイルへ保存し、ファイルパスとメタデータを返却してエージェントのコンテキスト溢れを決定論的に防止。
- **Smart Dynamic Capture (動的画像キャプチャ)**:
  - 単一ページ抽出（1枚）かつ `inline != false` の場合は、Base64 画像（`ImageContent`）を直接レスポンスに注入して AI の視覚推論器に提供。
  - 複数ページ抽出（2枚以上）または `inline == false` の場合は、画像ファイルをローカル保存してファイルハンドル一覧のみを返却し、コンテキスト溢れを防止。
- **シングルバイナリ・ゼロ依存**: Windows 環境で Python やサードパーティライブラリを一切インストールすることなく、単一の可搬バイナリ `doctools-cli.exe` を配置するだけで動作。
- **Context-Aware / Handle-Based**: 大容量データは標準出力に撒き散らさず、ローカルファイルへの出力パスを返却することで、AI エージェントのコンテキスト消費を最小化。

## ターゲットユーザー
- AI エージェント（Gemini-CLI, Antigravity, Claude, Codex 等）を活用して、ローカルの大量のドキュメント（仕様書、データシート、マニュアル、プレゼン資料等）を参照しながら開発や分析を行うエンジニア。
- AI エージェントに信頼性の高いドキュメント操作・検索能力を与えたい開発者。

## 主要機能
- **DocTools (ドキュメント操作 CLI)**:
  - **Excel**: シート一覧取得、シートCSV抽出、セル単位差分比較 (Diff)、安全ガードレール付きパッチ適用 (Patch)、座標付きMarkdown抽出、キーワード検索、シートFit-to-Page画像化 (`go-ole` 連携)。
  - **PowerPoint**: テキスト抽出、複数ファイル結合 (純Go)、スライド画像化 (`go-ole` 連携)。
  - **PDF**: ページ分割・結合・埋め込み画像抽出 (`pdfcpu`)、**MuPDF (go-fitz) による高精度日本語テキスト抽出**、全画面スライド画像化 (MuPDF CGO 静的リンク)。
  - **Word (.docx)**: 見出し階層や表（Markdown テーブル）を保持した純 Go 構造化テキスト抽出、単一ファイル内キーワード検索。
  - **CSV / Text / Image / HTML**: 高速なテキスト・画像・エンコーディング処理。
- **Agent-Native MCP サーバー (`doctools-cli mcp`)**:
  - **stdio JSON-RPC 2.0**: エージェント環境（Antigravity, Claude, Cursor 等）のコンテキストに常駐する組み込み MCP サーバー。
  - **統合ツール `doctools_run`**: 単一のツール窓口から `read`（自動判定・Smart Dynamic Response 付き抽出）、`search`（単一ファイル内ピンポイント検索）、および `capture`（Smart Dynamic Capture によるページ画像化）を提供。
- **Agent Introspection (3-Layer Introspection)**:
  - **Layer 1 (`--help`)**: 人間および初見エージェント向けヘルプ。
  - **Layer 2 (`doctools-cli agent-context`)**: 全コマンドの構造・型・引数スキーマを記述した機械判読可能な JSON 出力。
  - **Layer 3 (`SKILL.md`)**: エージェント向けコンパニオンスキル。
- **PageIndex (推論ベースRAG支援)**:
  - **Structure Extraction & Node Retrieval**: ドキュメント構造解析およびピンポイント取得（完全ローカル・決定論的探索）。
- **Search (全文検索)**:
  - **Bleve Search Engine**: 各種ドキュメント (Excel, PPT, PDF, Word, CSV, Text) を On-the-fly で Markdown/CSV テキストへ変換・チャンク分割し、ファイルパスやシート名・ページ番号などのメタデータ付きでインデックス登録。スコア、前後ハイライトスニペット、AI向けナビゲーション構造 (`target`) を返却する爆速全文検索。

## ビジネス・開発目標
AI エージェントがローカル知識ベースを高速かつ確実に探索できる環境を提供し、応答速度の劇的向上（ミリ秒単位の起動）とトークン消費削減を達成する。
また、Python 依存やインストール手順を完全に排除し、社内やチーム内での配布・共有を極限まで容易にする。
