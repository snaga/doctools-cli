# `pkg/mcp` - Model Context Protocol (MCP) Server Module

## 1. 責務 (Responsibility)
AI エージェント（Claude, Cursor, Antigravity 等）向けに、Model Context Protocol (MCP) の stdio トランスポートに基づく JSON-RPC 2.0 サーバー機能を提供します。
単一の統合ディスパッチャツール `doctools_run` を通じて、PDF, Excel, PPTX, Word (.docx), CSV, Text, HTML などのドキュメントに対する読み込み（`read`）、ピンポイント検索（`search`）、および視覚化画像キャプチャ（`capture`）をエージェントに提供します。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **stdio 厳格分離の原則**:
  - `stdout` は JSON-RPC 2.0 プロトコル通信専用とし、デバッグログや内部出力は一切出力してはならない（常に `stderr` を使用）。
- **上位依存の禁止**:
  - `pkg/mcp` は `pkg/cli` などの上位 CLI コマンド層に依存してはならない。
  - 各種ドキュメント処理エンジン（`pkg/pdf`, `pkg/excel`, `pkg/pptx`, `pkg/docx`, `pkg/html` など）をディスパッチ対象として呼び出す。
- **セキュリティガードレール**:
  - `options.output_path` および `options.output_dir` に対するパストラバーサル（`..`）および危険な実行可能拡張子（`.exe`, `.bat`, `.cmd`, `.ps1` 等）の指定を拒否し、エージェント環境の安全性を担保する。
- **Smart Dynamic Response (read コマンド)**:
  - 出力テキストサイズが設定閾値（デフォルト 4,000 文字）以下の場合はインラインで直接返却し、超過時は自動で Markdown ファイル保存を行いファイルパス情報を返却する。
- **Smart Dynamic Capture (capture コマンド)**:
  - 抽出枚数が単一ページ（1枚）かつ `inline != false` の場合のみ、Base64 画像（`type: "image"` の MCP `ImageContent`）を直接レスポンスに注入して AI の視覚推論器に提供する。
  - 抽出枚数が複数ページ（2枚以上）または `inline == false` の場合は、コンテキスト爆発（トークン枯渇）を防止するため画像ファイルをディスクに保存し、ファイルハンドル一覧（パス・解像度・容量）のみを返却する。
  - `ToCallToolResult()` 内のテキスト JSON（`type: "text"`）チャンクに生 Base64 文字列を重複出力してはならない（`ImageBase64` は `json:"-"` で秘匿）。
  - プリンタ記法（例: `"1"`, `"2-4"`, `"1,3,5"`, `""`[空は全ページ]）をサポートし、不在ページはスキップ警告、全不在時は `status: "error"` を返却する。
- **Office COM 環境制約**:
  - PowerPoint/Excel のキャプチャは Windows 環境上の Microsoft Office COM を利用する。非 Windows または Office 未導入環境では、PDF キャプチャへの誘導を含む親切なエラー案内メッセージを返却する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Newline-delimited JSON-RPC 2.0**:
  - *Why*: MCP 公式仕様に準拠し、外部プロセス実行型エージェントとの間でオーバーヘッドなくストリーム通信を行うため。
- **Unified Tool (`doctools_run`)**:
  - *Why*: ツール定義過多（Tool Overload）によるエージェントのコンテキスト圧迫や選択迷いを防ぎ、単一 Gateway でファイル形式判別と操作を完結させるため。
