package scan

import "os"

// writeFile 是测试辅助; 放独立文件避免 _test.go 之间的导入耦合。
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
