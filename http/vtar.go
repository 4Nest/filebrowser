package fbhttp

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	gopath "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/afero"
)

const (
	tarBlockSize = 512
	tarEOFSize   = 1024
)

type vtarEntry struct {
	fsPath    string
	isDir     bool
	hdrBytes  []byte
	hdrStart  int64
	hdrEnd    int64
	dataStart int64
	dataEnd   int64
	padEnd    int64
}

// VirtualTar presents a deterministic, uncompressed, seekable POSIX/PAX tar
// archive over one or more files/directories without writing any temporary
// archive to disk. Implementing io.ReadSeeker allows http.ServeContent to serve
// exact Content-Length, ETag, and HTTP Range requests (206 Partial Content) for
// multi-threaded downloaders (IDM / Aria2) and resumable transfers.
type VirtualTar struct {
	fsys      afero.Fs
	entries   []vtarEntry
	totalSize int64
	fileCount int
	modTime   time.Time
	etag      string

	offset     int64
	openIdx    int
	openFile   afero.File
	openFileAt int64
}

func buildTarHeaderBytes(info fs.FileInfo, nameInArchive string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, err
	}
	hdr.Name = nameInArchive
	if info.IsDir() && !strings.HasSuffix(hdr.Name, "/") {
		hdr.Name += "/"
		hdr.Size = 0
	}
	// Use PAX format so long paths (>100 bytes) and large files (>8 GiB, e.g. BDMV .m2ts)
	// are encoded cleanly and deterministically.
	hdr.Format = tar.FormatPAX
	hdr.Uid = 0
	hdr.Gid = 0
	hdr.Uname = ""
	hdr.Gname = ""
	hdr.AccessTime = time.Time{}
	hdr.ChangeTime = time.Time{}

	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	// Do not call tw.Close() because Close appends the 1024-byte EOF trailer.
	return buf.Bytes(), nil
}

type rawTarSource struct {
	fsPath        string
	nameInArchive string
	info          fs.FileInfo
}

func collectVirtualTarSources(d *data, path, commonPath string) ([]rawTarSource, error) {
	if !d.Check(path) {
		return nil, nil
	}

	info, err := d.user.Fs.Stat(path)
	if err != nil {
		return nil, err
	}

	var list []rawTarSource
	if path != commonPath {
		nameInArchive := strings.TrimPrefix(path, commonPath)
		nameInArchive = strings.TrimPrefix(nameInArchive, string(filepath.Separator))
		nameInArchive = filepath.ToSlash(nameInArchive)
		nameInArchive = strings.ReplaceAll(nameInArchive, "\\", "_")
		if cleaned := gopath.Clean("/" + nameInArchive); cleaned != "/"+nameInArchive {
			return nil, fmt.Errorf("refusing unsafe archive entry name: %q", nameInArchive)
		}
		// Include regular files and directories (skip symlinks/devices/pipes).
		if info.Mode().IsRegular() || info.IsDir() {
			list = append(list, rawTarSource{
				fsPath:        path,
				nameInArchive: nameInArchive,
				info:          info,
			})
		}
	}

	if info.IsDir() {
		f, err := d.user.Fs.Open(path)
		if err != nil {
			return nil, err
		}
		names, readErr := f.Readdirnames(0)
		_ = f.Close()
		if readErr != nil {
			return nil, readErr
		}
		// Deterministic lexicographical order guarantees identical byte offsets
		// across concurrent Range requests from multi-threaded downloaders.
		sort.Strings(names)
		for _, name := range names {
			sub, err := collectVirtualTarSources(d, filepath.Join(path, name), commonPath)
			if err != nil {
				continue
			}
			list = append(list, sub...)
		}
	}

	return list, nil
}

func NewVirtualTar(d *data, filenames []string, commonDir string) (*VirtualTar, error) {
	sortedRoots := append([]string(nil), filenames...)
	sort.Strings(sortedRoots)

	var sources []rawTarSource
	for _, root := range sortedRoots {
		sub, err := collectVirtualTarSources(d, root, commonDir)
		if err != nil {
			continue
		}
		sources = append(sources, sub...)
	}

	vt := &VirtualTar{
		fsys:    d.user.Fs,
		entries: make([]vtarEntry, 0, len(sources)),
		openIdx: -1,
	}

	hasher := sha256.New()
	var cursor int64

	for _, src := range sources {
		hdrBytes, err := buildTarHeaderBytes(src.info, src.nameInArchive)
		if err != nil {
			return nil, err
		}
		if src.info.ModTime().After(vt.modTime) {
			vt.modTime = src.info.ModTime()
		}

		var dataSize int64
		if !src.info.IsDir() {
			dataSize = src.info.Size()
			vt.fileCount++
		}

		hdrStart := cursor
		hdrEnd := hdrStart + int64(len(hdrBytes))
		dataStart := hdrEnd
		dataEnd := dataStart + dataSize
		padLen := (tarBlockSize - (dataSize % tarBlockSize)) % tarBlockSize
		padEnd := dataEnd + padLen

		vt.entries = append(vt.entries, vtarEntry{
			fsPath:    src.fsPath,
			isDir:     src.info.IsDir(),
			hdrBytes:  hdrBytes,
			hdrStart:  hdrStart,
			hdrEnd:    hdrEnd,
			dataStart: dataStart,
			dataEnd:   dataEnd,
			padEnd:    padEnd,
		})

		fmt.Fprintf(hasher, "%s|%d|%d\n", src.nameInArchive, dataSize, src.info.ModTime().Unix())
		cursor = padEnd
	}

	vt.totalSize = cursor + tarEOFSize
	if vt.modTime.IsZero() {
		vt.modTime = time.Now()
	}
	vt.etag = `"` + hex.EncodeToString(hasher.Sum(nil))[:24] + `"`

	return vt, nil
}

