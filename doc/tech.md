# テクノロジースタック

## 言語・ランタイム
- **Go** (1.22 以上)
  - **選定理由**: 静的型付け、超高速なコンパイルと起動速度、メモリ効率の良さ、および `CGO_ENABLED=1` による完全静的コンパイル（シングルバイナリ化）が可能なため採用。

## コアライブラリ
- **spf13/cobra**
  - **選定理由**: Go のデファクトスタンダード CLI フレームワーク。サブコマンド構築、フラグ管理、ヘルプ自動生成に優れる。
- **qax-os/excelize**
  - **選定理由**: 超高速かつメモリ効率の良い純 Go 製 Excel (.xlsx) ライブラリ。シート読み込み、CSV抽出を高速化。
- **pdfcpu/pdfcpu**
  - **選定理由**: 純 Go 製の軽量な PDF 操作ライブラリ。PDF の分割、マージ、テキスト抽出を CGO なしで高速に処理。
- **MuPDF (fitz CGO Binding)**
  - **選定理由**: PDF ページをマルチモーダル LLM 向けに最高品質で PNG/JPG 画像化するためのラスタライザ。ビルド時に `.a` 静的ライブラリとして内蔵。
- **blevesearch/bleve**
  - **選定理由**: 純 Go 製の高性能全文検索エンジン。追加サーバー不要で、N-gram による日本語検索および高速インデックス構築に対応。
- **go-ole/go-ole**
  - **選定理由**: Windows 環境において Excel および PowerPoint を COM (Component Object Model) 経由で直接操作するために採用。Fit-to-Page PDF 化や高精度スライド画像化を実現。
- **saintfish/chardet**
  - **選定理由**: テキストファイルや CSV のエンコーディング判定を行う純 Go ライブラリ。
- **google.golang.org/genai**
  - **選定理由**: PageIndex インデックス構築時のサマリー生成および DocSearch GUI での Query Expansion（クエリ同義語・表記揺れ展開）において、Google Gemini API を利用するための公式 Go SDK。
- **golang.org/x/sys/windows**
  - **選定理由**: Windows 低レベルキーボードフック (`WH_KEYBOARD_LL`) による Ctrl 連打検知、および枠なし検索バー小窓（Win32 `CreateWindowEx`）を CGO なしでネイティブ実現するために採用。

## アーキテクチャパターン
- **Agent-Native CLI**: MCP プロトコルに代わり、Unix パイプや JSON 標準出力を活用する特注 CLI。
- **Multi-Binary Workspace (CLI / GUI 分離)**: AI エージェントのコンテキスト・Introspection を汚染しないよう、Agent-Native CLI (`doctools-cli.exe`) と人間向けデスクトップ検索ランチャー (`docsearch-gui.exe`) を同一リポジトリ内の独立バイナリとして分離。ドメインロジック (`pkg/fts` 等) は共通利用。
- **Single Executable Binary**: すべての依存関係（WebUI アセット含む）を Go 1.16+ `embed` で単一実行ファイルに内蔵。
- **3-Layer Introspection**: `--help` (L1), `doctools-cli agent-context` (L2), `SKILL.md` (L3) による多角的なエージェント誘導。
- **Handle-based Data Access**: 大容量出力はローカルファイルパス（ハンドル）として返却。
- **`internal/version` + ldflags バージョン注入**: バージョン文字列を `internal/version/version.go` に一元管理し、ビルド時に `-ldflags="-X 'doctools/internal/version.Version=x.y.z'"` で注入。ハードコードを排除し、CI/CD フレンドリーなバージョン管理を実現。

## プラットフォーム固有の制約
- **Windows / Office 自動化**: PowerPoint スライド画像化および Excel Fit-to-Page 画像化機能は、Microsoft Office がインストールされた Windows 環境でのみ COM 操作 (`go-ole`) により動作。
- **Windows GUI / ホットキー**: `docsearch-gui.exe` の Ctrl 連打検知および Win32 ネイティブ小窓は Windows 専用機能（OS メッセージループ）。

## 実装上の重要なハック・最適化
- **Excel シート画像化の最適化 (Fit-to-Page Hack)**:
  `go-ole` 経由で Excel の `PageSetup.Zoom = false`, `PageSetup.FitToPagesWide = 1` を動的設定し、PDF へエクスポートしてから画像化することで、表や図形の横崩れを防止。
- **MuPDF CGO 静的リンク**:
  `CGO_ENABLED=1` で MuPDF C ライブラリを静的コンパイルリンクし、外部 DLL 不要の単一バイナリを維持。
- **低レベルキーボードフックによる Ctrl 連打検知**:
  `SetWindowsHookEx(WH_KEYBOARD_LL, ...)` を用い、300〜400ms 以内の連続した Ctrl キー Up/Down イベントを検知して検索バーを即座にフォアグラウンド表示。
- **Bleve IndexAlias によるマルチインデックス横断検索**:
  複数指定された Bleve インデックス群を `bleve.NewIndexAlias()` で束ね、1クエリで統合スコアリング＆インデックス別件数集計を同時実行。

