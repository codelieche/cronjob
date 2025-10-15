package runner

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// prepareBuildContext 准备构建上下文（打包为 tar 格式）
func (r *ContainerRunner) prepareBuildContext() (io.ReadCloser, error) {
	// 获取绝对路径
	contextPath, err := filepath.Abs(r.config.Context)
	if err != nil {
		return nil, fmt.Errorf("获取构建上下文路径失败: %w", err)
	}

	// 检查路径是否存在
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("构建上下文路径不存在: %s", contextPath)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "docker-build-context-*.tar")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 创建 tar writer
	tarWriter := tar.NewWriter(tmpFile)
	defer tarWriter.Close()

	// 遍历上下文目录，打包文件
	err = filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过 .git 目录
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// 获取相对路径
		relPath, err := filepath.Rel(contextPath, path)
		if err != nil {
			return err
		}

		// 创建 tar header
		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}
		header.Name = relPath

		// 写入 header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("打包构建上下文失败: %w", err)
	}

	// 关闭 tar writer
	if err := tarWriter.Close(); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("关闭 tar writer 失败: %w", err)
	}

	// 重新打开文件用于读取
	tmpFile.Close()
	readFile, err := os.Open(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("打开临时文件失败: %w", err)
	}

	// 返回一个包装的 ReadCloser，会在关闭时删除临时文件
	return &buildContextReader{
		file:     readFile,
		tempPath: tmpFile.Name(),
	}, nil
}

// buildContextReader 包装 os.File，在关闭时删除临时文件
type buildContextReader struct {
	file     *os.File
	tempPath string
}

func (r *buildContextReader) Read(p []byte) (n int, err error) {
	return r.file.Read(p)
}

func (r *buildContextReader) Close() error {
	r.file.Close()
	os.Remove(r.tempPath)
	return nil
}
