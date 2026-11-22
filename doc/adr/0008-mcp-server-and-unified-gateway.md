# 0008. Agent-Native MCP サーバー機能と統合ゲートウェイ (doctools_run) の導入

- ステータス: 承認済み (Accepted)
- 日付: 2026-11-22

## コンテキスト (Context)

これまで `doctools-cli` は、Unix パイプやサブプロセス起動によるオーバーヘッドゼロの Agent-Native CLI ツールとして設計・実装されてきた。
しかし、実際のコーディングエージェント（Antigravity, Claude, Cursor 等）の日常運用を観察すると、以下の重大な認知力学的課題が顕在化していた：

1. **アフォーダンスの落差と事前学習の重力**:
   エージェントにとって CLI ツールは「汎用シェル（`run_command`）の引数文字列を組み立てる」という 2 段階の間接参照を要する。プロンプトで「Office/PDF 操作には `doctools` を使え」と指示しても、事前学習で何億回も刻まれた `read_file` や Python スクリプト自作（`import pypdf` 等）の強烈な引力に抗えず、安易に逃避・迷走してタスクが失敗する。
2. **ツール過多による選択精度の劣化 (Tool Overload)**:
   仮に `doctools` の持つ多彩なサブコマンド（`pdf_extract_text`, `excel_extract_markdown`, `pptx_extract_text`, `fts_query` 等）を個別の MCP ツールとして大量に露出させると、Tool Definition スキーマだけで数万トークンを浪費し、エージェントが「どのツールを使うべきか」迷走して推論精度が急落する（Cloudflare MCP 等でも報告された課題）。
3. **コンテキスト浪費と 2 ステップ摩擦のジレンマ**:
   大容量テキストを直接返すとコンテキストが吹き飛ぶ（Lost in the Middle / Context Overflow）一方で、常にローカルファイル出力にすると「ファイル作成 ➔ `read_file`」の 2 ステップが強制され、数ページの小さな確認タスクにおいて手数が無駄に増える。

## 決定事項 (Decision)

`doctools-cli` 単一バイナリ内に **stdio MCP サーバー (`doctools-cli mcp`)** を組み込み、以下の 3 大設計原則に基づく統合ゲートウェイを導入する。

### 1. 単一の統合ゲートウェイ (`doctools_run`)
Cloudflare 型の Dispatcher パターンを採用し、エージェントに公開する MCP ツールを単一の `doctools_run` に集約する。
- **コアアクション**: `read`（テキスト・表の Markdown 抽出）および `search`（単一ファイル内ピンポイント検索）の 2 種に統合。
- **自動判別**: ファイル拡張子（`.pdf`, `.xlsx`, `.pptx`, `.docx` 等）に基づき、最適な内部エンジン（MuPDF, excelize, zip+xml 等）を自動選択。
- **Negative Affordance (Description Hacking)**: ツールの説明文に「PDF/Excel/PPTX の調査時は `read_file` や Python 自作を禁止し、必ず本ツールを使うこと」を明記し、毎ターン Tool Definition Block から強烈な注意を喚起。

### 2. Smart Dynamic Response (動的閾値ハンドリング)
抽出結果のサイズに応じ、返却方式を自動で切り替える：
- **閾値以内（デフォルト: 4,000 文字 / 約 1,000 トークン以下）**:
  抽出された Markdown テキストをレスポンス本文に **直接返却**（インライン返却）。エージェントは 1 ターンで即座に内容を解釈・回答可能。
- **閾値超過（長大ドキュメント）**:
  自動的に `.tmp/` または指定パスへ Markdown ファイルとして保存し、ファイルパス・総ページ数・文字数などのメタデータとメッセージを返却（Handle-based 返却）。コンテキスト溢れを決定論的に防止。

### 3. 単一ファイル内ピンポイント検索 (`search`)
ディレクトリ全体の FTS に加え、指定された 1 つのファイル（PDF, XLSX, PPTX）の内部を対象としたキーワード検索機能を提供。一致したページ番号・シート名・スライド番号および前後スニペットを構造化データとして返却する。

## 影響と結果 (Consequences)

### メリット
- **想起率の最大化**: Tool Definition Block に 1 級市民として常駐するため、`read_file` への逃避や Python 自作の迷走が根絶される。
- **最速のタスク完結**: 小さなドキュメントは 1 ステップで直接読め、巨大なドキュメントでも安全にファイル保存されるため、エージェントの思考が最短距離で完結する。
- **シングルバイナリの維持**: 外部の Node.js や Python ランタイムを一切使わず、Go 単一バイナリの `mcp` サブコマンドとして完結。

### デメリット・注意点
- MCP プロトコルハンドラ（JSON-RPC 2.0 over stdio）の実装・テストが必要。
- CLI モードと MCP モードの双方が同一のコアエンジン（`pkg/pdf`, `pkg/excel`, `pkg/pptx` 等）を参照するため、パッケージ間の単方向依存制約を厳格に維持する必要がある。
