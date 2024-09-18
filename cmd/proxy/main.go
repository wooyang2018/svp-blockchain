package main

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

func executeCommand(cmd string) (string, error) {
	// 创建命令
	command := exec.Command("sh", "-c", cmd)
	// 获取输出
	output, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体中的命令
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	cmd := string(body)
	output, err := executeCommand(cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("Command execution error: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回输出
	w.Write([]byte(output))
}

func main() {
	http.HandleFunc("/execute", commandHandler)
	fmt.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
