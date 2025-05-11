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
	"runtime"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// getShellCommand は、OSに応じて適切なシェルコマンドを返します
func getShellCommand() *exec.Cmd {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShellでPSReadlineの問題があるため、cmdを使用
		cmd = exec.Command("cmd.exe")
	default: // Unix系OS（Linux、macOS）
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
		cmd = exec.Command(selectedShell, "-i")
	}
	return cmd
}

// readAvailableShells は /etc/shells からシェルのパスを読み込みます
func readAvailableShells() []string {
	shells := []string{}

	// Windowsの場合は早期リターン
	if runtime.GOOS == "windows" {
		return shells
	}

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

	// Windowsの場合はPTYを使用しない（Windowsではptyが完全にサポートされていない）
	if runtime.GOOS == "windows" {
		handleWindowsConnection(conn, cmd)
		return
	}

	// PTYを作成して、シェルプロセスと接続
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("ptyの作成に失敗: %v", err)
		return
	}
	// 必ず閉じるようにする
	defer ptmx.Close()

	// シェルの出力をクライアントに送信し、クライアントの入力をシェルに送信する
	var wg sync.WaitGroup
	wg.Add(2) // 2つのゴルーチンを待つ

	// PTYの出力をクライアントに送信
	go func() {
		defer wg.Done()
		io.Copy(conn, ptmx)
		log.Printf("pty出力の転送が終了しました: %s", conn.RemoteAddr())
	}()

	// クライアントの入力をPTYに送信
	go func() {
		defer wg.Done()
		io.Copy(ptmx, conn)
		log.Printf("入力の転送が終了しました: %s", conn.RemoteAddr())
	}()

	// ゴルーチンの終了を待つ
	wg.Wait()

	// シェルプロセスの終了を待つ
	if err := cmd.Wait(); err != nil {
		log.Printf("シェルプロセスが終了しました: %v", err)
	}

	log.Printf("接続を終了しました: %s", conn.RemoteAddr())
}

// handleWindowsConnection はWindows向けの接続処理を行います（PTYを使用しない）
func handleWindowsConnection(conn net.Conn, cmd *exec.Cmd) {
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
