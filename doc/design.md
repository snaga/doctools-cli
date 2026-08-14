# Agent-Native CLI 設計書 (`doctools-cli`)

## 目次
- [機能一覧](#機能一覧)
- [機能詳細](#機能詳細)
- [アーキテクチャ](#アーキテクチャ)
    - [設計方針](#設計方針)
    - [全体構成図](#全体構成図)
    - [レイヤー構造](#レイヤー構造)
    - [全体シーケンス図](#全体シーケンス図)
    - [データフロー図](#データフロー図)
- [インターフェース](#インターフェース)
    - [CLI インターフェース (API/Tool相当)](#cli-インターフェース-apitool相当)
    - [データインターフェース](#データインターフェース)
- [コンポーネント](#コンポーネント)
    - [ハイレベル・コンポーネント図](#ハイレベルコンポーネント図)
    - [コンポーネント・パッケージ詳細 IPO テーブル](#コンポーネントパッケージ詳細-ipo-テーブル)
- [データモデル](#データモデル)
    - [Bleve FTS インデックス構造](#bleve-fts-インデックス構造)
    - [PageIndex JSON スキーマ](#pageindex-json-スキーマ)
- [エラーハンドリング](#エラーハンドリング)

---

## 機能一覧

| 機能カテゴリ | 機能ID | 機能名 | 概要 | 対応要件ID |
|:---|:---|:---|:---|:---|
| 探訪 | INTRO-F01 | agent-context | CLI の全サブコマンドと引数型定義を JSON 出力 | NFR-03 |
| Excel操作 | EXCEL-F01 | excel list-sheets | Excel 内の全シート名一覧を取得 | EXCEL-01 |
| Excel操作 | EXCEL-F02 | excel extract-csv | Excel シートを個別の CSV ファイルとして抽出 | EXCEL-02 |
| Excel操作 | EXCEL-F03 | excel extract-images | Excel シートを Fit-to-Page 設定で高精度 PNG 化 (`go-ole`) | EXCEL-03 |
| Excel操作 | EXCEL-F05 | excel diff | Excel ファイル間のセル単位差分比較 (値, 数式, 結合範囲 `merged_range`) | EXCEL-05 |
| Excel操作 | EXCEL-F06 | excel patch | 自動バックアップ・BOMトリム・配列パッチ・`old_value`ガードレール付き安全パッチ適用 | EXCEL-06 |
| Excel操作 | EXCEL-F07 | excel extract-markdown | Excel シートの Markdown 抽出 (オプション `--with-coords` でセル座標付与) | EXCEL-07 |
| Excel操作 | EXCEL-F08 | excel search-cell | キーワードによるセル検索およびセル番地・結合情報取得 | EXCEL-08 |
| Excel操作 | EXCEL-F09 | excel extract-text | excel extract-markdown への統一エイリアス | EXCEL-09 |
| PowerPoint操作 | PPTX-F01 | pptx extract-text | PPTX スライドのテキストを Markdown として抽出 | PPTX-01 |
| PowerPoint操作 | PPTX-F02 | pptx merge | 複数の PPTX ファイルを 1 つにマージ | PPTX-02 |
| PowerPoint操作 | PPTX-F03 | pptx extract-images | PPTX スライドを指定解像度で PNG 化 (`go-ole`) | PPTX-03 |
| PDF操作 | PDF-F01 | pdf extract-text | PDF テキストを Markdown として抽出 (`pdfcpu`) | PDF-01 |
| PDF操作 | PDF-F02 | pdf split / pdf merge | PDF の特定ページ切り出し、および複数 PDF 結合 | PDF-02 |
| PDF操作 | PDF-F03 | pdf extract-images | PDF 埋め込み画像オブジェクトの抽出 (`pdfcpu`) | PDF-03 |
| PDF操作 | PDF-F04 | pdf extract-pages | PDF ページ全画面ラスタライズ画像化 (`go-fitz` / MuPDF CGO) | PDF-04 |
| 全文検索 (FTS) | FTS-F01 | fts build | Bleve エンジンによる並列 N-gram 全文検索インデックス構築 | FTS-01 |
| 全文検索 (FTS) | FTS-F02 | fts query | キーワードおよびブール条件による全文検索 (AI ナビゲーション付与) | FTS-02 |
| 全文検索 (FTS) | FTS-F03 | fts build --file-timeout | タイムアウト制御による安全なファイルスキップ | FTS-03 |
| 全文検索 (FTS) | FTS-F04 | fts build --include-ext | 対象拡張子の包含・除外フィルター | FTS-04 |
| 全文検索 (FTS) | FTS-F05 | fts query (Self-Describing) | AI エージェント向けクエリ構文・フィールド定義の自己説明 | FTS-05 |
| 構造RAG | PI-F01 | pageindex build | LLM サマリー付き構造 PageIndex インデックスの生成 | PAGEINDEX-01 |
| 構造RAG | PI-F02 | pageindex tree | 目次・ツリー構造の段階的取得 | PAGEINDEX-02 |
| 構造RAG | PI-F03 | pageindex content | 特定ノード / ページ / シートのフルテキストピンポイント抽出 | PAGEINDEX-03 |
| CSV操作 | CSV-F01 | csv read-cells | 指定した行・列範囲のセル値を JSON 配列取得 | CSV-01 |
| CSV操作 | CSV-F02 | csv search | CSV 内の文字列検索とセル位置取得 | CSV-02 |
| CSV操作 | CSV-F03 | csv metadata | CSV のエンコーディング・行数・列数取得 | CSV-03 |
| CSV操作 | CSV-F04 | csv extract | CSV の指定範囲を別 CSV へ切り出し抽出 | CSV-04 |
| テキスト操作 | TEXT-F01 | text head / text tail | テキストファイルの先頭 / 末尾行取得 | TEXT-01 |
| テキスト操作 | TEXT-F02 | text grep | 正規表現によるテキストファイル内検索 | TEXT-02 |
| テキスト操作 | TEXT-F03 | text metadata / text convert | 文字コード判定および相互変換 | TEXT-03 |
| テキスト操作 | TEXT-F04 | text copy-clipboard | クリップボードへのテキスト設定 (Windows) | TEXT-04 |
| HTML操作 | HTML-F01 | html extract-text | HTML ファイルから Markdown テキスト抽出 | HTML-01 |
| 画像操作 | IMAGE-F01 | image metadata | 画像の解像度（幅・高さ）等のメタデータ取得 | IMAGE-01 |
| 画像操作 | IMAGE-F02 | image crop | 画像の指定領域切り抜き (Crop) | IMAGE-02 |
| 画像操作 | IMAGE-F03 | image save-clipboard | クリップボード画像の PNG 保存 (Windows) | IMAGE-03 |
| 共通ユーティリティ | UTIL-F01 | util zip / util unzip | 複数ファイルの ZIP 圧縮、および解凍 | UTIL-01 |

---

## 機能詳細

### 機能カテゴリ: 探訪 (INTRO)
- **INTRO-F01: agent-context**
  - **概要**: CLI の全サブコマンド、引数型、フラグの構造化 JSON スキーマを出力する。
  - **対応要件**: NFR-03
  - **設計のポイント**: Cobra コマンドツリーを動的走査して JSON スキーマを自動生成する。

### 機能カテゴリ: Excel操作 (EXCEL)
- **EXCEL-F01: excel list-sheets**
  - **概要**: Excel ファイルの全シート名を取得する。
  - **対応要件**: EXCEL-01
  - **設計のポイント**: `excelize` を用い、メモリ消費を抑えてシート名一覧を取得。
- **EXCEL-F02: excel extract-csv**
  - **概要**: Excel シートを個別 CSV として書き出す。
  - **対応要件**: EXCEL-02
  - **設計のポイント**: `excelize` で高速抽出。デフォルトエンコーディングは UTF-8。
- **EXCEL-F03: excel extract-images**
  - **概要**: `go-ole` 経由で `PageSetup.FitToPagesWide = 1` を設定し、PDF 経由で高品質 PNG 化。
  - **対応要件**: EXCEL-03
  - **設計のポイント**: COM オブジェクト破棄とゾンビプロセス防止の徹底。
- **EXCEL-F05: excel diff**
  - **Input**: 比較元 `.xlsx` パス (`file_a`), 比較対象 `.xlsx` パス (`file_b`), `--sheets` (`-s`), `--json`
  - **Processing**: 2つの Excel を `excelize` で開き、共通シートの各セル値・数式・結合範囲 (`merged_range`) を比較。結合セルは代表セル（左上）のみ比較して `MergedRange` 情報を付与。
  - **Output**: 差分要素リスト (`file_a`, `file_b`, `total_changes`, `differences` 配列) を JSON またはテキスト出力。
  - **対応要件**: EXCEL-05
- **EXCEL-F06: excel patch**
  - **Input**: 対象 `.xlsx` パス, `--patch-file` (`-p`), `--backup` (`-b`), `--dry-run`, `--schema`, `--json`
  - **Processing**:
    1. `--schema` フラグ指定時: サンプルパッチ JSON テンプレートを出力して終了。
    2. BOM トリム & JSON アンマーシャル: `bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))` を実行後、整形式 JSON 配列 `[]ExcelPatchItem` として直感的にパース。失敗時は具体的なヒント付きエラーを出力。
    3. ガードレール検証:
       - `OldValue` (または `ExpectedOldValue`) が指定されている場合、現在のセル値と厳格比較（不一致時エラーアボート）。
       - `null` (または空ポインタ指定) の場合: `Guardrail bypassed` ログを記録し無条件上書き。
       - キー省略時: 警告 `Guardrail omitted` ログを記録し上書き。
    4. パッチ適用 & 監査ログ出力: `dryRun == false` 時ディスク保存。
  - **Output**: 監査ログ (`target_file`, `backup_file`, `dry_run`, `applied_count`, `audit_log`) を JSON 出力。
  - **対応要件**: EXCEL-06
- **EXCEL-F07: excel extract-markdown**
  - **Input**: 対象 `.xlsx` パス, `--sheets` (`-s`), `--with-coords`, `--output` (`-o`), `--json`
  - **Processing**: 指定シートのセル内容を Markdown テーブル形式で抽出。`--with-coords` フラグ指定時は各セル値の先頭にセル座標プレフィックス（例: `[C5]`）を付与。
  - **Output**: 生成された Markdown パスまたは Markdown テキスト。
  - **対応要件**: EXCEL-07
- **EXCEL-F08: excel search-cell**
  - **Input**: 対象 `.xlsx` パス, 検索クエリ, `--sheets` (`-s`), `--json`
  - **Processing**: 指定シートの全セル内容を部分一致/正規表現で検索。一致セルの番地、値、シート名、結合範囲情報を抽出。
  - **Output**: 一致結果リスト (`total_matches`, `matches` 配列) を JSON 出力。
  - **対応要件**: EXCEL-08
- **EXCEL-F09: excel extract-text**
  - **概要**: `excel extract-markdown` への統一エイリアス。
  - **対応要件**: EXCEL-09

### 機能カテゴリ: PowerPoint操作 (PPTX)
- **PPTX-F01: pptx extract-text**
  - **概要**: PPTX スライドテキストを Markdown ファイルに抽出保存。
  - **対応要件**: PPTX-01
  - **設計のポイント**: 純 Go (zip+xml) でスライドツリーを走査。
- **PPTX-F02: pptx merge**
  - **概要**: 複数の PPTX ファイルを1つにマージ保存。
  - **対応要件**: PPTX-02
  - **設計のポイント**: 純 Go で SlideMaster / XML 構造を保持して結合。
- **PPTX-F03: pptx extract-images**
  - **概要**: 指定スライドを高精度 PNG 画像として書き出し。
  - **対応要件**: PPTX-03
  - **設計のポイント**: `go-ole` 経由で `PowerPoint.Application` の `slide.Export` を呼び出し。

### 機能カテゴリ: PDF操作 (PDF)
- **PDF-F01: pdf extract-text**
  - **概要**: PDF テキストを Markdown ファイルへ抽出保存。
  - **対応要件**: PDF-01
  - **設計のポイント**: `pdfcpu` を用い、高速にテキスト抽出。
- **PDF-F02: pdf split / merge**
  - **概要**: PDF ページ範囲切り出しおよび複数 PDF 結合。
  - **対応要件**: PDF-02
  - **設計のポイント**: `pdfcpu` により高速なバイナリ結合・分割処理。
- **PDF-F03: pdf extract-images**
  - **概要**: PDF 内部に埋め込まれた画像オブジェクト (JPEG/PNG) を抽出し保存。
  - **対応要件**: PDF-03
  - **設計のポイント**: `pdfcpu` により埋め込み画像オブジェクトを無劣化で一括抽出。
- **PDF-F04: pdf extract-pages**
  - **概要**: PDF 各ページを全画面ラスタライズレンダリングし、指定 DPI および画像フォーマットで一括出力。
  - **対応要件**: PDF-04
  - **設計のポイント**: `go-fitz` (MuPDF CGO) バインディングにより、高速かつ高精細にページ描画を行い画像ファイルとして保存。

### 機能カテゴリ: 全文検索 (FTS)
- **FTS-F01: fts build**
  - **概要**: 指定フォルダの全文書 (Excel, PPT, PDF, CSV, Text) を On-the-fly で Markdown/CSV テキストへ変換・チャンク化し、差分チェックを行いメタデータ付き Bleve インデックスを構築。
  - **対応要件**: FTS-01
  - **設計のポイント**: 各形式パーサーによる On-the-fly テキスト化、`mtime` 比較による未変更ファイルの高速スキップ、Goroutine 並列処理、N-gram 解析、メタデータ (`file_path`, `unit_type`, `locator` 等) の保存。
- **FTS-F02: fts query**
  - **概要**: キーワードおよびブール演算子による全文検索を行い、コンテキスト前後スニペットとAI向けナビゲーション情報を返却。
  - **対応要件**: FTS-02
  - **設計のポイント**: Bleve ハイライター/フラグメンターによる前後テキスト抽出、`source` (共通ファイル情報) および `target` (形式別位置情報) を持つ JSON レスポンス構造。
- **FTS-F03: fts build --file-timeout**
  - **概要**: 重いファイルや破損ファイルのパース処理を `context.WithTimeout` で安全にスキップ。
  - **対応要件**: FTS-03
- **FTS-F04: fts build --include-ext / --exclude-ext**
  - **概要**: デフォルトで Office/PDF バイナリのみを対象とし、`--include-ext` / `--exclude-ext` で柔軟に拡張子を指定。
  - **対応要件**: FTS-04
- **FTS-F05: fts query (AIエージェント向け Self-Describing 強化)**
  - **概要**: `fts query` コマンドの Command `Long` 説明文に Bleve QueryString の構文ルール、具体例、および検索可能フィールド一覧を明記。
  - **対応要件**: FTS-05

### 機能カテゴリ: 構造RAG (PAGEINDEX)
- **PAGEINDEX-F01: pageindex build**
  - **概要**: `google.golang.org/genai` を用い、ドキュメントの階層構造サマリーを生成。
  - **対応要件**: PAGEINDEX-01
- **PAGEINDEX-F02: pageindex tree**
  - **概要**: ドキュメントの目次・階層構造ツリーの段階的取得。
  - **対応要件**: PAGEINDEX-02
- **PAGEINDEX-F03: pageindex content**
  - **概要**: 特定ノード ID の全文コンテンツのピンポイント抽出。
  - **対応要件**: PAGEINDEX-03

### 機能カテゴリ: CSV操作 (CSV)
- **CSV-F01: csv read-cells / CSV-F02: csv search / CSV-F03: csv metadata / CSV-F04: csv extract**
  - **概要**: CSV セル抽出・値検索・自動エンコーディング判定 (`chardet`)・別ファイル保存。
  - **対応要件**: CSV-01, CSV-02, CSV-03, CSV-04

### 機能カテゴリ: テキスト操作 (TEXT)
- **TEXT-F01: text head/tail / TEXT-F02: text grep / TEXT-F03: text metadata/convert / TEXT-F04: text copy-clipboard**
  - **概要**: 先頭・末尾行表示、正規表現 Grep 検索、エンコーディング判定 (`chardet`) 変換、Windows クリップボード設定。
  - **対応要件**: TEXT-01, TEXT-02, TEXT-03, TEXT-04

### 機能カテゴリ: HTML / IMAGE / UTIL
- **HTML-F01: html extract-text / IMAGE-F01: image metadata / IMAGE-F02: image crop / IMAGE-F03: image save-clipboard / UTIL-F01: util zip/unzip**
  - **概要**: HTML テキスト抽出、画像メタデータ・切り抜き・クリップボード保存、ZIP 圧縮・解凍。
  - **対応要件**: HTML-01, IMAGE-01〜03, UTIL-01

---

## アーキテクチャ

### 設計方針
1. **Agent-Native CLI 原則の徹底**: 非対話型・JSON構造化出力 (`stdout`/`stderr` 完全分離)・3-Layer Introspection (Layer 1: `--help`, Layer 2: `agent-context`, Layer 3: `SKILL.md`)。
2. **シングルバイナリ化 (Single Binary)**: CGO 静的リンクによる MuPDF / Bleve / `go-ole` の内蔵。
3. **ドメインの明確分離**: ドキュメント操作 / 全文検索 (`fts`) / 構造 RAG (`pageindex`) の分離。

### 全体構成図

```mermaid
graph TD
    Agent["AI Agent / LLM"] -->|CLI Flag & JSON| CLI["CLI Entrypoint: Cobra"]
    
    subgraph SingleBinary["Single Binary (doctools-cli.exe)"]
        CLI --> ExcelSvc["pkg/excel"]
        CLI --> PPTXSvc["pkg/pptx"]
        CLI --> PDFSvc["pkg/pdf"]
        CLI --> FTSSvc["pkg/fts"]
        CLI --> PISvc["pkg/pageindex"]
        CLI --> CSVSvc["pkg/csv"]
        CLI --> TextSvc["pkg/text"]
        CLI --> UtilSvc["pkg/util"]
        
        ExcelSvc --> Excelize["excelize Engine"]
        PDFSvc --> PDFCpu["pdfcpu Engine"]
        PDFSvc --> MuPDF["MuPDF CGO Static Lib"]
        FTSSvc --> Bleve["Bleve Search Engine"]
        PISvc --> GenAI["google.golang.org/genai"]
        CSVSvc & TextSvc --> Chardet["saintfish/chardet"]
    end
    
    subgraph WindowsSystem["Windows System"]
        ExcelSvc -->|go-ole| ExcelApp["Excel.Application"]
        PPTXSvc -->|go-ole| PPTApp["PowerPoint.Application"]
        TextSvc & UtilSvc -->|go-ole| WinClip["Windows Clipboard"]
    end
```

### レイヤー構造
- **Presentation Layer (`pkg/cli`)**: フラグ入力解析、TTY 判定、3-Layer Introspection, JSON 構造化レスポンス成形。
- **Service Layer (`pkg/excel`, `pkg/pdf`, `pkg/fts`, `pkg/pageindex` 等)**: 各ドメインのビジネスロジックおよびファイル処理。
- **Infrastructure / Native Layer**: `go-ole` COM バインディング、MuPDF CGO 静的バインディング、`saintfish/chardet` エンコーディング判定、Bleve ストレージ。

### 全体シーケンス図

```mermaid
sequenceDiagram
    participant Agent as AI エージェント
    participant CLI as pkg/cli (Cobra)
    participant Svc as pkg/excel (Service)
    participant OLE as go-ole Binding
    participant COM as Excel.Application
    
    Agent->>CLI: doctools-cli excel extract-images sample.xlsx --json
    CLI->>Svc: ExtractImages("sample.xlsx")
    Svc->>OLE: CoInitialize & CreateObject
    OLE->>COM: Workbooks.Open & PageSetup.FitToPagesWide=1
    COM-->>OLE: ExportAsFixedFormat(PDF)
    OLE->>COM: Workbook.Close & Application.Quit
    Svc->>Svc: Convert PDF to PNG (MuPDF)
    Svc-->>CLI: Return Image Paths List
    CLI-->>Agent: stdout: {"status": "success", "data": {"output_paths": [...]}}
```

### データフロー図

```mermaid
flowchart LR
    DocFiles["ローカル文書群 PDF/PPTX/XLSX"] -->|Goroutine並列走査| Extractor["テキスト抽出エンジン"]
    Extractor -->|PlainText| Tokenizer["N-gram 日本語トークナイザ"]
    Tokenizer -->|Inverted Index| BleveWriter["Bleve 永続化ライブラリ"]
    BleveWriter -->|Index File| IndexStorage["./.doctools_fts.index"]
```

---

## インターフェース

### CLI インターフェース (全 36 コマンド IPO)

全サブコマンドは標準で `--json` フラグ（構造化 JSON 出力）および `--force` フラグ（非対話的実行）をサポートします。

1. **`doctools-cli agent-context`**
   - Input: `--json` (bool)
   - Processing: Cobra コマンドツリー走査による JSON スキーマ生成
   - Output: 全コマンド構文・パラメータ定義の JSON
2. **`doctools-cli excel list-sheets <file>`**
   - Input: `file` (string)
   - Processing: `excelize` によるシート名一覧取得
   - Output: `{"sheets": ["Sheet1", "Sheet2"]}`
3. **`doctools-cli excel extract-csv <file>`**
   - Input: `file` (string), `--sheets` (string), `--output-dir` (string)
   - Processing: シート走査および CSV 保存
   - Output: `{"output_paths": [".../Sheet1.csv"]}`
4. **`doctools-cli excel extract-images <file>`**
   - Input: `file` (string), `--output-dir` (string), `--dpi` (int)
   - Processing: `go-ole` 経由で Fit-to-Page PDF 生成後、MuPDF で PNG 化
   - Output: `{"output_paths": [".../Sheet1_p1.png"]}`
5. **`doctools-cli excel diff <file_a> <file_b>`**
   - Input: `file_a` (string), `file_b` (string), `--sheets` (string)
   - Processing: セル単位での値・数式・結合範囲差分抽出
   - Output: `{"total_changes": 2, "differences": [...]}`
6. **`doctools-cli excel patch <file>`**
   - Input: `file` (string), `--patch-file` (string), `--backup` (bool), `--dry-run` (bool), `--schema` (bool)
   - Processing: `old_value` 事前検証、自動バックアップ、結合セル代表代入
   - Output: `{"applied_count": 1, "audit_log": [...]}`
7. **`doctools-cli excel extract-markdown <file>`**
   - Input: `file` (string), `--sheets` (string), `--with-coords` (bool), `--output` (string)
   - Processing: Markdown テーブル形式抽出（セル座標プレフィックス対応）
   - Output: `{"output_path": ".../table.md"}`
8. **`doctools-cli excel search-cell <file> <query>`**
   - Input: `file` (string), `query` (string), `--sheets` (string)
   - Processing: 指定シート内の全セル文字列検索
   - Output: `{"total_matches": 3, "matches": [...]}`
9. **`doctools-cli excel extract-text <file>`**
   - Input: `file` (string), `--sheets` (string), `--with-coords` (bool), `--output` (string)
   - Processing: `excel extract-markdown` の統一エイリアス実行
   - Output: `{"output_path": ".../table.md"}`
10. **`doctools-cli pptx extract-text <file>`**
    - Input: `file` (string), `--output` (string)
    - Processing: Zip+XML スライド解析と Markdown 書き出し
    - Output: `{"output_path": ".../presentation.md"}`
11. **`doctools-cli pptx merge <files...>`**
    - Input: `files` ([]string), `--output` (string)
    - Processing: スライド構造の結合
    - Output: `{"output_path": ".../merged.pptx"}`
12. **`doctools-cli pptx extract-images <file>`**
    - Input: `file` (string), `--slides` (string), `--width` (int), `--height` (int)
    - Processing: `go-ole` 経由で PowerPoint スライド Export
    - Output: `{"output_paths": [".../slide_001.png"]}`
13. **`doctools-cli pdf extract-text <file>`**
    - Input: `file` (string), `--output` (string)
    - Processing: `pdfcpu` によるテキスト抽出
    - Output: `{"output_path": ".../document.md"}`
14. **`doctools-cli pdf split <file>`** / **`pdf merge <files...>`**
    - Input: `file` (string), `--pages` (string) / `files` ([]string), `--output` (string)
    - Processing: `pdfcpu` ページ分離および結合
    - Output: `{"output_path": ".../out.pdf"}`
15. **`doctools-cli pdf extract-images <file>`**
    - Input: `file` (string), `--output-dir` (`-o`) (string), `--pages` (string)
    - Processing: `pdfcpu` による埋め込み画像オブジェクトの抽出
    - Output: `{"output_paths": [".../img_001.png"]}`
16. **`doctools-cli pdf extract-pages <file>`**
    - Input: `file` (string), `--output-dir` (`-o`) (string), `--dpi` (int, デフォルト 150), `--format` (string, デフォルト "png"), `--start-page` (int, デフォルト 1), `--end-page` (int, デフォルト 0)
    - Processing: `go-fitz` (MuPDF) による各ページの全画面ラスタライズ保存
    - Output: `{"total_pages": 3, "dpi": 150, "format": "png", "output_paths": [".../report_page_01.png"]}`
17. **`doctools-cli fts build <dir>`**
    - Input: `dir` (string), `--index-dir` (string), `--file-timeout` (duration, デフォルト 10s), `--include-ext` (stringSlice), `--exclude-ext` (stringSlice), `--verbose` (bool)
    - Processing: On-the-fly テキスト化・`mtime` 差分比較・タイムアウト制御・Bleve インデックス書き込み
    - Output: `{"indexed_files": 30, "skipped_files": 90, "timeout_files": 0, "indexed_chunks": 120, "time_ms": 450}`
18. **`doctools-cli fts query <query>`**
    - Input: `query` (string), `--limit` (int)
    - Processing: Bleve ハイライター/フラグメンターによる前後スニペットと AI ナビゲーション付与
    - Output: `{"total_hits": 5, "hits": [{"score": 1.452, "snippet": "...", "source": {...}, "target": {...}}]}`
19. **`doctools-cli pageindex build <dir>`**
    - Input: `dir` (string)
    - Processing: `genai` SDK を用いた構造解析と JSON 永続化
    - Output: `{"index_file": ".../pageindex.json"}`
20. **`doctools-cli pageindex tree <file>`**
    - Input: `file` (string), `--node-id` (string), `--depth` (int)
    - Processing: 目次ツリーの部分切り出し
    - Output: `{"structure": [...]}`
21. **`doctools-cli pageindex content <file>`**
    - Input: `file` (string), `--node-id` (string)
    - Processing: ノード ID に紐づく本文抽出
    - Output: `{"content": "..."}`
22. **`doctools-cli csv read-cells <file>`**
    - Input: `file` (string), `--start-row` (int), `--end-row` (int)
    - Processing: `encoding/csv` ストリーム読み込み
    - Output: `{"cells": [[...]]}`
23. **`doctools-cli csv search <file>`**
    - Input: `file` (string), `--query` (string)
    - Processing: キーワード一致セル検索
    - Output: `{"matches": [{"row": 5, "col": 2, "value": "..."}]}`
24. **`doctools-cli csv metadata <file>`**
    - Input: `file` (string)
    - Processing: `chardet` エンコーディング判定と行列数取得
    - Output: `{"encoding": "Shift_JIS", "rows": 100, "cols": 10}`
25. **`doctools-cli csv extract <file>`**
    - Input: `file` (string), `--output` (string)
    - Processing: 指定領域の別 CSV 保存
    - Output: `{"output_path": ".../subset.csv"}`
26. **`doctools-cli text head <file>`** / **`text tail <file>`**
    - Input: `file` (string), `--lines` (int)
    - Processing: ファイル先頭・末尾行取得
    - Output: `{"lines": ["..."]}`
27. **`doctools-cli text grep <file>`**
    - Input: `file` (string), `--pattern` (string)
    - Processing: 正規表現一致行の抽出
    - Output: `{"matches": [{"line": 10, "text": "..."}]}`
28. **`doctools-cli text metadata <file>`** / **`text convert <file>`**
    - Input: `file` (string), `--to-encoding` (string)
    - Processing: `chardet` 判定および相互変換
    - Output: `{"encoding": "UTF-8", "output_path": "..."}`
29. **`doctools-cli text copy-clipboard <text>`**
    - Input: `text` (string)
    - Processing: `go-ole` / Windows API クリップボード書き込み
    - Output: `{"message": "Copied"}`
30. **`doctools-cli html extract-text <file>`**
    - Input: `file` (string), `--output` (string)
    - Processing: HTML パースと Markdown 変換
    - Output: `{"output_path": ".../page.md"}`
31. **`doctools-cli image metadata <file>`**
    - Input: `file` (string)
    - Processing: 画像ヘッダ解析による幅・高さ取得
    - Output: `{"width": 1920, "height": 1080}`
32. **`doctools-cli image crop <file>`**
    - Input: `file` (string), `--bounds` (string)
    - Processing: 指定矩形領域のクロップ保存
    - Output: `{"output_path": ".../crop.png"}`
33. **`doctools-cli image save-clipboard`**
    - Input: `--output-dir` (string)
    - Processing: クリップボード画像取得と PNG 保存
    - Output: `{"output_path": ".../clip.png"}`
34. **`doctools-cli util zip <files...>`** / **`util unzip <file>`**
    - Input: `files` ([]string) / `file` (string)
    - Processing: `archive/zip` 圧縮および解凍
    - Output: `{"output_path": "..."}`

### データインターフェース
- **入力ファイル仕様**: Excel (.xlsx), PPTX (.pptx), PDF (.pdf), CSV, Text, HTML, Image (PNG/JPG)。標準で UTF-8 / Shift_JIS に対応し、`saintfish/chardet` による自動判別を実施。
- **出力ファイル仕様**: 抽出テキスト（Markdown: `.md`）、切り出し表データ（`.csv`）、画像（`.png`）、圧縮アーカイヴ（`.zip`）。原則として入力ファイルと同一フォルダに自動書き出し。

---

## コンポーネント

### ハイレベル・コンポーネント図

```mermaid
graph LR
    RootCmd["pkg/cli/root.go"] --> ExcelCmd["pkg/cli/excel.go"]
    RootCmd --> FTSCmd["pkg/cli/fts.go"]
    RootCmd --> PICmd["pkg/cli/pageindex.go"]
    RootCmd --> CSVCmd["pkg/cli/csv.go"]
    
    ExcelCmd --> ExcelSvc["pkg/excel/excel.go"]
    FTSCmd --> FTSSvc["pkg/fts/fts.go"]
    PICmd --> PISvc["pkg/pageindex/retrieval.go"]
    CSVCmd --> CSVSvc["pkg/csv/csv.go"]
```


### コンポーネント・パッケージ詳細 IPO テーブル

| パッケージ名 | 主要関数/メソッド | 入力 (Input) | 処理概要 (Processing) | 出力 (Output) |
| :--- | :--- | :--- | :--- | :--- |
| `pkg/excel` | `ExtractCSV(path, sheets)` | XLSXパス, シート名一覧 | `excelize` で各シート走査し CSV 書き出し | 生成 CSV パス一覧 |
| `pkg/excel` | `ExtractImages(path, dpi)` | XLSXパス, DPI | `go-ole` で FitToPagesWide 適用し PDF 経由 PNG 化 | 生成 PNG パス一覧 |
| `pkg/pptx` | `ExtractText(path)` | PPTXパス | Zip+XML 解析によりスライドテキスト抽出 | Markdown ファイルパス |
| `pkg/pptx` | `Merge(paths, outPath)` | PPTXパス一覧, 出力パス | SlideMaster を維持した XML 構造結合 | 結合 PPTX パス |
| `pkg/pptx` | `ExtractImages(path, slides)` | PPTXパス, スライド番号 | `go-ole` 経由で PowerPoint `slide.Export` 実行 | 生成 PNG パス一覧 |
| `pkg/pdf` | `ExtractText(path)` | PDFパス | `pdfcpu` によりテキスト抽出 | Markdown ファイルパス |
| `pkg/pdf` | `ExtractImages(path, outputDir, selectedPages)` | PDFパス, 出力フォルダ, ページ番号配列 | `pdfcpu` による埋め込み画像抽出 | 生成画像パス一覧 |
| `pkg/pdf` | `ExtractPages(inputPath, outputDir, dpi, format, startPage, endPage, force)` | PDFパス, 出力フォルダ, DPI, フォーマット, 開始/終了ページ, 上書きフラグ | `go-fitz` (MuPDF) による全画面ページ描画と保存 | レンダリング情報と生成画像パス一覧 |
| `pkg/fts` | `BuildIndex(dirPath, timeout, force)` | ディレクトリパス, タイムアウト時間, 上書きフラグ | 各形式パーサーで On-the-fly テキスト化・`mtime` 差分比較・`context.WithTimeout` 制御を行い Bleve 書き込み | 処理ファイル件数・スキップ件数・タイムアウト件数・チャンク件数・処理時間 |
| `pkg/fts` | `QueryIndex(query, limit)` | 検索文字列, 件数 | Bleve ハイライトスニペット生成および構造化 JSON (source/target) 出力 | ヒットスコア・ハイライトスニペット・AI位置ナビゲーション構造一覧 |
| `pkg/pageindex` | `BuildSummaryTree(dir)` | ディレクトリパス | `genai` SDK 経由での構造解析・要約 | `pageindex.json` パス |
| `pkg/csv` | `DetectEncoding(path)` | CSV/Textパス | `saintfish/chardet` による文字コード自動判定 | エンコーディング名 |
| `pkg/text` | `GrepFile(path, pattern)` | Textパス, 正規表現 | ストリーム行走査と正規表現マッチング | マッチ行番号・テキスト一覧 |
| `pkg/html` | `ExtractText(path)` | HTMLパス | HTML要素パースと Markdown テキスト変換 | Markdown ファイルパス |
| `pkg/image` | `CropImage(path, bounds)` | 画像パス, 矩形座標 | 画像のクロップ切り抜き保存 | 生成画像パス |
| `pkg/util` | `ZipFiles(paths, outPath)` | ファイルパス一覧 | `archive/zip` による圧縮 | 生成 ZIP パス |

---

## データモデル

### Bleve FTS インデックス構造 (`pkg/fts`)

チャンク単位 (`1シート / 1スライド / 1ページ / 1セクション`) でドキュメントをインデックス登録し、以下のフィールドマッピングで保持します：

```go
type DocumentChunk struct {
    ID          string    `json:"id"`           // 例: "path/to/doc.xlsx#Sheet1"
    FilePath    string    `json:"file_path"`    // 絶対パス (Store: true)
    FileName    string    `json:"file_name"`    // ファイル名 (Store: true)
    FileType    string    `json:"file_type"`    // "excel", "pptx", "pdf", "text", "csv" (Store: true)
    UpdatedAt   time.Time `json:"updated_at"`   // 最終更新日時 (Store: true)
    Content     string    `json:"content"`      // On-the-fly変換後のテキスト (Index: true, Store: true)
    UnitType    string    `json:"unit_type"`    // "sheet", "slide", "page", "section" (Store: true)
    UnitName    string    `json:"unit_name"`    // シート名や見出し名 (Store: true)
    PageOrIndex int       `json:"page_or_index"`// ページ番号やスライド番号 (1-indexed) (Store: true)
    Locator     string    `json:"locator"`      // "sheet=Q3_Target", "page=12" 等の識別子 (Store: true)
}
```

### 検索クエリレスポンス構造 (`doctools-cli fts query --json`)

```json
{
  "total_hits": 5,
  "hits": [
    {
      "score": 1.452,
      "snippet": "... 2026年度の <mark>売上目標</mark> に関する進捗率は以下の通り ...",
      "source": {
        "file_path": "C:/Users/satos/documents/sales_2026.xlsx",
        "file_name": "sales_2026.xlsx",
        "file_type": "excel",
        "updated_at": "2026-08-10T10:00:00Z"
      },
      "target": {
        "unit_type": "sheet",
        "unit_name": "Q3_Target",
        "page_or_index": 2,
        "locator": "sheet=Q3_Target"
      }
    }
  ]
}
```

### PageIndex JSON スキーマ (`pkg/pageindex`)
```json
{
  "doc_path": "string",
  "structure": [
    {
      "node_id": "string",
      "title": "string",
      "level": 0,
      "summary": "string",
      "pages": [1]
    }
  ]
}
```

### Excel パッチ JSON スキーマ & Go 構造体 (`pkg/excel`)

Excel パッチデータの入力形式は **整形式 JSON 配列 `[]ExcelPatchItem`** のみを受け入れます。

```go
type ExcelPatchItem struct {
    Sheet            string  `json:"sheet"`
    Cell             string  `json:"cell"`
    OldValue         *string `json:"old_value,omitempty"`
    ExpectedOldValue *string `json:"expected_old_value,omitempty"`
    NewValue         string  `json:"new_value"`
}
```

#### JSON スキーマサンプル (整形式配列)
```json
[
  {
    "sheet": "Sheet1",
    "cell": "A1",
    "old_value": "変更前文字列（厳格検証用）",
    "new_value": "変更後文字列"
  },
  {
    "sheet": "Sheet1",
    "cell": "B2",
    "old_value": null,
    "new_value": "無条件上書き文字列"
  }
]
```

---

## エラーハンドリング

すべてのコマンドは異常終了時に終了コード `1` 以上を返し、`stderr` に以下の標準エラー JSON を出力します：

```json
{
  "status": "error",
  "error_code": "FILE_NOT_FOUND",
  "message": "The specified file does not exist.",
  "hint": "Please verify the absolute file path."
}
```
