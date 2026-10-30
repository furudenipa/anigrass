# 開発・リリース手順

## 設定ファイルの役割

| ファイル | 役割 |
| --- | --- |
| `.github/workflows/ci.yml` | ブランチへの push・PR で整形、依存関係、vet、テスト、ビルド、配布物の試作を検証 |
| `.github/workflows/release.yml` | `v` と数字から始まるタグの push で CI を実行し、成功後に GitHub Releases へ公開 |
| `.github/dependabot.yml` | 毎週月曜 9:00（日本時間）に Go 依存関係・Actions の更新を確認し、種類ごとにまとめた PR を作成 |
| `.goreleaser.yaml` | 配布バイナリ・アーカイブ・チェックサム・リリースノートの生成設定 |

最低対応バージョンは Go 1.27 です。CI は Linux・macOS・Windows の最新安定版 Go（`stable`）でテストし、リリースも同じ指定でビルドします。
Actions はコミット SHA に固定しています。Dependabot の更新 PR は CI と変更内容を確認してマージしてください。自動マージは設定していません。
Go の `stable` と GoReleaser の `~> v2` は、それぞれ実行時点の安定版・v2 系を使用します。GoReleaser 本体の更新は Dependabot PR の対象外です。

## 初回の準備

1. GitHub に `furudenipa/anigrass` リポジトリを作成し、Settings → Actions → General で Actions が使用可能であることを確認します。
2. ローカルのリモートが未設定なら、次を実行します。

   ```sh
   git remote add origin https://github.com/furudenipa/anigrass.git
   ```

3. ファイルをコミットし、`git push -u origin main` で送信します。
4. GitHub の Actions タブで CI が成功したことを確認します。

公開処理には GitHub がジョブに発行する `GITHUB_TOKEN` を使います。個人用トークンや Annict トークンを Secrets に登録する必要はありません。公開ジョブだけに `contents: write` を付与しています。組織のポリシーで Actions やトークンの書き込みが制限されている場合は、その設定の確認が必要です。

## 普段の開発

```sh
gofmt -w main.go internal
go mod verify
go vet ./...
go test -race -coverprofile=coverage.out ./...
go build ./...
```

`go mod tidy` を実行した場合は `go.mod`・`go.sum` の変更も確認してください。
依存ライブラリを変更したら `THIRD_PARTY_NOTICES` のライセンスとバージョン表記も更新してください。

GoReleaser v2 をインストール済みなら、コミットが存在する状態で以下も実行できます。

```sh
goreleaser check
goreleaser release --snapshot --clean
```

試作の成果物は `dist/` に生成され、GitHub には公開されません。`--clean` は既存の `dist/` を削除してから生成します。CI でも同じ試作を実行しますが、試作ファイルのアップロード・保存はしていません。

## バージョンを公開する

公開したい変更をコミットして `main` に push し、CI の成功を確認してからタグを送信します。初回は例えば次のようにします。

```sh
git switch main
git status --short  # 未コミットの変更がないことを確認
git push origin main
git tag -a v0.1.0 -m 'Release v0.1.0'
git push origin v0.1.0
```

タグの push が公開操作です。ブランチの push だけではリリースは作成されません。
タグは対象コミットを指すため、未コミットの変更はリリースに入りません。既存の公開タグは付け直さず、変更時は新しいバージョンを作成してください。

Release ワークフローは CI 全体を再実行し、成功した場合だけ以下を公開します。

- macOS・Linux・Windows × amd64・arm64 の計 6 種類のバイナリ
- macOS・Linux は `.tar.gz`、Windows は `.zip`
- 各アーカイブに `README.md`・`LICENSE`・`THIRD_PARTY_NOTICES` を同梱
- アーカイブのチェックサム `checksums.txt` と Git 履歴に基づくリリースノート

`v0.1.0-rc.1` のようなバージョンはプレリリースになります。タグのトリガー条件は厳密なバージョン検証ではないため、`v0.1.0` のような SemVer 形式を使ってください。

公開後は Actions の Release が成功していること、Releases にアーカイブとチェックサムがあること、手元の OS 用アーカイブを展開して `anigrass --help` が実行できることを確認してください。
失敗時は Actions の失敗ジョブのログを確認します。修正コミットが必要なら新しいバージョンを使います。

## 公式リファレンス

- [GoReleaser と GitHub Actions](https://goreleaser.com/customization/ci/actions/)
- [GoReleaser のアーカイブ設定](https://goreleaser.com/customization/package/archives/)
- [Dependabot の設定](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)
