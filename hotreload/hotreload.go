// Package hotreload 提供开发期热更新：文件改动时自动重建并重启当前服务。
// 在服务 main.go 中调用 hotreload.WatchAndRun 即可，无需外部工具（air 等）。
//
// 是否开启热更新仅看运行方式，编译出的二进制默认视为生产、直接运行：
//   - 开发：以 `-dev` 参数运行，或二进制为 `go run` 的临时产物；当前进程成为监督者，
//     spawn 子进程（<root>/tmp/dev-main）跑服务，监听改动后重编译并重启；
//   - 生产：编译后的二进制直接运行，不热更新；
//   - 子进程（<root>/tmp/dev-main）直接 run，不再次监督。
package hotreload

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// devMainBin 监督者 spawn 子进程时固定编译/运行的输出二进制名，据此识别子进程防递归。
const devMainBin = "dev-main"

// WatchAndRun 开发环境以监督者身份运行 run；生产直接运行 run。
// root 为服务目录（go build 与子进程的工作目录）。
func WatchAndRun(root string, run func()) {
	// 子进程：不再次监督，直接运行。
	if isDevChild() {
		run()
		return
	}
	// 生产（编译出的二进制直接运行）：不热更新。
	if !isDev() {
		run()
		return
	}

	root, _ = filepath.Abs(root)
	bin := filepath.Join(root, "tmp", devMainBin)

	// 首次编译失败直接退出，避免进入一个无法服务的状态。
	if err := build(root, bin); err != nil {
		fmt.Println("[dev] 首次编译失败:", err)
		os.Exit(1)
	}

	var child *exec.Cmd
	spawn := func() { child = runChild(root, bin) }
	kill := func() {
		if child != nil && child.Process != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
		child = nil
	}
	restart := func() {
		if err := build(root, bin); err != nil { // 编译失败保留当前进程，不中断服务
			fmt.Println("[dev] 编译失败，保留当前进程:", err)
			return
		}
		kill()
		spawn()
	}

	spawn()
	fmt.Printf("[dev] 热更新运行中，监听 %s，改动 .go/.yaml/.yml/.toml 自动重建重启。\n", root)

	// 建立 fsnotify 监听，并在有改动时触发一次去抖后的重启。
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("[dev] fsnotify 初始化失败:", err)
		os.Exit(1)
	}
	defer fw.Close()
	watchTree(fw, root)

	trigger := make(chan struct{}, 1)
	go debounceLoop(trigger, restart)

	// 主循环：转发文件改动到 trigger；收到退出信号则关闭子进程后返回。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case ev := <-fw.Events:
			// 只关心写入/创建/删除/重命名，忽略 CHMOD 等元数据变更
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 && shouldReload(ev.Name) {
				select {
				case trigger <- struct{}{}:
				default: // 已有待处理信号，跳过避免堆积
				}
			}
		case <-sig:
			kill()
			return
		}
	}
}

// isDevChild 判断当前二进制是否为监督者 spawn 的子进程
// （固定输出到 <root>/tmp/dev-main）。子进程复用同一入口，需跳过监督防止递归。
func isDevChild() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return filepath.Base(exe) == devMainBin && filepath.Base(filepath.Dir(exe)) == "tmp"
}

// isDev 判断是否开发运行：
//   - 显式带 `-dev` 参数（dev.sh 以编译二进制启动开发时使用）；
//   - 或二进制为 `go run` 的临时产物。
//
// 编译出的二进制（无论从哪个目录运行）都视为生产，直接运行、不热更新。
func isDev() bool {
	for _, a := range os.Args[1:] {
		if a == "-dev" {
			return true
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(filepath.Dir(exe), "go-build")
}

// build 在服务目录内编译二进制；失败时返回编译输出以便排查。
func build(root, bin string) error {
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

// runChild 以子进程方式启动服务（运行 tmp/dev-main，进入后会因 isDevChild 跳过监督），
// 并后台回收防僵尸。
func runChild(root, bin string) *exec.Cmd {
	cmd := exec.Command(bin)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Println("[dev] 启动子进程失败:", err)
		return nil
	}
	go func() { _ = cmd.Wait() }()
	return cmd
}

// debounceLoop 消费 trigger 信号：先去抖再统一处理，避免连续保存触发多次重启。
// 重启后有冷却期，防止 build 产物事件引发重启循环。
func debounceLoop(trigger <-chan struct{}, restart func()) {
	var lastRestart time.Time
	const cooldown = 3 * time.Second
	for range trigger {
		time.Sleep(500 * time.Millisecond)
	drain:
		for {
			select {
			case <-trigger:
			default:
				break drain
			}
		}
		if time.Since(lastRestart) < cooldown {
			fmt.Println("[dev] 冷却期内，跳过本次重启")
			continue
		}
		fmt.Println("[dev] 检测到改动，重启…")
		restart()
		lastRestart = time.Now()
	}
}

// watchTree 递归遍历目录，为每个非排除目录建立 fsnotify 监听
// （fsnotify 不会自动跟随子目录）。
func watchTree(fw *fsnotify.Watcher, dir string) {
	_ = fw.Add(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && !isExcluded(e.Name()) {
			watchTree(fw, filepath.Join(dir, e.Name()))
		}
	}
}

// shouldReload 判断该路径改动是否需要重启：扩展名匹配且不在排除目录内。
func shouldReload(path string) bool {
	// 检查路径中任意目录段是否被排除（如 tmp/xxx.go 不会触发）
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if isExcluded(part) {
			return false
		}
	}
	switch filepath.Ext(path) {
	case ".go", ".yaml", ".yml", ".toml":
		return true
	}
	return false
}

// isExcluded 判断目录/文件名是否应被跳过（构建产物、版本库等）。
func isExcluded(name string) bool {
	switch name {
	case "tmp", "docs", "scripts", "migrations", ".git", ".dev":
		return true
	}
	return false
}
