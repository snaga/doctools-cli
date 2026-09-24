# `pkg/html` - HTML Processing Module

## 1. 責務 (Responsibility)
HTML ファイルからスクリプト、スタイル、タグを除去し、見出し・段落・リンク構造を保持した Markdown / テキストの高速抽出を提供する。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **ノイズ除去の徹底**:
  - `<script>`, `<style>`, `<nav>`, `<header>`, `<footer>` などの不要なボイラープレートタグを適切に除去し、AI エージェントが読むべき本文テキストを高密度に抽出する。
- **文字コードの適切な処理**:
  - `pkg/csv` や `pkg/text` のエンコーディング検出と連携し、UTF-8 以外の HTML（Shift_JIS 等）も正常に解釈する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Pure Go による正規表現 & トークナイズ処理**:
  - *Why*: ブラウザエンジン（Chromium 等）の重厚な依存を排除し、ミリ秒単位の超高速テキスト抽出と軽量シングルバイナリを維持するため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (HTML-01)
- 設計: `doc/design.md` (HTML-F01)
