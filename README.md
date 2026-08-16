# DocTools CLI (`doctools-cli`)

Gemini-CLI, Antigravity, Claude, Codex などの AI エージェントが、PDF, PowerPoint, Excel, CSV, HTML, テキスト, 画像などのローカルドキュメントを高速・決定論的に閲覧・検索・加工できるようにするための **Go言語製 Agent-Native CLI ツール (`doctools-cli.exe`)** です。

---

## 🚀 特長

1. **シングルバイナリ・ゼロ依存**:
   - Python 環境や多数の外部ライブラリを一切インストールすることなく、単一の可搬バイナリ `doctools.exe` を配置するだけで即座に動きます（Windows Office COM 連携、Bleve 全文検索エンジン、MuPDF CGO ラスタライザをすべて内蔵）。
2. **Agent-Native CLI 設計**:
   - MCP (Model Context Protocol) のトークンオーバーヘッドや接続待ちを排除。
   - 非対話的実行 (`--force`)、全データコマンドでの `--json` 構造化出力 (`stdout`/`stderr` 完全分離) を徹底保証。
3. **3-Layer Introspection (AI 自律探索機能)**:
   - `doctools-cli agent-context --json` コマンドを1回実行するだけで、全 36 サブコマンドの構文やパラメータ型スキーマを構造化 JSON で自己探索できます。

---

## 🤖 AI エージェント用指示プロンプト (`AGENTS.md` / システムプロンプト貼付用)

AI エージェント（Claude, Gemini, Cursor, Codex 等）に `doctools` CLI の存在を教え、トークンを節約しながら自律的に使いこなさせるためのプロンプトセットです。
プロジェクトの `AGENTS.md` や `CLAUDE.md`、またはシステムプロンプトにコピー＆ペーストしてご利用ください。

```markdown
# Agent Directive for DocTools CLI (`doctools-cli.exe`)

当プロジェクトでは、ローカルドキュメント（PDF, PPTX, Excel, CSV, テキスト, HTML, 画像）の解析・加工・検索に特化した Agent-Native CLI ツール `doctools-cli` が利用可能です。

## 基本原則
1. **構造化出力の必須化**: 全すべてのデータ取得・操作コマンドには必ず `--json` フラグを付与して実行し、`stdout` の JSON をパースして結果を得ること。
2. **非対話的実行**: 上書き・削除等の破壊的操作には `--force` フラグを付与し、プロンプト入力待ちでハングさせないこと。
3. **能力の自己探訪 (Introspection)**: ツールコマンドの引数型やオプション一覧を確認したい場合は、まず最初に `doctools-cli agent-context --json` を実行して探索すること。
4. **コンテキスト節約 (Handle-based)**: 画像抽出や大容量テキスト変換処理では、標準出力に全テキストを出力させず、出力ファイルパス（ハンドル）を受け取って必要な箇所のみ部分読み込みすること。

## 代表的なユースケースとコマンド実行例
- **Excel シート確認 & CSV / Markdown 抽出**:
  `doctools-cli excel list-sheets sample.xlsx --json`
  `doctools-cli excel extract-csv sample.xlsx --sheets "Sheet1" --output-dir ./out --json`
  `doctools-cli excel extract-markdown sample.xlsx --with-coords --output ./out/table.md --json`
- **Excel 差分比較 (Diff) & 事前検証付きパッチ適用 (Patch)**:
  `doctools-cli excel diff file_a.xlsx file_b.xlsx --json`
  `doctools-cli excel patch file.xlsx --patch-file patch.json --dry-run --json`
  `doctools-cli excel search-cell sample.xlsx "売上" --json`
- **Excel シートの Fit-to-Page 画像化 (視覚理解 VLM 向け)**:
  `doctools-cli excel extract-images sample.xlsx --output-dir ./out/imgs --json`
- **PowerPoint スライドテキスト抽出 / 画像化**:
  `doctools-cli pptx extract-text presentation.pptx --output ./out/slides.md --json`
  `doctools-cli pptx extract-images presentation.pptx --output-dir ./out/ppt_imgs --json`
- **PDF のテキスト抽出・埋め込み画像抽出・全画面スライド画像化**:
  `doctools-cli pdf extract-text doc.pdf --output ./out/doc.md --json`
  `doctools-cli pdf extract-images doc.pdf --output-dir ./out/img_objs --json`
  `doctools-cli pdf extract-pages doc.pdf --output-dir ./out/slides --dpi 150 --format png --json`
- **Bleve 全文検索 (On-the-fly チャンク化・差分更新・拡張子/ディレクトリフィルター & QueryString 複合検索)**:
  `doctools-cli fts build ./docs --index-path ./fts.bleve --include-ext md,csv,pdf --exclude-dir Temp,node_modules --verbose --json`
  `doctools-cli fts query ./fts.bleve "検索キーワード" --limit 10 --json`
  `# 大量ドキュメントからの高度検索レシピ (QueryString 構文):`
  `# 1. 特定システムかつ特定のフレーズ検索: "file_path:*sysA* AND content:\"認証機能\""`
  `# 2. PDF ファイルのみ、または除外指定: "file_type:.pdf AND content:要件定義 -旧版"`
  `# 3. Excel特定シート名での絞り込み: "unit_name:Sheet1 AND content:売上"`
  `# 4. PPTX/PDF特定スライド・ページでの絞り込み: "locator:\"slide=2\" AND content:構成図"`
