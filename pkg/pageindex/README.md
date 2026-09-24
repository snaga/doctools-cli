# `pkg/pageindex` - Hierarchical Structural RAG Module

## 1. 責務 (Responsibility)
大規模ドキュメントの構造化ツリー（`*.pageindex.json`）の読み込み・段階的探索（`tree` による深さ／ノード指定フィルタリング）および、特定ノード／ページ／シートのピンポイントフルテキスト抽出（`content`）の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **トークン消費の極小化 (Progressive Disclosure)**:
  - 巨大な目次・構造ツリー全体を返却せず、指定された `node_id` および探索深さ（`depth`）でフィルタリングした部分木のみを返却する。
- **コンテンツ抽出の抽象化**:
  - `pkg/pdf`, `pkg/pptx`, `pkg/excel` の各パーサーを協調させ、`node_id` やページ範囲・シート名に応じた適切な抽出エンジンを透過的に呼び出す。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **JSON ベースの階層構造モデル**:
  - *Why*: LLM が最も理解しやすい階層的ツリー表現（PageIndex）を採用し、ベクター検索では失われがちな文書全体のコンテキストと位置関係を決定論的に保持するため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (PAGEINDEX-01 〜 PAGEINDEX-03)
- 設計: `doc/design.md` (PAGEINDEX-F01 〜 PAGEINDEX-F03)
