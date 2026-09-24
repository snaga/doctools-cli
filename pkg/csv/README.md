# `pkg/csv` - CSV Processing Module

## 1. 責務 (Responsibility)
CSV ファイルの自動文字コード判定（`chardet`）、メタデータ抽出（行数・最大列数）、特定行・列の範囲抽出（`ReadCells` / `Extract`）、およびセル内全文キーワード検索の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **文字コードの自動判定とフォールバック**:
  - 日本語環境特有の Shift_JIS / Windows-31J / EUC-JP / UTF-8 を透過的に判別し、パース時に自動変換する。
- **1-based インデックスの統一**:
  - 行番号および列番号は CLI / API レベルで 1-based（1始まり）として扱い、内部で適切に 0-based スライスと変換する。
- **部分抽出によるメモリ効率化**:
  - 巨大な CSV ファイル全体をメモリにロードせず、ストリームまたは指定範囲（`startRow`〜`endRow`）のみを効率的に処理する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **saintfish/chardet & 標準 encoding/csv の採用**:
  - *Why*: ICU 由来の高精度な文字コード検出ライブラリと Go 標準の高速な CSV パーサーを組み合わせ、外部 C ライブラリ非依存でクロスプラットフォーム動作を実現するため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (CSV-01 〜 CSV-04)
- 設計: `doc/design.md` (CSV-F01 〜 CSV-F04)