func (vt *VirtualTar) Size() int64       { return vt.totalSize }
func (vt *VirtualTar) FileCount() int    { return vt.fileCount }
func (vt *VirtualTar) ModTime() time.Time { return vt.modTime }
func (vt *VirtualTar) ETag() string      { return vt.etag }

func (vt *VirtualTar) closeCurrentFile() {
	if vt.openFile != nil {
		_ = vt.openFile.Close()
		vt.openFile = nil
		vt.openIdx = -1
		vt.openFileAt = 0
	}
}

func (vt *VirtualTar) Close() error {
	vt.closeCurrentFile()
	return nil
}

func (vt *VirtualTar) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = vt.offset + offset
	case io.SeekEnd:
		target = vt.totalSize + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if target < 0 {
		return 0, fmt.Errorf("negative seek position: %d", target)
	}
	vt.offset = target
	return vt.offset, nil
}

func (vt *VirtualTar) findEntry(pos int64) int {
	return sort.Search(len(vt.entries), func(i int) bool {
		return vt.entries[i].padEnd > pos
	})
}

func (vt *VirtualTar) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if vt.offset >= vt.totalSize {
		vt.closeCurrentFile()
		return 0, io.EOF
	}

	written := 0
	for written < len(p) && vt.offset < vt.totalSize {
		idx := vt.findEntry(vt.offset)
		if idx >= len(vt.entries) {
			// Trailing 1024-byte EOF zero blocks.
			vt.closeCurrentFile()
			rem := vt.totalSize - vt.offset
			n := int64(len(p) - written)
			if n > rem {
				n = rem
			}
			clear(p[written : written+int(n)])
			written += int(n)
			vt.offset += n
			continue
		}

		e := &vt.entries[idx]
		switch {
		case vt.offset < e.hdrEnd:
			// Serve tar header bytes.
			inHdr := vt.offset - e.hdrStart
			avail := int64(len(e.hdrBytes)) - inHdr
			n := int64(len(p) - written)
			if n > avail {
				n = avail
			}
			copy(p[written:written+int(n)], e.hdrBytes[inHdr:inHdr+n])
			written += int(n)
			vt.offset += n

		case vt.offset < e.dataEnd:
			// Serve file payload bytes.
			inFile := vt.offset - e.dataStart
			if vt.openIdx != idx || vt.openFile == nil {
				vt.closeCurrentFile()
				f, err := vt.fsys.Open(e.fsPath)
				if err != nil {
					if written > 0 {
						return written, nil
					}
					return 0, err
				}
				vt.openFile = f
				vt.openIdx = idx
				vt.openFileAt = 0
			}
			if vt.openFileAt != inFile {
				if _, err := vt.openFile.Seek(inFile, io.SeekStart); err != nil {
					vt.closeCurrentFile()
					if written > 0 {
						return written, nil
					}
					return 0, err
				}
				vt.openFileAt = inFile
			}

			avail := e.dataEnd - vt.offset
			want := int64(len(p) - written)
			if want > avail {
				want = avail
			}
			n, err := vt.openFile.Read(p[written : written+int(want)])
			if n > 0 {
				written += n
				vt.offset += int64(n)
				vt.openFileAt += int64(n)
			}
			if err == io.EOF {
				// File truncated on disk since stat; zero-pad remaining declared size so
				// tar stream stays structurally valid.
				if vt.offset < e.dataEnd {
					short := e.dataEnd - vt.offset
					canPad := int64(len(p) - written)
					if canPad > short {
						canPad = short
					}
					clear(p[written : written+int(canPad)])
					written += int(canPad)
					vt.offset += canPad
				}
				vt.closeCurrentFile()
				continue
			}
			if err != nil {
				vt.closeCurrentFile()
				if written > 0 {
					return written, nil
				}
				return 0, err
			}
			if vt.offset >= e.dataEnd {
				vt.closeCurrentFile()
			}

		default:
			// 512-byte alignment zero padding.
			vt.closeCurrentFile()
			avail := e.padEnd - vt.offset
			n := int64(len(p) - written)
			if n > avail {
				n = avail
			}
			clear(p[written : written+int(n)])
			written += int(n)
			vt.offset += n
		}
	}

	if written == 0 {
		return 0, io.EOF
	}
	return written, nil
}
