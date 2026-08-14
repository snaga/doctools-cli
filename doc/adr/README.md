# アーキテクチャ決定レコード (ADR)

Doctools CLI (`doctools-cli`) における主要なアーキテクチャ設計および技術選定の決定履歴です。

| ID | タイトル | ステータス | 策定日 | 概要 |
|:---|:---|:---|:---|:---|
| [0001](0001-adoption-of-go-agent-native-cli-architecture.md) | Go製 Agent-Native CLI アーキテクチャの採用およびシングルバイナリ配布 | Accepted | 2026-08-11 | ゼロ依存シングルバイナリ、3-Layer Introspection、`stdout`/`stderr`完全分離 |
| [0002](0002-on-the-fly-chunked-full-text-search-with-bleve.md) | Bleve による On-the-fly チャンク化全文検索と AI ナビゲーション | Accepted | 2026-08-11 | シート・スライド・ページ単位の On-the-fly 階層チャンク化と差分更新 |
| [0003](0003-excel-cell-level-diff-and-safe-patch-mechanism.md) | Excel セル単位 Diff 抽出および安全ガードレール付き Patch 適用機構 | Accepted | 2026-08-12 | 決定論的差分抽出、`old_value`事前検証、自動バックアップ、結合セル保護 |
| [0004](0004-pageindex-hierarchical-navigation-architecture.md) | PageIndex 階層ナビゲーション構造の導入 | Accepted | 2026-08-12 | 長大ドキュメントの章節ツリー探索とピンポイント抽出によるコンテキスト節約 |
| [0005](0005-multi-format-image-extraction-and-com-strategy.md) | マルチフォーマット画像抽出と Windows COM 連携戦略 | Accepted | 2026-08-12 | MuPDF CGO および Windows Office COM (Fit-to-Page) による視覚画像化 |

