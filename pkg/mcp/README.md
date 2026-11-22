# `pkg/mcp` - Model Context Protocol (MCP) Server Module

## 1. 責務 (Responsibility)
AI エージェント（Claude, Cursor, Antigravity 等）向けに、Model Context Protocol (MCP) の stdio トランスポートに基づく JSON-RPC 2.0 サーバー機能を提供します。
単一の統合ディスパッチャツール `doctools_run` を通じて、PDF, Excel, PPTX などのドキュメントに対する読み込み（`read`）およびピンポイント検索（`search`）をエージェントに提供します。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **stdio 厳格分離の原則**:
  - `stdout` は JSON-RPC 2.0 プロトコル通信専用とし、デバッグログや内部出力は一切出力してはならない（常に `stderr` を使用）。
- **上位依存の禁止**:
  - `pkg/mcp` は `pkg/cli` などの上位 CLI コマンド層に依存してはならない。
  - 各種ドキュメント処理エンジン（`pkg/pdf`, `pkg/excel`, `pkg/pptx`, `pkg/fts` など）をディスパッチ対象として呼び出す。
- **Smart Dynamic Response**:
  - 出力テキストサイズが設定閾値（デフォルト 4,000 文字）以下の場合はインラインで直接返却し、超過時は自動で Markdown ファイル保存を行いファイルパス情報を返却する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Newline-delimited JSON-RPC 2.0**:
  - *Why*: MCP 公式仕様に準拠し、外部プロセス実行型エージェントとの間でオーバーヘッドなくストリーム通信を行うため。
- **Unified Tool (`doctools_run`)**:
  - *Why*: ツール定義過多（Tool Overload）によるエージェントのコンテキスト圧迫や選択迷いを防ぎ、単一 Gateway でファイル形式判別と操作を完結させるため。
