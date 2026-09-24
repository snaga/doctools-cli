# `pkg/pptx` - PowerPoint Processing Module

## 1. 責務 (Responsibility)
PowerPoint プレゼンテーション（`.pptx`）からのスライドテキスト抽出（Pure Go Zip+XML パース）、ファイル結合（`Merge`）、および Windows 実機 PowerPoint 連携（`go-ole`）によるスライド画像抽出（`extract-images`）の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **クロスプラットフォーム Pure Go と Windows COM の分離**:
  - テキスト抽出（`ExtractTextPureGo`）および結合（`Merge`）は Pure Go で実装し、全プラットフォームで高速動作可能とする。
  - COM 連携によるスライド画像化（`pptx_com.go`）は `//go:build windows` で分離し、非 Windows 環境向けに `pptx_com_stub.go` を提供する。
- **1-based スライド番号の統一**:
  - `startSlide`, `endSlide`, `slides` は 1-based（1始まり）として扱う。
- **COM オブジェクトの確実な解放**:
  - PowerPoint アプリケーションや Presentation オブジェクトは `defer oleutil.CallMethod(app, "Quit")` および `Release()` で確実に破棄し、ゴーストプロセスを残さない。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Pure Go Zip+XML によるテキスト抽出**:
  - *Why*: Office アプリケーションの起動オーバーヘッド（数秒）を完全に排除し、ミリ秒単位でスライド構造・テキストを抽出するため。
- **COM (go-ole) によるスライド画像化**:
  - *Why*: PowerPoint の複雑なオートシェイプ・SmartArt・レイアウトを正確に画像化するため、Windows 実機 PowerPoint エンジンを活用。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (PPTX-01 〜 PPTX-03)
- 設計: `doc/design.md` (PPTX-F01 〜 PPTX-F03)
