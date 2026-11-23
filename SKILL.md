---
name: doctools-cli
description: AIエージェントおよびLLMが、PDF, PowerPoint, Excel, CSV, HTML, テキスト, 画像などのローカルドキュメントを高速・決定論的に閲覧・抽出・検索・加工するための Agent-Native CLI ツールです。
---

# `doctools-cli` コンパニオンスキル (Agent Companion Skill)

`doctools-cli` は、AI エージェント向けに最適化されたローカルドキュメント操作 CLI ツールです。
サブプロセス起動、Unix パイプ、および stdio MCP サーバー経由で高速に動作します。

## 🎯 コア原則 (Agent Rules)

1. **コンテキスト最小化 (Handle-Based Output)**:
   - 大規模な出力（1,000文字超）はメモリ内に全文をダンプせず、ローカルファイル（`--output`）に保存してファイルパス（ハンドル）を受け取ってください。
2. **標準入出力分離**:
   - 成功結果は `stdout` に JSON で出力されます。エラー・警告は `stderr` に出力されます。
3. **安全第一 (Non-destructive by default)**:
   - 既存ファイルの上書き時は `--force` が必要です。

---

## 🛠️ 主要コマンドリファレンス

### 1. 全文検索 (FTS: Bleve)
```bash
# インデックス作成（差分自動更新）
doctools-cli fts build <dir> [--include-ext .pdf,.xlsx] [--exclude-dir .git,node_modules]

# ハイライト・AIナビゲーション付きクエリ検索
doctools-cli fts query <index-path> "<keyword>" [--limit 10]
```

### 2. PDF 操作
```bash
# 高精度テキスト抽出 (ToUnicode CMap 解決・MuPDF)
doctools-cli pdf extract-text <file.pdf> [-o output.md] [--start-page 1 --end-page 5]

# ページ全画面画像化 (マルチモーダル VLM 向け)
doctools-cli pdf extract-pages <file.pdf> [-o ./tmp] [--dpi 150]
```

### 3. Excel 操作
```bash
# シート一覧
doctools-cli excel list-sheets <file.xlsx>

# Markdown 抽出（座標プレフィックス対応）
doctools-cli excel extract-markdown <file.xlsx> [-s Sheet1] [--with-coords]

# セル検索
doctools-cli excel search-cell <file.xlsx> "<query>"

# 安全パッチ適用 (事前検証・自動バックアップ付き)
doctools-cli excel patch-excel <file.xlsx> <patch.json>
```

### 4. PowerPoint 操作
```bash
# テキスト抽出
doctools-cli pptx extract-text <file.pptx> [-o output.md]

# スライド画像化 (Windows / COM)
doctools-cli pptx extract-images <file.pptx> [-s 1,2,3]
```

### 5. PageIndex (構造ツリー探索)
```bash
# ドキュメント構造ツリーの取得
doctools-cli pageindex tree <file> [--depth 2]

# ノード単位ピンポイント本文抽出
doctools-cli pageindex content <file> --node-id <id>
```

### 6. MCP サーバー
```bash
# stdio MCP サーバー起動 (JSON-RPC 2.0)
doctools-cli mcp
```
エージェント環境（Antigravity, Claude, Cursor 等）のツール定義で `doctools_run` を設定して呼び出します。
- `command: "read"`: 自動形式判定・Smart Dynamic Response 付きテキスト抽出
- `command: "search"`: 単一ファイル内ピンポイント検索
- `command: "capture"`: Smart Dynamic Capture (単一ページBase64注入 / 複数ページファイル保存)
