package fbhttp

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

func TestVirtualTarIntegrityAndRangeSeek(t *testing.T) {
	mem := afero.NewMemMapFs()
	require.NoError(t, mem.MkdirAll("/Disc1/BDMV/STREAM", 0o755))
	require.NoError(t, mem.MkdirAll("/Disc1/BDMV/PLAYLIST", 0o755))

	wantFiles := map[string][]byte{
		"Disc1/BDMV/index.bdmv":          []byte("INDX0200-small-header"),
		"Disc1/BDMV/PLAYLIST/00000.mpls": bytes.Repeat([]byte("MPLS"), 300),
		"Disc1/BDMV/STREAM/00001.m2ts":   bytes.Repeat([]byte{0x47, 0x01, 0x02, 0x03, 0x04}, 1024),
	}
	for p, content := range wantFiles {
		require.NoError(t, afero.WriteFile(mem, "/"+p, content, 0o644))
	}

	d := &data{
		settings: &settings.Settings{},
		server:   &settings.Server{},
		user: &users.User{
			Fs: mem,
		},
	}

	vt, err := NewVirtualTar(d, []string{"/Disc1"}, "/")
	require.NoError(t, err)
	defer vt.Close()

	require.Equal(t, 3, vt.FileCount())
	require.Greater(t, vt.Size(), int64(5120))

	// 1. Sequential read must produce a valid tar archive with identical files.
	fullBytes, err := io.ReadAll(vt)
	require.NoError(t, err)
	require.Equal(t, vt.Size(), int64(len(fullBytes)))

	tr := tar.NewReader(bytes.NewReader(fullBytes))
	gotFiles := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(tr)
			require.NoError(t, err)
			gotFiles[hdr.Name] = b
		}
	}
	require.Equal(t, wantFiles, gotFiles)

	// 2. Simulated multi-threaded Range requests (random seeks across headers,
	// file bodies, 512-byte padding, and EOF) must match fullBytes byte-for-byte.
	const chunk = 777
	reconstructed := make([]byte, len(fullBytes))
	for off := len(fullBytes) - chunk; off > -chunk; off -= chunk {
		start := off
		if start < 0 {
			start = 0
		}
		end := off + chunk
		if end > len(fullBytes) {
			end = len(fullBytes)
		}
		pos, err := vt.Seek(int64(start), io.SeekStart)
		require.NoError(t, err)
		require.Equal(t, int64(start), pos)

		buf := make([]byte, end-start)
		n, err := io.ReadFull(vt, buf)
		require.NoError(t, err)
		require.Equal(t, end-start, n)
		copy(reconstructed[start:end], buf)
	}
	require.Equal(t, fullBytes, reconstructed)
}
