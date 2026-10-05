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

## アーキテクチャパターン
- **Agent-Native CLI**: MCP プロトコルに代わり、Unix パイプや JSON 標準出力を活用する特注 CLI。
- **Single Executable Binary**: すべての依存関係を単一の `doctools-cli.exe` に内蔵。
- **3-Layer Introspection**: `--help` (L1), `doctools-cli agent-context` (L2), `SKILL.md` (L3) による多角的なエージェント誘導。
- **Handle-based Data Access**: 大容量出力はローカルファイルパス（ハンドル）として返却。
- **`internal/version` + ldflags バージョン注入**: バージョン文字列を `internal/version/version.go` に一元管理し、ビルド時に `-ldflags="-X 'doctools/internal/version.Version=x.y.z'"` で注入。ハードコードを排除し、CI/CD フレンドリーなバージョン管理を実現。

## プラットフォーム固有の制約
- **Windows / Office 自動化**: PowerPoint スライド画像化および Excel Fit-to-Page 画像化機能は、Microsoft Office がインストールされた Windows 環境でのみ COM 操作 (`go-ole`) により動作。

## 実装上の重要なハック・最適化
- **Excel シート画像化の最適化 (Fit-to-Page Hack)**:
  `go-ole` 経由で Excel の `PageSetup.Zoom = false`, `PageSetup.FitToPagesWide = 1` を動的設定し、PDF へエクスポートしてから画像化することで、表や図形の横崩れを防止。
- **MuPDF CGO 静的リンク**:
  `CGO_ENABLED=1` で MuPDF C ライブラリを静的コンパイルリンクし、外部 DLL 不要の単一バイナリを維持。
