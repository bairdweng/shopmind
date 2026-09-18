package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"shopmind/internal/config"
	"shopmind/internal/server"
	"shopmind/internal/vault"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`shopmind %s - 电商工作台骨架

用法:
  shopmind serve [--port 8788] [--open] [--static web/dist]
  shopmind version

环境变量:
  SHOPMIND_ROOT  项目根目录（默认当前目录）

`, version)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.String("port", config.DefaultPort, "HTTP 端口")
	open := fs.Bool("open", false, "启动后打开浏览器")
	static := fs.String("static", "", "静态资源目录，默认 web/dist")
	_ = fs.Parse(args)

	if *static == "" {
		*static = filepath.Join(config.Root(), "web", "dist")
	}

	store := vault.New()
	if err := store.UnlockFromLocalKey(); err != nil && !errors.Is(err, vault.ErrNoLocalKey) && !errors.Is(err, vault.ErrNotInitialized) {
		log.Printf("本机密钥未能解锁: %v", err)
	}

	addr := ":" + *port
	srv := server.New(*static, store)
	log.Printf("shopmind serve on http://127.0.0.1%s", addr)
	log.Printf("局域网: http://%s%s", localIP(), addr)
	log.Printf("data dir: %s", config.DataDir())

	if *open {
		go openBrowser("http://127.0.0.1:" + *port)
	}

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Run()
}

func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1"
}
