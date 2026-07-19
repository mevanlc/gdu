//go:build darwin

package analyze

import (
	"math"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type fileInfoWithStat struct {
	os.FileInfo
	stat *syscall.Stat_t
}

func (f fileInfoWithStat) Sys() any {
	return f.stat
}

func TestStatCompressedFileSizeOnAPFS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(path, []byte("logical contents"), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)
	if !isAPFS(path, stat.Dev) {
		t.Skip("test requires APFS")
	}

	compressedStat := *stat
	compressedStat.Flags |= unix.UF_COMPRESSED
	compressedStat.Blocks = 9
	compressedStat.Blksize = 4096
	compressedInfo := fileInfoWithStat{FileInfo: info, stat: &compressedStat}

	assert.Equal(t, int64(8192), statCompressedFileSize(path, compressedInfo))
	assert.Equal(t, info.Size(), statCompressedFileSize(path, info))
}

func TestRoundedAllocatedSize(t *testing.T) {
	tests := []struct {
		name      string
		blocks    int64
		blockSize int64
		fallback  int64
		expected  int64
	}{
		{name: "already aligned", blocks: 8, blockSize: 4096, fallback: 100, expected: 4096},
		{name: "rounds up", blocks: 9, blockSize: 4096, fallback: 100, expected: 8192},
		{name: "zero allocation", blocks: 0, blockSize: 4096, fallback: 100, expected: 0},
		{name: "negative blocks", blocks: -1, blockSize: 4096, fallback: 100, expected: 100},
		{name: "invalid block size", blocks: 8, blockSize: 0, fallback: 100, expected: 100},
		{name: "block multiplication overflow", blocks: math.MaxInt64, blockSize: 4096, fallback: 100, expected: 100},
		{
			name:      "rounding overflow",
			blocks:    math.MaxInt64 / statBlockSize,
			blockSize: (math.MaxInt64/statBlockSize)*statBlockSize - 1000,
			fallback:  100,
			expected:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, roundedAllocatedSize(tt.blocks, tt.blockSize, tt.fallback))
		})
	}
}
