# BadDoor - Backdoor server for LinkStation (LS700D series)

This is a really simple backdoor server for LinkStation. It is used to gain root shell access to the device.
It is not intended for production use. It is only for educational purposes.

This package is not supported by the author.Use at your own risk.

## 主な機能

- TCP接続を通じたリモートシェルアクセス
- 複数クライアントの同時接続処理
- `/etc/shells`から以下の優先順位でシェルを選択
    1. zsh
    2. fish
    3. ksh
    4. tcsh
    5. bash
    6. sh
    7. 上記が見つからない場合は `/bin/sh` をデフォルトとして使用

## 使用方法

### サーバー側

Dockerを使ってlinux/arm64向けにクロスビルドを行います：

```bash
# クロスビルド用の環境がインストールされていない場合、最初の1回だけこれが必要
docker run --privileged --rm tonistiigi/binfmt --install all

git clone https://github.com/Maki-Daisuke/baddoor.git
cd baddoor
docker buildx build --platform linux/arm64 -t baddoor_builder .  && \
docker create --name temp_container baddoor_builder              && \
docker cp temp_container:/app/baddoor.deb .                      ;  \
docker rm temp_container  &&  docker rmi baddoor_builder

# プログラムのインストール（systemdに登録される）
dpkg -i baddoor.deb

# プログラムの手動実行（デフォルトポート4444を使用）
./baddoor

# 特定のポート番号（例：8080）を指定して実行
./baddoor -p 8080
```

または、直接実行することもできます：

```bash
# デフォルトポート4444を使用
go run cmd/baddoor/main.go

# 特定のポート番号（例：8080）を指定
go run cmd/baddoor/main.go -p 8080
```

これにより、サーバーは指定されたポート（デフォルトは4444）でTCP接続の受付を開始します。

### クライアント側

クライアントプログラムをビルドします：

```bash
go build -o badclient cmd/badclient/main.go
```

サーバーに接続するには、badclientを使用します。パスワードは`-p`オプションで指定できます。

```bash
badclient -p <パスワード> <サーバーIP>:<ポート番号>
```

パスワードが指定されていない場合、プロンプトが表示されます。

接続が確立されると、リモートシェルセッションが開始され、コマンドを実行できるようになります。

## セキュリティ上の注意点

このプログラムは教育目的または信頼性の高いネットワーク内での使用のみを想定しています。以下のセキュリティ上の問題に注意してください：

2. **暗号化の欠如**: データは平文で送受信されるため、盗聴のリスクがあります
3. **権限の問題**: プログラムは実行ユーザーの権限でシェルを起動するため、適切な権限管理が必要です

## ライセンス

MIT License

## 免責事項

このソフトウェアは教育目的のみを意図して作成されています。作者は、このプログラムの悪用や不正使用によって生じた損害について一切の責任を負いません。使用者は、適用されるすべての法律と規制に従う責任があります。
