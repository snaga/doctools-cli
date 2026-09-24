# `pkg/image` - Image Processing Module

## 1. 責務 (Responsibility)
画像ファイル（PNG, JPEG, GIF）のメタデータ取得（幅・高さ・フォーマット）、矩形領域クロップ（`Crop`）、およびクリップボード画像のファイル保存機能の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **座標境界チェック (Bounds Protection)**:
  - クロップ座標 `(left, top, right, bottom)` が元画像の境界内に収まるようにクリッピング・バリデーションを行い、パニックや不正領域参照を防ぐ。
- **プラットフォーム固有処理の分離**:
  - Windows クリップボード操作などの OS 固有処理は適切に抽象化または条件分岐し、ヘッドレス／クロスプラットフォーム環境での安全性を確保する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Go 標準 `image` パッケージの採用**:
  - *Why*: OpenCV や ImageMagick などの重厚なネイティブライブラリをリンクせず、標準ライブラリのみで軽量かつ安全に画像メタデータ解析・基本加工を行うため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (IMAGE-01 〜 IMAGE-03)
- 設計: `doc/design.md` (IMAGE-F01 〜 IMAGE-F03)
