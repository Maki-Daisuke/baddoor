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
	"github.com/msteinert/pam"
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

	// クライアントからパスワードを読み込む
	fmt.Fprint(conn, "Input admin password:")
	reader := bufio.NewReader(conn)
	password, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("パスワードの読み込みに失敗: %v", err)
		return
	}
	password = strings.TrimSpace(password)

	// PAM認証
	err = pamAuthenticate("admin", password)
	if err != nil {
		fmt.Sprintln(conn, "Authentication failed.")
		return
	}

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

// pamAuthenticate は、PAMを使用してユーザーを認証します
func pamAuthenticate(username, password string) error {
	transaction, err := pam.StartFunc("login", username, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return password, nil
		case pam.ErrorMsg, pam.TextInfo:
			fmt.Println(msg)
			return "", nil
		}
		panic("unrecognized PAM message style")
	})

	if err != nil {
		return fmt.Errorf("PAM Start failed: %w", err)
	}
	if err = transaction.Authenticate(0); err != nil {
		return fmt.Errorf("Authentication failed: %w", err)
	}
	return nil
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
