package terminal

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestNpm(t *testing.T) {
	// cmd := exec.Command("npm", "run", "build")

	cmd := exec.Command("cmd", "/C", "npm run build")
	cmd.Dir = "D:\\godata\\go-build-admin\\web"

	// cmd := exec.Command("wire")
	// cmd.Dir = "D:\\godata\\go-build-admin\\cmd\\app"

	// 创建管道以捕获标准输出和标准错误
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("error creating stdout pipe: %s", err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("error creating stderr pipe: %s", err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("error starting command: %s", err.Error())
		return
	}

	// 读取并打印标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 读取并打印标准错误
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintln(os.Stderr, scanner.Text())
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		fmt.Printf("error waiting for command: %s", err.Error())
		return
	}
}
