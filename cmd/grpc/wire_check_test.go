package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestWireProviderSets 确保 wire ProviderSet 在当前提交下可以成功解析。
// 通过执行 `wire check`，即便开发者忘记手动运行 go generate，也能在 go test 阶段立即暴露缺失的 Bind / Provider。
func TestWireProviderSets(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve runtime caller information")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	packages := []string{
		"./cmd/grpc",
		"./cmd/tasks/catalog_inbox",
		"./cmd/tasks/outbox",
	}

	for _, pkg := range packages {
		pkg := pkg
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command("wire", "check", pkg)
			cmd.Dir = moduleRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("wire check %s failed: %v\n%s", pkg, err, string(output))
			}
		})
	}
}
