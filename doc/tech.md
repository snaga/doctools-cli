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
- **Windows GUI / ホットキー / トレイ常駐**: `docsearch-gui.exe` の Ctrl 連打検知（低レベルキーフック）、タスクトレイ常駐（`Shell_NotifyIconW`）、ウィンドウ最前面化（`SetForegroundWindow`）、および自己デタッチ起動（`windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS`）は Windows 専用機能（OS メッセージループ）。

## 実装上の重要なハック・最適化
- **Excel シート画像化の最適化 (Fit-to-Page Hack)**:
  `go-ole` 経由で Excel の `PageSetup.Zoom = false`, `PageSetup.FitToPagesWide = 1` を動的設定し、PDF へエクスポートしてから画像化することで、表や図形の横崩れを防止。
- **MuPDF CGO 静的リンク**:
  `CGO_ENABLED=1` で MuPDF C ライブラリを静的コンパイルリンクし、外部 DLL 不要の単一バイナリを維持。
- **低レベルキーボードフックによる Ctrl 連打検知**:
  `SetWindowsHookEx(WH_KEYBOARD_LL, ...)` を用い、300〜400ms 以内の連続した Ctrl キー Up/Down イベントを検知してブラウザ検索画面を直接起動。
- **Win32 Shell_NotifyIconW によるタスクトレイ常駐**:
  タスクバーにウィンドウを表示せず、通知領域にアイコンを登録。右クリック時に `CreatePopupMenu` / `TrackPopupMenu` で「検索画面を開く」「設定」「終了」メニューを提供し、Graceful Shutdown を実現。
- **SSE (Server-Sent Events) & Win32 前面化による同一タブ完全制御**:
  ブラウザ起動時、既存クライアントが接続済みの場合は OS 側のブラウザ起動（URL呼び出し）を完全に抑制し、SSE ストリーム経由で `focus` / `settings` イベントを通知。同時に `EnumWindows` + `SetForegroundWindow` でブラウザウィンドウを前面化することで、Firefox や Chrome 等での新規タブ乱立を 100% 防止。
- **日本語クエリの自動フレーズクォート処理**:
  ひらがな・カタカナ・漢字を含む入力トークンを Unicode 判定し、Lucene 記法（`+`/`-`）を維持したまま自動的に `"..."` フレーズクエリへ変換。Bleve の N-gram トークナイザが文字種境界で OR 分割してしまう挙動を排除。
- **自己デタッチ起動 & Job Object Breakaway**:
  `docsearch-gui.exe` 起動時、`--foreground` フラグが未指定の場合は自身を `CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS | CREATE_BREAKAWAY_FROM_JOB` でバックグラウンド子プロセスとしてフォークし、親プロセスはプロンプトを即時解放（約20ms）。親 Job Object による道連れ終了を遮断し、拒否環境では自動フォールバック。
- **Bleve IndexAlias によるマルチインデックス横断検索**:
  複数指定された Bleve インデックス群を `bleve.NewIndexAlias()` で束ね、1クエリで統合スコアリング＆インデックス別件数集計を同時実行。

