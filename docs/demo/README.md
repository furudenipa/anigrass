# デモ GIF の撮影

[VHS](https://github.com/charmbracelet/vhs)、ttyd、ffmpeg をインストールします。

リポジトリのルートで、次を実行します。アクセストークンは不要です。

```sh
bash docs/demo/record.sh
```

`docs/demo/anigrass.gif` に、オプションなしの `anigrass` を実行する
デモが出力されます。

撮影専用の `fixture/main.go` は、本体の CLI 処理に架空の API 応答を
渡します。過去365日分の視聴記録を固定シードで生成し、日常的な視聴、
週末の一気見、年に1回の視聴しない週を表現します。Annict への通信は発生しません。
撮影用バイナリは一時ディレクトリに作成し、終了時に削除します。
