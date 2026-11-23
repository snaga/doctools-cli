# 要件定義 (Requirements)

## 概要
Gemini-CLI, Antigravity, Claude などの AI エージェントが、PDF, PowerPoint, Excel, CSV, HTML, テキスト, 画像などのローカルドキュメントを高速・決定論的に閲覧・検索・加工できるようにするための **Go言語製 Agent-Native CLI ツール (`doctools-cli.exe`)** を提供する。

## 前提条件
- 配布物は単一の実行可能バイナリ (`doctools-cli.exe`) であり、Python ランタイムや外部 DLL は不要であること。
- Windows / Linux / macOS 等のシェル環境から実行可能であること。
- Windows 環境において Office COM 操作（Excel / PowerPoint のスライド画像化等）を行う場合、Microsoft Office がインストールされていること。非 Windows 環境または Office 未導入環境で該当コマンドが実行された場合、システムは明確なエラーメッセージを出力して終了すること。

## 非機能要件・横断要件 (Agent-Native CLI 原則)
- **要件 NFR-01: 非対話型の徹底 (Non-interactive)**
  - ユーザーストーリー: AI エージェントとして、実行中に人間の入力待ちでハングしない CLI を利用したい。なぜなら無人または自律実行を円滑に行うためだ。
  - EARS 受け入れ基準:
    - ［Ubiquitous］ システムは、TTY 接続有無にかかわらず対話プロンプトを一切表示してはならない。
    - ［Event-driven］ 破壊的操作または上書き操作が要求されたとき、システムは `--force` フラグが指定されている場合のみ処理を実行しなければならない。

- **要件 NFR-02: 構造化出力 (Structured Output & Error Handling)**
  - ユーザーストーリー: AI エージェントとして、出力データを正確にパースし、エラー時には自己修正のヒントを得たい。なぜなら再試行の成功率を上げるためだ。
  - EARS 受け入れ基準:
    - ［Event-driven］ `--json` フラグが指定されたとき、システムは正常結果をパース可能な JSON 形式で `stdout` に出力しなければならない。
    - ［Event-driven］ エラーが発生したとき、システムはエラーメッセージおよび有効な選択肢 (enum) を `stderr` に出力し、非ゼロの終了コードを返さなければならない。

- **要件 NFR-03: 3-Layer Introspection (探索機能)**
  - ユーザーストーリー: AI エージェントとして、CLI の機能一覧や入力型定義を自律的に探索・理解したい。なぜなら事前にコマンド構文を学習・適合するためだ。
  - EARS 受け入れ基準:
    - ［Event-driven］ `doctools-cli agent-context` コマンドが実行されたとき、システムは全サブコマンド、引数型、および利用可能オプションの JSON 定義を出力しなければならない。

- **要件 NFR-04: テストカバレッジ率 (High Test Coverage)**
  - ユーザーストーリー: 開発者およびユーザーとして、すべてのパッケージが十分にテストされ高品質であることが保証されている状態を望む。なぜならリファクタリングや機能追加時のデグレを確実に防ぐためだ。
  - EARS 受け入れ基準:
    - ［Ubiquitous］ システムの主要ロジックパッケージ (`pkg/pptx`, `pkg/util`, `pkg/excel`, `pkg/fts`, `pkg/cli`, `pkg/pdf`, `pkg/csv`, `pkg/text`, `pkg/html`, `pkg/image`, `pkg/pageindex`, `pkg/mcp`) は、ステートメントカバレッジ 90% 以上を達成しなければならない。

---

## 機能要件

