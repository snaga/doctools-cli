# プロジェクト構造

## ディレクトリ構成（Go / Agent-Native CLI）

```text
.
├── cmd/                        # CLI / GUI エントリーポイント
│   ├── doctools-cli/           # AIエージェント向け CLI バイナリ (main.go)
│   │   └── main.go
│   └── docsearch-gui/          # 人間向け デスクトップ全文検索GUIバイナリ (main.go)
│       └── main.go
├── internal/                   # モジュール外部から import 不可のプライベートパッケージ
│   └── version/                # バージョン情報 (ldflags -X で注入)
│       └── version.go
├── pkg/                        # 再利用可能な Go パッケージ
│   ├── cli/                    # Cobra コマンド定義 & agent-context
│   │   ├── root.go
│   │   ├── agent_context.go
│   │   ├── excel.go
│   │   ├── pptx.go
│   │   ├── pdf.go
│   │   ├── fts.go
│   │   ├── pageindex.go
│   │   ├── csv.go
│   │   ├── text.go
│   │   ├── html.go
│   │   ├── image.go
│   │   └── util.go
│   ├── docsearch/              # 人間向けデスクトップ検索GUIサービス
│   │   ├── hook.go             # Win32 低レベルキーフック (Ctrl連打検知)
│   │   ├── window.go           # Win32 ネイティブ小窓 (検索バー & インデックス選択)
│   │   ├── server.go           # ローカル検索 Web サーバー & REST API
│   │   ├── expansion.go        # Query Expansion サービス (LLM連携)
│   │   ├── history.go          # 検索履歴の永続化・サジェストマッチング
│   │   └── web/                # 埋め込み WebUI アセット (HTML/CSS/JS)
│   ├── excel/                  # Excel 操作サービス (excelize + go-ole)
│   ├── pptx/                   # PPTX 操作サービス (zip+xml + go-ole)
│   ├── pdf/                    # PDF 操作サービス (pdfcpu + MuPDF fitz)
│   ├── fts/                    # 全文検索サービス (bleve N-gram・On-the-fly チャンク化)
│   ├── pageindex/              # PageIndex サービス (genai SDK)
│   ├── csv/                    # CSV 操作サービス (ストリーム処理)
│   ├── text/                   # テキスト操作・Grep・文字コード判定 (chardet)
│   ├── html/                   # HTML テキスト抽出サービス
│   ├── image/                  # 画像メタデータ・クロップ・クリップボード
│   └── util/                   # 共通ユーティリティ (ZIP圧縮/解凍・レスポンス成形)
├── doc/                        # 🌍 公開用ドキュメント・仕様書マスター
│   ├── adr/                    # アーキテクチャ決定レコード (ADR 0001〜0005)
│   ├── product.md              # 製品概要・憲法
│   ├── tech.md                 # 技術スタック仕様
│   ├── structure.md            # プロジェクト構造（このファイル）
│   ├── requirements.md         # 要件定義
│   └── design.md               # 詳細設計書
├── dist/                       # 配布パッケージ成果物 (.gitignore 対象)
│   └── doctools-cli-{ver}-{os}-{arch}/
│       ├── doctools-cli.exe    # Agent-Native CLI バイナリ
│       └── docsearch-gui.exe   # 人間向けデスクトップ検索GUIバイナリ
├── go.mod                      # Go モジュール定義
├── go.sum                      # Go 依存関係チェックサム
├── LICENSE                     # ライセンスファイル (Apache-2.0)
└── README.md                   # プロジェクト説明・エージェント指示プロンプト
```



## 命名規則
- ディレクトリ / パッケージ名: 小文字単語・簡潔（例: `excel`, `pptx`, `pdf`）
- コマンド / サブコマンド名: ケバブケース (例: `extract-images`, `list-sheets`)
- オプション / フラグ名: ケバブケース (例: `--output-dir`, `--json`)
