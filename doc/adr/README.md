# アーキテクチャ決定レコード (ADR)

Doctools CLI (`doctools-cli`) における主要なアーキテクチャ設計および技術選定の決定履歴です。

| ID | タイトル | ステータス | 策定日 | 概要 |
|:---|:---|:---|:---|:---|
| [0001](0001-adoption-of-go-agent-native-cli-architecture.md) | Go製 Agent-Native CLI アーキテクチャの採用およびシングルバイナリ配布 | Accepted | 2026-08-11 | ゼロ依存シングルバイナリ、3-Layer Introspection、`stdout`/`stderr`完全分離 |
| [0002](0002-on-the-fly-chunked-full-text-search-with-bleve.md) | Bleve による On-the-fly チャンク化全文検索と AI ナビゲーション | Accepted | 2026-08-11 | シート・スライド・ページ単位の On-the-fly 階層チャンク化と差分更新 |
| [0003](0003-excel-cell-level-diff-and-safe-patch-mechanism.md) | Excel セル単位 Diff 抽出および安全ガードレール付き Patch 適用機構 | Accepted | 2026-08-12 | 決定論的差分抽出、`old_value`事前検証、自動バックアップ、結合セル保護 |
| [0004](0004-pageindex-hierarchical-navigation-architecture.md) | PageIndex 階層ナビゲーション構造の導入 | Accepted | 2026-08-12 | 長大ドキュメントの章節ツリー探索とピンポイント抽出によるコンテキスト節約 |
| [0005](0005-multi-format-image-extraction-and-com-strategy.md) | マルチフォーマット画像抽出と Windows COM 連携戦略 | Accepted | 2026-08-12 | MuPDF CGO および Windows Office COM (Fit-to-Page) による視覚画像化 |
| [0006](0006-removal-of-docsearch-gui-and-agent-native-purification.md) | docsearch-gui の廃止と Agent-Native CLI への純化 | Accepted | 2026-11-22 | デスクトップGUI完全廃止、バイナリ分割解消、CLI単一バイナリ純化 |
| [0007](0007-adoption-of-mupdf-for-pdf-text-extraction.md) | MuPDF (go-fitz) による高精度 PDF テキスト抽出の採用 | Accepted | 2026-11-22 | ToUnicode CMap 完全解決による文字化け根絶、描画命令混入防止 |
| [0008](0008-mcp-server-and-unified-gateway.md) | Agent-Native MCP サーバー機能と統合ゲートウェイ (doctools_run) の導入 | Accepted | 2026-11-22 | stdio MCP サーバー内蔵、1級市民化、Dispatcher型統合ツール、Smart Dynamic Response |
| [0009](0009-smart-dynamic-capture-command.md) | Smart Dynamic Capture コマンドとマルチモーダル VLM 連携 | Accepted | 2026-11-23 | command: capture の追加、単一ページBase64注入/複数ページファイルハンドル動的切替、プリンタ記法 |