```

---

## 🛠️ 主要機能・コマンド一覧

```bash
# 探索・メタ情報 (Layer 2 Introspection)
doctools-cli agent-context [--json]

# バージョン確認（--help の先頭にも表示されます）
doctools-cli --version

# Excel 操作 (excelize + go-ole Fit-to-Page + Diff/Patch/Markdown/Search)
doctools-cli excel list-sheets <file.xlsx> --json
doctools-cli excel extract-csv <file.xlsx> [--sheets <sheet1,sheet2>] [--output-dir <dir>] --json
doctools-cli excel extract-images <file.xlsx> [--output-dir <dir>] [--dpi <dpi>] --json
doctools-cli excel extract-markdown <file.xlsx> [--sheets <sheet1,sheet2>] [--with-coords] [--output <file.md>] --json
doctools-cli excel extract-text <file.xlsx> [--output <file.md>] --json  # extract-markdown の統一エイリアス
doctools-cli excel diff <file_a.xlsx> <file_b.xlsx> [--sheets <sheet1,sheet2>] --json
doctools-cli excel patch <file.xlsx> --patch-file <patch.json> [--backup] [--dry-run] [--schema] --json
# ※パッチファイルは整形式 JSON 配列 [...] を指定。old_value による安全ガードレールに対応。--schema でテンプレート出力。
doctools-cli excel search-cell <file.xlsx> <query> [--sheets <sheet1,sheet2>] --json

# PowerPoint 操作 (純Go Zip+XML + go-ole)
doctools-cli pptx extract-text <file.pptx> [--output <file.md>] --json
doctools-cli pptx merge <file1.pptx> <file2.pptx> ... --output <merged.pptx> --json
doctools-cli pptx extract-images <file.pptx> [--output-dir <dir>] [--width <w>] [--height <h>] --json

# PDF 操作 (pdfcpu + MuPDF CGO)
doctools-cli pdf extract-text <file.pdf> [--output <file.md>] --json
doctools-cli pdf split <file.pdf> --start-page 1 --end-page 3 --output <split.pdf> --json
doctools-cli pdf merge <file1.pdf> <file2.pdf> ... --output <merged.pdf> --json
doctools-cli pdf extract-images <file.pdf> [--output-dir <dir>] --json
doctools-cli pdf extract-pages <file.pdf> [-o <dir>] [--dpi <dpi>] [--format <png|jpg>] [--start-page <n>] [--end-page <n>] [--force] --json

# 全文検索 (On-the-fly チャンク化 Bleve 日本語検索・差分更新・拡張子/ディレクトリフィルター・隠しフォルダ自動除外)
doctools-cli fts build <target-dir> [-i <index-path>] [--include-ext <xlsx,pdf>] [--exclude-ext <tmp>] [--include-dir <dir1,dir2>] [--exclude-dir <node_modules,Temp,build>] [-t <10s>] [-f] [-v] --json
doctools-cli fts query <index-path> <query> [-l <limit>] --json
# ※デフォルトのインデックス対象は Office/PDF バイナリのみ。Markdown や CSV も含める場合は --include-ext md,csv,pdf を指定。
# ※.git や .obsidian, .trash などの隠しフォルダ（. 始まり）はデフォルトで自動除外されます。隠しフォルダを対象にする場合は --include-dir .obsidian のように明示指定してください。

# PageIndex 構造RAG探索 (目次・ツリー構造の段階的探索)
doctools-cli pageindex tree <file> [--node-id <id>] [--depth <n>] --json
doctools-cli pageindex content <file> --node-id <id> --json

# CSV / テキスト操作 (chardet 自動文字コード判定)
doctools-cli csv read-cells <file.csv> [--start-row <n>] [--end-row <n>] --json
doctools-cli csv search <file.csv> <query> --json
doctools-cli csv metadata <file.csv> --json
doctools-cli csv extract <file.csv> --output <subset.csv> --json
doctools-cli text head <file.txt> -n 10 --json
doctools-cli text tail <file.txt> -n 10 --json
doctools-cli text grep <file.txt> <pattern> --json
doctools-cli text metadata <file.txt> --json
doctools-cli text convert <file.txt> -e utf-8 --json
doctools-cli text copy-clipboard <text> --json

# HTML / 画像 / ユーティリティ
doctools-cli html extract-text <file.html> --json
doctools-cli image metadata <file.png> --json
doctools-cli image crop <file.png> --bounds <left,top,right,bottom> --json
doctools-cli image save-clipboard --json
doctools-cli util zip <file1> <file2> ... --output <archive.zip> --json
doctools-cli util unzip <archive.zip> --output-dir <dir> --json
```

---

## 📦 配布用バイナリの使い方

配布された ZIP アーカイブ（例: `doctools-cli-0.6.1-windows-amd64.zip`）を解凍し、中身の `doctools-cli.exe` を任意のフォルダに配置して PATH に通すだけでご利用いただけます。Python や追加インストーラは不要です。

```bash
# 動作確認
doctools-cli agent-context --json
```

---

## 📜 ライセンス

本プロジェクトは [Apache License 2.0](LICENSE) の下で公開されています。

