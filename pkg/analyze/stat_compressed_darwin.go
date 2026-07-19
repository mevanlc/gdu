//go:build darwin

package analyze

import (
	"bytes"
	"math"
	"os"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

const statBlockSize = int64(512)

var statCompressedFilesystems sync.Map

func statCompressedFileSize(path string, info os.FileInfo) int64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || stat.Flags&unix.UF_COMPRESSED == 0 {
		return info.Size()
	}
	if !isAPFS(path, stat.Dev) {
		return info.Size()
	}

	return roundedAllocatedSize(stat.Blocks, int64(stat.Blksize), info.Size())
}

func isAPFS(path string, device int32) bool {
	if cached, ok := statCompressedFilesystems.Load(device); ok {
		return cached.(bool)
	}

	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return false
	}
	end := bytes.IndexByte(stat.Fstypename[:], 0)
	if end < 0 {
		end = len(stat.Fstypename)
	}
	supported := string(stat.Fstypename[:end]) == "apfs"
	statCompressedFilesystems.Store(device, supported)
	return supported
}

func roundedAllocatedSize(blocks, blockSize, fallback int64) int64 {
	if blocks < 0 || blockSize <= 0 || blocks > math.MaxInt64/statBlockSize {
		return fallback
	}
	size := blocks * statBlockSize
	remainder := size % blockSize
	if remainder == 0 {
		return size
	}
	increment := blockSize - remainder
	if size > math.MaxInt64-increment {
		return fallback
	}
	return size + increment
}
