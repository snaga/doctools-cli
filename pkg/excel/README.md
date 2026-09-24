# `pkg/excel` - Excel Processing Module

## 1. 責務 (Responsibility)
Excel ワークブック（`.xlsx`, `.xlsm`）からのシート一覧取得、CSV/Markdown抽出（セル座標付き対応）、セル差分検出（Diff）、事前検証付き安全パッチ適用（Patch）、および Windows 実機連携による高精度 Fit-to-Page 画像化の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **非破壊・安全パッチ原則**:
  - パッチ適用時、`old_value` が指定されている場合は対象セルの現在値と完全一致しない限り更新を中断（Abort）し、意図せぬ競合上書きを防ぐ。
  - パッチ適用前に必ず自動バックアップ（`.bak`）を作成する（`--backup` 指定時）。
- **結合セルと座標追跡**:
  - セル単位処理および差分比較では、結合セル範囲（`merged_range`）の親セル座標を正として扱う。
- **OS依存の局所化**:
  - シートの Fit-to-Page 画像化（`go-ole`）などの Windows 専用処理はビルドタグ `//go:build windows` で分離し、他プラットフォームでのビルド互換性を損なわないこと。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **qax-os/excelize の採用**:
  - *Why*: 大規模な XLSX 構造・数式・結合セルのパースにおいて、Pure Go で最も安定し標準仕様に準拠しているため。
- **Fit-to-Page 画像化における COM (go-ole) の採用**:
  - *Why*: Pure Go のレンダラでは複雑な罫線・フォント・印刷範囲の完全再現が困難なため、Windows 上の実機 Excel エンジンを直接駆動して最高精度の VLM（視覚言語モデル）向け画像を得る。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (EXCEL-01 〜 EXCEL-07)
- 設計: `doc/design.md` (EXCEL-F01 〜 EXCEL-F07)