### 要件カテゴリ: Excel操作 (EXCEL)
- 要件 EXCEL-01: Excel シート一覧取得 (`doctools-cli excel list-sheets`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な XLSX ファイルパスが指定されたとき、システムは全シート名のリストを JSON 形式で `stdout` に出力しなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 存在しないファイルまたは破損したファイルが指定されたとき、システムはエラーを出力して非ゼロの終了コードで終了しなければならない。
- 要件 EXCEL-02: Excel シート CSV 抽出 (`doctools-cli excel extract-csv`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な XLSX ファイルと対象シート名が指定されたとき、各シートを CSV ファイルとして保存し、生成されたファイルパスの一覧を JSON 形式で返さなければならない。
- 要件 EXCEL-03: Excel シート Fit-to-Page PDF化 (`doctools-cli excel extract-images`)
  - EARS 受け入れ基準: ［Event-driven / State-driven］ Windows 環境かつ Excel がインストールされている状態で画像化/エクスポートが要求されたとき、システムは `go-ole` 経由で Fit-to-Page 印刷設定を行い、PDF ファイルパス一覧を返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ Office 未インストール環境または非 Windows 環境で実行されたとき、システムは未サポート環境エラーを `stderr` に出力して処理を中断しなければならない。
- 要件 EXCEL-05: Excel セル単位 Diff 抽出 (`doctools-cli excel diff`)
  - EARS 受け入れ基準: ［Event-driven］ 2つの有効な XLSX ファイルパスが指定されたとき、システムはセル単位の差分（値、数式、結合範囲 `merged_range` 情報）を抽出し、JSON 形式で `stdout` に出力しなければならない。
- 要件 EXCEL-06: Excel 安全ガードレール付き Patch 適用 (`doctools-cli excel patch`)
  - EARS 受け入れ基準: ［Event-driven］ 対象 XLSX ファイルとパッチ JSON ファイルが指定されたとき、システムは UTF-8 BOM を自動除去した上で整形式パッチ JSON 配列 `[ ... ]` を読み込まなければならない。
  - EARS 受け入れ基準: ［Event-driven］ `old_value`（または `expected_old_value`）が文字列指定されている場合、変更前セル値と一致した時のみ書き換えを実行し、不一致時はパッチ処理をアボートしなければならない。`null` 指定時は無条件で上書きを実行し、キー省略時は警告を出力して上書きを実行しなければならない。
  - EARS 受け入れ基準: ［Event-driven］ `--schema` フラグが指定されたとき、システムはパッチファイルのサンプル JSON テンプレートを `stdout` に出力しなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ JSON パースや検証エラーが発生したとき、システムはエラーメッセージとともに復旧用の具体的なヒントメッセージ（`hint`）を含めて出力しなければならない。
- 要件 EXCEL-07: Excel 座標付き Markdown 抽出 (`doctools-cli excel extract-markdown`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な XLSX ファイルが指定されたとき、システムは各シートのセル内容を Markdown テーブル形式で出力しなければならない。［Event-driven］ `--with-coords` フラグが指定されたとき、システムは各セル値の先頭にセル座標プレフィックス（例: `[C5]`）を付与して出力しなければならない。
- 要件 EXCEL-08: Excel セル検索・座標取得 (`doctools-cli excel search-cell`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な XLSX ファイルと検索クエリが指定されたとき、システムは一致するセル番地、シート名、セル値、および結合範囲情報を検索し、JSON 形式で `stdout` に出力しなければならない。
- 要件 EXCEL-09: Excel テキスト抽出コマンド名の統一エイリアス (`doctools-cli excel extract-text`)
  - EARS 受け入れ基準: ［Event-driven］ `doctools-cli excel extract-text` コマンドが実行されたとき、システムは `doctools-cli excel extract-markdown` と同等の挙動で Markdown 抽出処理を実行しなければならない。

### 要件カテゴリ: PowerPoint操作 (PPTX)
- 要件 PPTX-01: PowerPoint テキスト抽出 (`doctools-cli pptx extract-text`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な PPTX ファイルが指定されたとき、純 Go (Zip+XML) でスライドテキストを解析し、Markdown ファイルとして保存してパスを返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 破損した PPTX ファイルが指定されたとき、システムはパースエラーを出力し非ゼロで終了しなければならない。
- 要件 PPTX-02: PowerPoint 結合 (`doctools-cli pptx merge`)
  - EARS 受け入れ基準: ［Event-driven］ 複数の有効な PPTX ファイルパスと出力パスが指定されたとき、スライド構造を結合した新しい PPTX ファイルを生成し、パスを返さなければならない。
- 要件 PPTX-03: PowerPoint スライド画像化 (`doctools-cli pptx extract-images`)
  - EARS 受け入れ基準: ［Event-driven / State-driven］ Windows 環境かつ PowerPoint がインストールされている状態で画像化が要求されたとき、`go-ole` を用いて指定スライドを直接 PNG 画像としてエクスポートし、パス一覧を返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 存在しないスライド番号が指定されたとき、システムは有効範囲外エラーを出力しなければならない。

### 要件カテゴリ: PDF操作 (PDF)
- 要件 PDF-01: PDF テキスト抽出 (`doctools-cli pdf extract-text`)
  - ユーザーストーリー: AI エージェントおよびユーザーとして、日本語を含む PDF から文字化けや描画命令オペレータの混入なく、ToUnicode CMap を完全解決した高精度な UTF-8 テキストを抽出し、Markdown/テキストファイルとして取得したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ 有効な PDF ファイルが指定されたとき、システムは `MuPDF (fitz)` を用いて ToUnicode CMap を解決した高精度な UTF-8 テキスト（日本語・縦書き対応）をインメモリで抽出し、ページ区切り（`--- Page N ---`）を含むテキストとして返却、および指定パスへファイル保存しなければならない。
    - ［Event-driven］ `--start-page` および `--end-page` が指定されたとき、システムは指定範囲 (1-based) のページのみを対象としてテキスト抽出を行わなければならない。
    - ［Ubiquitous］ システムは、PDF の Contents ストリームにある生の描画オペレータ（PostScript コマンド）や未解決 CID バイト列を出力テキストに混入させてはならない。
    - ［Unwanted-behavior］ 暗号化（パスワード付き）PDF または `start-page > end-page` の不正な範囲が指定されたとき、システムはエラーメッセージを出力して処理を中断しなければならない。
- 要件 PDF-02: PDF 切り出し・結合 (`doctools-cli pdf split`, `doctools-cli pdf merge`)
  - EARS 受け入れ基準: ［Event-driven］ ページ範囲または複数 PDF パスが指定されたとき、`pdfcpu` を用いて新しい PDF ファイルを生成し、出力パスを返さなければならない。
- 要件 PDF-03: PDF 埋め込み画像抽出 (`doctools-cli pdf extract-images`)
  - EARS 受け入れ基準: ［Event-driven］ PDF ファイルが指定されたとき、PDF内部に埋め込まれている個々の画像オブジェクト (JPEG/PNG) を抽出し、保存された画像ファイルのパス一覧を返さなければならない。
- 要件 PDF-04: PDF ページ全画面レンダリング画像化 (`doctools-cli pdf extract-pages`)
  - ユーザーストーリー: AI エージェントおよびユーザーとして、PDF ページの全体（テキスト・図形含む見た目）を指定解像度 (DPI) で全画面画像化して取得したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ 有効な PDF ファイルパスが指定されたとき、システムは各ページを全画面ラスタライズし、指定解像度 (DPI, デフォルト 150) および画像フォーマット (`png` または `jpg`, デフォルト `png`) で出力ディレクトリへ保存しなければならない。
    - ［Event-driven］ `--start-page` および `--end-page` フラグが指定されたとき、システムは指定範囲 (1-based) のページのみを出力しなければならない。
    - ［Event-driven］ `--json` フラグが指定されたとき、システムは処理結果 (`total_pages`, `dpi`, `format`, `output_paths`) を含む構造化 JSON データを `stdout` に出力しなければならない。

### 要件カテゴリ: 全文検索 (FTS)
- 要件 FTS-01: On-the-fly 差分全文検索インデックス構築 (`doctools-cli fts build`)
  - ユーザーストーリー: AI エージェントとして、ローカルの指定フォルダ内にある各種ドキュメント（Officeバイナリ, PDF等）をOn-the-fly変換・チャンク化し、変更のあったファイルのみメタデータ付きで全文検索インデックスに登録したい。
  - EARS 受け入れ基準: ［Event-driven］ 対象ディレクトリが指定されたとき、システムは Excel (シート毎), PPT (スライド毎), PDF (ページ毎: `MuPDF (fitz)` による高精度インメモリテキスト抽出) 等を On-the-fly で変換およびチャンク分割し、ファイル共通情報（`file_path`, `file_name`, `file_type`）および形式別位置メタデータ（`unit_type`, `unit_name`, `page_or_index`, `locator`）を保存して `Bleve` インデックスを構築しなければならない。PDF からの抽出テキストに生のベクター描画オペレータ（`/Artifact BMC` 等）を含めてはならない。
  - EARS 受け入れ基準: ［Event-driven / State-driven］ 明示的に拡張子が指定されない限り、システムはデフォルトで Office/PDF バイナリ (`.xlsx`, `.pptx`, `.pdf`, `.docx`) のみをインデックス対象とし、Markdown, CSV, JSON などのテキストファイルは重複登録防止のためスキップしなければならない。
  - EARS 受け入れ基準: ［Event-driven / State-driven］ 既存インデックスが存在する場合、システムはファイルの最終更新日時 (`mtime`) を比較し、前回のインデックス登録時から更新されていないファイルの解析・登録処理をスキップしなければならない。
- 要件 FTS-02: コンテキスト・メタデータ付き全文検索クエリ (`doctools-cli fts query`)
  - ユーザーストーリー: AI エージェントとして、キーワード検索を行い、一致項目のスコア、一致前後のハイライトテキスト、および元のファイル・位置に直接アクセスするための構造化メタデータを取得したい。
  - EARS 受け入れ基準: ［Event-driven］ 検索クエリが渡されたとき、システムは `Bleve` インデックスを検索し、一致スコア、検索キーワードを含む前後ハイライトテキスト (`snippet`)、および AI 向け位置ナビゲーションデータ (`source` と `target`) を含む JSON を指定件数制限内で返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 不正なクエリ構文や存在しないインデックスパスが指定されたとき、システムはエラーメッセージを出力し非ゼロ終了コードで終了しなければならない。
- 要件 FTS-03: ファイル解析タイムアウト安全スキップ (`doctools-cli fts build --file-timeout`)
  - EARS 受け入れ基準: ［Event-driven］ 単一ファイルの解析・抽出処理が指定されたタイムアウト時間（デフォルト: 10秒）を超過した場合、システムは `context.WithTimeout` により即座に処理を中断し、警告を出力して該当ファイルをスキップした上で残りのファイルの処理を継続しなければならない。
- 要件 FTS-04: 柔軟な拡張子包含・除外フィルター (`doctools-cli fts build --include-ext / --exclude-ext`)
  - EARS 受け入れ基準: ［Event-driven］ `--include-ext` (または `-ext`) が指定されたとき、システムは指定拡張子のみを対象としてインデックス化しなければならない。
  - EARS 受け入れ基準: ［Event-driven］ `--exclude-ext` (または `--exclude`) が指定されたとき、システムは指定拡張子をインデックス対象から除外しなければならない。
- 要件 FTS-05: AIエージェント向け FTS クエリ自己説明 (Self-Describing) 強化 (`doctools-cli fts query` ヘルプ & Introspection 拡張)
  - EARS 受け入れ基準:
    - ［Event-driven］ `doctools-cli agent-context` または `doctools-cli fts query --help` が呼び出されたとき、システムは Command の説明文 (`Long`) に QueryString の構文ルール、具体例、および検索可能フィールド一覧 (`content`, `file_path`, `file_name`, `file_type`, `unit_type`, `unit_name`, `locator`, `updated_at`, `page_or_index`) を明記して出力しなければならない。
    - ［Ubiquitous］ システムは拡張子の指定において、ドットあり (`.md,.csv`) とドットなし (`md,csv`) のどちらの表記も同等に受け入れ、小文字に正規化して判定を行わなければならない。
- 要件 FTS-06: 隠しディレクトリのデフォルト除外 & ディレクトリ除外・包含フィルター (`doctools-cli fts build --exclude-dir / --include-dir`)
  - EARS 受け入れ基準:
    - ［Ubiquitous］ システムは、デフォルトで先頭がドット `.` で始まる隠しディレクトリ（探索のルートディレクトリ自身を除く）の探索を `filepath.SkipDir` によりスキップしなければならない。
    - ［Event-driven］ `--exclude-dir` フラグ（カンマ区切りまたは複数指定）が指定されたとき、システムは一致するディレクトリ名の配下走査をスキップしなければならない。
    - ［Event-driven］ `--include-dir` フラグ（カンマ区切りまたは複数指定）が指定されたとき、システムは指定されたディレクトリ配下のドキュメントのみをインデックス対象としなければならない。
    - ［Event-driven］ 隠しディレクトリ（`.` で始まるディレクトリ）であっても、`--include-dir` に明示的に指定されている場合は、システムはそのディレクトリの走査を許可しなければならない。

### 要件カテゴリ: 構造RAG (PAGEINDEX)
- 要件 PAGEINDEX-01: PageIndex 構造サマリーインデックス構築 (`doctools-cli pageindex build`)
  - ユーザーストーリー: AI エージェントとして、ドキュメントの階層構造サマリーインデックスを構築したい。
  - EARS 受け入れ基準: ［Event-driven］ 対象ディレクトリが指定されたとき、システムはドキュメントの構造メタデータを解析し、PageIndex 互換 JSON として保存しなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ LLM サマリー生成オプション使用時に `GEMINI_API_KEY` 環境変数が未設定の場合、システムは構造メタデータのみのフォールバック生成を行い、警告を出力しなければならない。
- 要件 PAGEINDEX-02: 目次・階層構造探索 (`doctools-cli pageindex tree`)
  - EARS 受け入れ基準: ［Event-driven］ ドキュメントパスおよびノード ID（任意）、深さ（depth）が指定されたとき、階層ツリー構造を JSON で返さなければならない。
- 要件 PAGEINDEX-03: ノードコンテンツピンポイント抽出 (`doctools-cli pageindex content`)
  - EARS 受け入れ基準: ［Event-driven］ ドキュメントパスおよびノード ID が指定されたとき、該当箇所のフルテキストを抽出して返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 存在しないノード ID が指定されたとき、システムは該当ノード不在エラーを返さなければならない。

### 要件カテゴリ: CSV操作 (CSV)
- 要件 CSV-01: CSV セル範囲読み込み (`doctools-cli csv read-cells`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な CSV ファイルパスおよび行・列範囲が指定されたとき、システムは該当範囲のセルデータを 2 次元配列 JSON として `stdout` に出力しなければならない。
- 要件 CSV-02: CSV 値検索 (`doctools-cli csv search`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な CSV ファイルパスおよび検索クエリが指定されたとき、システムは一致する行番号・列番号・セル値の一覧を JSON 形式で出力しなければならない。
- 要件 CSV-03: CSV メタデータ取得 (`doctools-cli csv metadata`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な CSV ファイルパスが指定されたとき、システムは文字コード判定（`chardet`）、総行数、最大列数を含むメタデータを JSON 形式で出力しなければならない。
- 要件 CSV-04: CSV 範囲抽出ファイル保存 (`doctools-cli csv extract`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な CSV ファイルパス、抽出範囲、および出力パスが指定されたとき、システムは指定領域を切り出した新しい CSV ファイルを生成しパスを返さなければならない。

### 要件カテゴリ: テキスト操作 (TEXT)
- 要件 TEXT-01: 先頭/末尾行閲覧 (`doctools-cli text head`, `doctools-cli text tail`)
  - EARS 受け入れ基準: ［Event-driven］ 有効なテキストファイルパスおよび行数（`--lines`）が指定されたとき、システムはファイルの先頭または末尾から指定行数を抽出し JSON 形式で出力しなければならない。
- 要件 TEXT-02: テキスト Grep 検索 (`doctools-cli text grep`)
  - EARS 受け入れ基準: ［Event-driven］ 有効なテキストファイルパスおよび正規表現パターンが指定されたとき、システムは一致した行番号とテキスト行の一覧を JSON 形式で出力しなければならない。
- 要件 TEXT-03: エンコーディング変換・判定 (`doctools-cli text metadata`, `doctools-cli text convert`)
  - EARS 受け入れ基準: ［Event-driven］ 有効なテキストファイルパスが指定されたとき、システムは文字コード判定結果を出力し、変換先エンコーディング（`--encoding`）が指定されたときは変換後ファイルを生成してパスを返さなければならない。
- 要件 TEXT-04: クリップボードテキストコピー (`doctools-cli text copy-clipboard`)
  - EARS 受け入れ基準: ［Event-driven］ 入力テキスト文字列が指定されたとき、システムは Windows クリップボードにテキストを設定し、完了ステータスを返さなければならない。

### 要件カテゴリ: HTML操作 (HTML)
- 要件 HTML-01: HTML テキスト抽出 (`doctools-cli html extract-text`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な HTML ファイルパスが指定されたとき、システムはタグを除去・構造化して Markdown テキストファイルを生成し、出力パスを返さなければならない。

### 要件カテゴリ: 画像操作 (IMAGE)
- 要件 IMAGE-01: 画像メタデータ取得 (`doctools-cli image metadata`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な画像ファイルパス（PNG/JPG）が指定されたとき、システムは画像サイズ（幅・高さ）およびフォーマット情報を JSON 形式で出力しなければならない。
- 要件 IMAGE-02: 画像切り抜き (`doctools-cli image crop`)
  - EARS 受け入れ基準: ［Event-driven］ 有効な画像ファイルパスおよび切り抜き座標（`--left`, `--top`, `--right`, `--bottom`）が指定されたとき、システムは指定領域をクロップした画像ファイルを生成しパスを返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 画像境界を超える座標や不正な矩形（left >= right 等）が指定されたとき、システムはエラーを出力しなければならない。
- 要件 IMAGE-03: クリップボード画像保存 (`doctools-cli image save-clipboard`)
  - EARS 受け入れ基準: ［Event-driven］ クリップボードに画像データが存在するとき、システムは PNG 画像ファイルとして指定パスに保存し、生成ファイルパスを返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ クリップボードに画像が存在しないとき、システムはデータ不在エラーを返さなければならない。

### 要件カテゴリ: ユーティリティ (UTIL)
- 要件 UTIL-01: ZIP 圧縮・解凍 (`doctools-cli util zip`, `doctools-cli util unzip`)
  - EARS 受け入れ基準: ［Event-driven］ 対象ファイル群または有効な ZIP ファイルパスが指定されたとき、システムは `archive/zip` による安全な圧縮または解凍を実行し、処理結果パスを返さなければならない。
  - EARS 受け入れ基準: ［Unwanted-behavior］ 解凍時に展開先ディレクトリの外へファイルを展開しようとするパストラバーサル（Zip Slip）を検知したとき、システムは処理を即座に中止し、セキュリティエラーを出力しなければならない。

### 要件カテゴリ: MCPサーバー (MCP)
- 要件 MCP-01: stdio MCP サーバー起動 (`doctools-cli mcp`)
  - ユーザーストーリー: AI エージェントとして、標準入出力 (stdio) 経由で JSON-RPC 2.0 メッセージをやり取りし、doctools の全機能を 1 級市民ツールとして利用したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ `doctools-cli mcp` コマンドが実行されたとき、システムは stdio 上で JSON-RPC 2.0 プロトコルをリッスンし、MCP クライアントからの初期化リクエスト (`initialize`) およびツール一覧要求 (`tools/list`) に応答しなければならない。
    - ［State-driven］ MCP サーバーの実行中、システムは `stdout` を JSON-RPC メッセージ専用とし、デバッグログや警告メッセージを `stdout` に混入させてはならない（`stderr` を使用）。
    - ［Unwanted-behavior］ 不正な JSON や未定義の JSON-RPC メソッドが送信されたとき、システムは仕様に準拠した JSON-RPC エラーオブジェクト（ParseError, MethodNotFound 等）を返却しなければならない。
- 要件 MCP-02: 統合ディスパッチゲートウェイ (`doctools_run`)
  - ユーザーストーリー: AI エージェントとして、ドキュメント形式ごとの複雑なコマンド構文やフラグを意識せず、単一のツール窓口から直感的に `read`, `search`, `capture` アクションを実行したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ `doctools_run` ツールが呼び出されたとき、システムは `file_path` の拡張子（`.pdf`, `.xlsx`, `.pptx`, `.csv`, `.html`, `.txt` 等。`.docx` は未対応）を自動判別し、対応する内部パーサーまたは検索・画像化エンジンへディスパッチしなければならない。
    - ［Unwanted-behavior］ 未対応の拡張子または存在しない `file_path` が指定されたとき、システムはエラーメッセージおよび利用可能なファイル形式一覧を返却しなければならない。
    - ［Ubiquitous］ `doctools_run` のツール定義（Description）は、エージェントに対し PDF や Office ファイルの閲覧・検索時に `read_file` や Python スクリプト自作を行わず、必ず本ツールを使用することを義務付ける強い指示（Negative Affordance）を含まなければならない。
- 要件 MCP-03: Smart Dynamic Response (動的閾値ハンドリング)
  - ユーザーストーリー: AI エージェントとして、小さなドキュメントは 1 ターンで素早く読み取り、長大なドキュメントではコンテキストウィンドウが溢れないように自動的にファイルとして保護されたい。
  - EARS 受け入れ基準:
    - ［Event-driven］ `command: "read"` の実行時、抽出された Markdown テキストの文字数が指定閾値（`max_inline_chars`、デフォルト: 4,000文字）以下の場合、システムはレスポンスの `content` に Markdown テキストを直接格納してインライン返却しなければならない。
    - ［Event-driven］ `command: "read"` の実行時、抽出された Markdown テキストの文字数が指定閾値を超過した場合、システムはテキストをローカル Markdown ファイル（`.tmp/` または指定出力先）に自動保存し、生成ファイルパス、総ページ数/シート数、文字数を含むメタデータおよびガイドメッセージを返却しなければならない。
    - ［Unwanted-behavior］ 不正なページ範囲やスライド範囲が指定されたとき、またはファイル書き込みに失敗したとき、システムはステータス error と詳細なエラーメッセージを返却しなければならない。
- 要件 MCP-04: 単一ファイル内ピンポイント検索 (`command: "search"`)
  - ユーザーストーリー: AI エージェントとして、特定の PDF や Excel、PPTX ファイルの内部から指定したキーワードを検索し、一致したページやシートの座標と前後スニペットを取得したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ `command: "search"` と有効な `file_path` および `query` が指定されたとき、システムは対象ファイル内を高速走査し、一致した位置情報（PDF: ページ番号、Excel: シート名およびセル番地、PPTX: スライド番号）と前後ハイライトスニペットの一覧を Markdown 形式テキストとして返却しなければならない。
    - ［Unwanted-behavior］ 検索クエリが空の場合、または対象ファイル内にクエリが 1 件も一致しなかったとき、システムは適切なエラーメッセージまたは 0 件ヒットである旨の明確な結果を返却しなければならない。
- 要件 MCP-05: Smart Dynamic Capture (`command: "capture"`)
  - ユーザーストーリー: AI エージェントとして、PDF や PowerPoint、Excel の特定ページやスライド、シートを高解像度画像としてキャプチャし、マルチモーダル視覚推論に活用したい。
  - EARS 受け入れ基準:
    - ［Event-driven］ `command: "capture"` と有効な `file_path`（PDF/PPTX/Excel）が指定されたとき、システムはプリンタ記法（例: `"1"`, `"2-4"`, `"2,5,8"`）に従って指定ページ群を高解像度画像（デフォルト: 150dpi PNG）としてラスタライズし、ローカルファイル（既定: `.tmp/{base}_page_{num}.png`）に保存しなければならない。
    - ［Event-driven］ キャプチャ結果が単一ページ（1枚）かつ `options.inline != false` のとき、システムはファイル保存パス情報に加えて MCP ImageContent（Base64 形式）をレスポンスに含めてインライン返却しなければならない。
    - ［Event-driven］ キャプチャ結果が複数ページ（2枚以上）のとき、システムはコンテキスト保護のため ImageContent を含めず、全生成画像ファイルパスの一覧およびメタデータ（解像度・ファイルサイズ）のみを返却しなければならない。
    - ［Unwanted-behavior］ テキスト・CSV・HTML などの画像化非対応形式が指定されたとき、システムはステータス error と画像化非対応である旨のエラーメッセージを返却しなければならない。
    - ［Unwanted-behavior］ PowerPoint または Excel のキャプチャ実行時に Windows Office 環境が利用できない場合、システムはステータス error と Office 未導入である旨のエラーメッセージを返却しなければならない。
    - ［Unwanted-behavior］ 指定されたページがすべて対象ドキュメントの範囲外の場合、システムはステータス error と有効なページ範囲を返却しなければならない（一部のみ範囲外の場合は存在するページのみ抽出しスキップ警告を添える）。

