# テクノロジースタック

## 言語・ランタイム
- **Go** (1.25 以上)
  - **選定理由**: 静的型付け、超高速なコンパイルと起動速度、メモリ効率の良さ、および `CGO_ENABLED=1` による完全静的コンパイル（シングルバイナリ化）が可能なため採用。

## コアライブラリ
- **spf13/cobra**
  - **選定理由**: Go のデファクトスタンダード CLI フレームワーク。サブコマンド構築、フラグ管理、ヘルプ自動生成に優れる。
- **github.com/xuri/excelize/v2**
  - **選定理由**: 超高速かつメモリ効率の良い純 Go 製 Excel (.xlsx) ライブラリ。シート読み込み、CSV抽出を高速化。
- **pdfcpu/pdfcpu**
  - **選定理由**: 純 Go 製の軽量な PDF 操作ライブラリ。PDF の分割（Split）、マージ（Merge）、埋め込み画像抽出などの構造操作を CGO なしで高速・安全に処理するために採用。
- **MuPDF (fitz CGO Binding / gen2brain/go-fitz)**
  - **選定理由**: PDF ページをマルチモーダル LLM 向けに最高品質で PNG/JPG 画像化するラスタライザ、および **ToUnicode CMap 完全解決による高精度日本語テキスト抽出エンジン** として採用。PyMuPDF と同等のレイアウト解析・フォントマッピング能力を内蔵し、文字化けや描画オペレータ混入のない純粋な UTF-8 テキスト抽出と高速 FTS インデックス構築を実現。ビルド時に `.a` 静的ライブラリとして完全内蔵。
- **blevesearch/bleve**
  - **選定理由**: 純 Go 製の高性能全文検索エンジン。追加サーバー不要で、N-gram による日本語検索および高速インデックス構築に対応。
- **go-ole/go-ole**
  - **選定理由**: Windows 環境において Excel および PowerPoint を COM (Component Object Model) 経由で直接操作するために採用。Fit-to-Page PDF 化や高精度スライド画像化を実現。
- **saintfish/chardet**
  - **選定理由**: テキストファイルや CSV のエンコーディング判定を行う純 Go ライブラリ。
- **google.golang.org/genai**
  - **選定理由**: PageIndex インデックス構築時のサマリー生成において、Google Gemini API を利用するための公式 Go SDK。

## テストツール
- **testing** (Go 標準パッケージ): 単体テストおよび結合テストの基本フレームワーク。
- **net/http/httptest**: API および HTTP モックの検証。
- **go test -race -cover**: データ競合の検出およびステートメントカバレッジ 90% 以上の品質担保。

## アーキテクチャパターン
- **Agent-Native Dual Interface**:
  - **CLI モード**: サブプロセス起動によるオーバーヘッドゼロ・Unix パイプ・JSON 標準出力を活用する特注 CLI。
  - **stdio MCP サーバーモード**: エージェント環境の Tool Definition Block に常駐し、1 級市民として直接発火可能な Model Context Protocol (JSON-RPC 2.0 over stdio) サーバー。
- **統合ディスパッチャ (Unified Dispatcher / Gateway)**:
  - ツール乱立（Tool Overload）を抑止するため、単一の `doctools_run` エントリポイントから内部のアクション (`read`, `search`, `capture`) およびファイル形式ハンドラへディスパッチ。
- **Smart Dynamic Response (動的閾値ハンドリング)**:
  - 抽出結果の文字数・トークンサイズを計測し、閾値（デフォルト 4,000 文字 / 約 1,000 トークン、オプション指定可能）以下の場合はインライン Markdown 返却、超過時は自動でローカルファイル保存とハンドル返却に切り替えるハイブリッド機構。
- **Smart Dynamic Capture (動的画像キャプチャ制御)**:
  - ドキュメント（PDF/PPTX/Excel）の指定ページを PNG 画像化する際、単一ページ抽出（1枚）かつ `inline != false` の場合は MCP `ImageContent` (Base64) を直接注入してエージェントの視覚推論器に提供。複数ページ抽出（2枚以上）または `inline == false` の場合はファイル保存を行いハンドル一覧のみ返却してコンテキスト溢れを防止。また、`text` チャンクへの生 Base64 文字列の漏洩を型レベルで防止。
- **Single Executable Binary**: すべての依存関係（MuPDF C 静的ライブラリ含む）を単一実行ファイル `doctools-cli.exe` に内蔵。
- **3-Layer Introspection**: `--help` (L1), `doctools-cli agent-context` (L2), `SKILL.md` (L3) による多角的なエージェント誘導。
- **Handle-based Data Access**: 大容量出力はローカルファイルパス（ハンドル）として返却。
- **`internal/version` + ldflags バージョン注入**: バージョン文字列を `internal/version/version.go` に一元管理し、ビルド時に `-ldflags="-X 'doctools-cli/internal/version.Version=x.y.z'"` で注入。ハードコードを排除し、CI/CD フレンドリーなバージョン管理を実現。

## プラットフォーム固有の制約
- **Windows / Office 自動化**: PowerPoint スライド画像化および Excel Fit-to-Page 画像化機能は、Microsoft Office がインストールされた Windows 環境でのみ COM 操作 (`go-ole`) により動作。

## 実装上の重要なハック・最適化
- **Excel シート画像化の最適化 (Fit-to-Page Hack)**:
  `go-ole` 経由で Excel の `PageSetup.Zoom = false`, `PageSetup.FitToPagesWide = 1` を動的設定し、PDF へエクスポートすることで、表や図形の横崩れを防止。
- **MuPDF CGO 静的リンク**:
  `CGO_ENABLED=1` で MuPDF C ライブラリを静的コンパイルリンクし、外部 DLL 不要の単一バイナリを維持。
- **日本語クエリの自動フレーズクォート処理**:
  ひらがな・カタカナ・漢字を含む入力トークンを Unicode 判定し、Lucene 記法（`+`/`-`）を維持したまま自動的に `"..."` フレーズクエリへ変換。Bleve の N-gram トークナイザが文字種境界で OR 分割してしまう挙動を排除。
