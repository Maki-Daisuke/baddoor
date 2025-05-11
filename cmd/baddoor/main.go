package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// getShellCommand は、OSに応じて適切なシェルコマンドを返します
func getShellCommand() *exec.Cmd {
	// /etc/shellsから優先順位に基づいてシェルを選択
	shellPriority := []string{"zsh", "fish", "ksh", "tcsh", "bash", "sh"}
	availableShells := readAvailableShells()
	selectedShell := "/bin/sh" // デフォルト

	for _, shell := range shellPriority {
		for _, availShell := range availableShells {
			if strings.HasSuffix(availShell, "/"+shell) {
				selectedShell = availShell
				log.Printf("選択されたシェル: %s", selectedShell)
				return exec.Command(selectedShell, "-i")
			}
		}
	}
	cmd := exec.Command(selectedShell, "-i")
	return cmd
}

// readAvailableShells は /etc/shells からシェルのパスを読み込みます
func readAvailableShells() []string {
	shells := []string{}

	file, err := os.Open("/etc/shells")
	if err != nil {
		log.Printf("/etc/shellsの読み込みに失敗: %v", err)
		return shells
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// コメント行や空行をスキップ
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			shells = append(shells, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("/etc/shellsの読み込み中にエラー: %v", err)
	}

	return shells
}

// handleConnection は、クライアント接続ごとに呼び出される関数です
func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("接続を受け付けました: %s", conn.RemoteAddr())

	// シェルプロセスを起動
	cmd := getShellCommand()

	// PTYを作成して、シェルプロセスと接続
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("ptyの作成に失敗: %v", err)
		return
	}
	// 必ず閉じるようにする
	defer ptmx.Close()

	// クライアントの入力をPTYに送信
	go func() {
		io.Copy(ptmx, conn)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Printf("stdin pipeの作成に失敗: %v", err)
			return
		}
		stdin.Close()
	}()

	// シェルの出力をクライアントに送信
	io.Copy(conn, ptmx)

	// シェルプロセスの終了を待つ
	if err := cmd.Wait(); err != nil {
		log.Printf("シェルプロセスが終了しました: %v", err)
	}

	log.Printf("接続を終了しました: %s", conn.RemoteAddr())
}

func main() {
	// コマンドライン引数でポート番号を指定できるようにする
	port := flag.Int("p", 4444, "サーバーがListenするポート番号")
	flag.Parse()

	// 接続先のアドレスとポート
	listenAddr := fmt.Sprintf("0.0.0.0:%d", *port) // 指定されたポートでリッスン

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
