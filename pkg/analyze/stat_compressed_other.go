//go:build !darwin

package analyze

import "os"

func statCompressedFileSize(_ string, info os.FileInfo) int64 {
	return info.Size()
}
