# `pkg/pdf` - PDF Processing & Rasterization Module

## 1. 責務 (Responsibility)
PDF ドキュメントからのテキスト抽出、ページ分割（`Split`）、ファイル結合（`Merge`）、埋め込み画像抽出（`ExtractImages`）、および MuPDF CGO バインディング（`go-fitz`）による高速・高精細な全画面スライド画像化（`ExtractPages`）の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **1-based ページ番号の統一**:
  - `startPage`, `endPage`, `selectedPages` はすべて 1-based（1始まり）として扱い、0 以下の場合は全ページ対象など適切に正規化する。
- **CGO リソース管理とクリーンアップ**:
  - `go-fitz`（MuPDF CGO ラスタライザ）でドキュメントを開いた際は、必ず `defer doc.Close()` を行いメモリリークを防ぐ。
- **ファイル上書きの制御**:
  - `ExtractPages` において、出力画像ファイルが既に存在する場合は `force == false` のときスキップ（またはエラー）とし、無駄な再レンダリングコストを回避する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **pdfcpu (Pure Go) と go-fitz (MuPDF CGO) のハイブリッド採用**:
  - *Why*: テキスト抽出・ページ分割・結合・埋め込み画像抽出は Pure Go の `pdfcpu` で軽量・高速に処理し、高負荷かつ精細さが求められるページ全体の画像レンダリングのみ CGO バインディングの `go-fitz` を活用することで、機能性とパフォーマンスの最高バランスを実現。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (PDF-01 〜 PDF-05)
- 設計: `doc/design.md` (PDF-F01 〜 PDF-F05)
