# `pkg/docx` - Word (.docx) Processing Module

## 1. 責務 (Responsibility)
Word (`.docx` / Office Open XML) アーカイブからの純 Go によるテキスト・見出し階層・表（Markdown テーブル）の高速・構造化抽出の提供。
ドキュメント全体の Markdown 抽出（`ExtractText`）および FTS / ピンポイント検索向けの論理セクション抽出（`ExtractSections`）を担当する。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **純 Go 実装の維持 (Pure Go Only)**:
  - Go 標準ライブラリ（`archive/zip`, `encoding/xml`, `strings`, `fmt`, `os` 等）および共通型 `pkg/models` のみを使用する。
  - 外部サードパーティライブラリ、CGO、Windows COM (`go-ole`) への依存は禁止とする。
- **ファイルスコープの限定**:
  - 対象形式を OOXML (`.docx`) に限定し、レガシーバイナリ形式（`.doc`）は対象外とする。
- **安全なアーカイブ展開**:
  - `archive/zip` 読み込み時は Zip Slip や過大サイズ展開のリスクを考慮した安全な読み出しを行う。
- **決定論的な Markdown 構造化**:
  - 見出しスタイル（`Heading 1`〜`Heading 6`, `outlineLvl`）を Markdown 見出し記法（`#`, `##` 等）に変換する。
  - 表（`<w:tbl>`）を行・列セルに分解し、標準 Markdown テーブル記法（`| ... |`）へ整形する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Pure Go Zip+XML によるテキスト抽出**:
  - *Why*: Office アプリケーション（Word）の起動オーバーヘッドや外部ツールのインストール要求を完全に排除し、シングルバイナリ原則と高い移植性を 100% 維持したままミリ秒単位の高速パースを実現するため。
- **キャプチャ機能の非提供**:
  - *Why*: Word ドキュメントはリフロー型であり、固定スライドや物理ページを持つ PPTX/PDF と異なり純 Go での確実なページ画像レンダリングが困難であるため、テキスト・セクション抽出に特化。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (DOCX-01, DOCX-02)
- 設計: `doc/design.md` (DOCX-F01, FTS-F07, IPO: `ExtractText`, `ExtractSections`)
- ADR: `doc/adr/0011-pure-go-docx-support.md`
