package fbhttp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type mediaInfoSummary struct {
	Format      string   `json:"format,omitempty"`
	FileSize    string   `json:"fileSize,omitempty"`
	Duration    string   `json:"duration,omitempty"`
	BitRate     string   `json:"bitRate,omitempty"`
	Video       []string `json:"video,omitempty"`
	Audio       []string `json:"audio,omitempty"`
	Subtitles   int      `json:"subtitles"`
	HasHDR      bool     `json:"hasHdr"`
	HDRLabel    string   `json:"hdrLabel,omitempty"`
}

type mediaInfoResponse struct {
	Path    string           `json:"path"`
	Name    string           `json:"name"`
	Text    string           `json:"text"`
	Summary mediaInfoSummary `json:"summary"`
}

func resolveMediaInfoBin() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("FB_MEDIAINFO_BIN")); custom != "" {
		return custom, nil
	}
	if p, err := exec.LookPath("mediainfo"); err == nil {
		return p, nil
	}
	for _, candidate := range []string{"/usr/bin/mediainfo", "/usr/local/bin/mediainfo"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("mediainfo binary not found on server (install mediainfo or set FB_MEDIAINFO_BIN)")
}

func resolveBDInfoBin() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("FB_BDINFO_BIN")); custom != "" {
		return custom, nil
	}
	for _, name := range []string{"bdinfo", "BDInfoCLI", "bdinfo-cli"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	for _, candidate := range []string{
		"/usr/local/bin/bdinfo",
		"/usr/bin/bdinfo",
		"/usr/local/bin/BDInfoCLI",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("bdinfo binary not found on server (install bdinfo or set FB_BDINFO_BIN)")
}

func parseMediaInfoSummary(report string) mediaInfoSummary {
	var sum mediaInfoSummary
	section := ""
	var curVideo, curAudio map[string]string

	flushVideo := func() {
		if len(curVideo) == 0 {
			return
		}
		var parts []string
		if w, h := curVideo["Width"], curVideo["Height"]; w != "" && h != "" {
			w = strings.ReplaceAll(strings.TrimSuffix(w, " pixels"), " ", "")
			h = strings.ReplaceAll(strings.TrimSuffix(h, " pixels"), " ", "")
			parts = append(parts, w+"×"+h)
		}
		if f := curVideo["Format"]; f != "" {
			parts = append(parts, f)
		}
		if bd := curVideo["Bit depth"]; bd != "" {
			parts = append(parts, bd)
		}
		if hdr := curVideo["HDR format"]; hdr != "" {
			sum.HasHDR = true
			if strings.Contains(hdr, "Dolby Vision") {
				sum.HDRLabel = "Dolby Vision"
			} else if strings.Contains(hdr, "HDR10+") {
				sum.HDRLabel = "HDR10+"
			} else {
				sum.HDRLabel = "HDR10"
			}
			parts = append(parts, sum.HDRLabel)
		}
		if len(parts) > 0 {
			sum.Video = append(sum.Video, strings.Join(parts, " · "))
		}
		curVideo = nil
	}

	flushAudio := func() {
		if len(curAudio) == 0 {
			return
		}
		var parts []string
		if lang := curAudio["Language"]; lang != "" {
			parts = append(parts, lang)
		}
		if f := curAudio["Commercial name"]; f != "" {
			parts = append(parts, f)
		} else if f := curAudio["Format"]; f != "" {
			parts = append(parts, f)
		}
		if ch := curAudio["Channel(s)"]; ch != "" {
			parts = append(parts, ch)
		}
		if len(parts) > 0 {
			sum.Audio = append(sum.Audio, strings.Join(parts, " "))
		}
		curAudio = nil
	}

	scanner := bufio.NewScanner(strings.NewReader(report))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":") {
			flushVideo()
			flushAudio()
			lower := strings.ToLower(line)
			switch {
			case lower == "general":
				section = "general"
			case strings.HasPrefix(lower, "video"):
				section = "video"
				curVideo = map[string]string{}
			case strings.HasPrefix(lower, "audio"):
				section = "audio"
				curAudio = map[string]string{}
			case strings.HasPrefix(lower, "text"):
				section = "text"
				sum.Subtitles++
			default:
				section = ""
			}
			continue
		}

		idx := strings.Index(line, ":")
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		switch section {
		case "general":
			switch key {
			case "Format":
				if sum.Format == "" {
					sum.Format = val
				}
			case "File size":
				if sum.FileSize == "" {
					sum.FileSize = val
				}
			case "Duration":
				if sum.Duration == "" {
					sum.Duration = val
				}
			case "Overall bit rate":
				if sum.BitRate == "" {
					sum.BitRate = val
				}
			}
		case "video":
			if curVideo != nil {
				if _, exists := curVideo[key]; !exists {
					curVideo[key] = val
				}
			}
		case "audio":
			if curAudio != nil {
				if _, exists := curAudio[key]; !exists {
					curAudio[key] = val
				}
			}
		}
	}
	flushVideo()
	flushAudio()
	return sum
}

var mediaInfoHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	reqPath := slashClean(r.URL.Path)
	if !d.Check(reqPath) {
		return http.StatusForbidden, nil
	}

	st, err := d.user.Fs.Stat(reqPath)
	if err != nil {
		return errToStatus(err), err
	}
	if st.IsDir() {
		return http.StatusBadRequest, errors.New("mediainfo requires a file, not a directory")
	}

	bin, err := resolveMediaInfoBin()
	if err != nil {
		return http.StatusServiceUnavailable, err
	}

	fullPath := d.user.FullPath(reqPath)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, fullPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return http.StatusInternalServerError, fmt.Errorf("mediainfo failed: %s", msg)
	}

	raw := stdout.String()
	// Strip server absolute path prefix so Complete name shows only filename.
	cleaned := sanitizeMediaInfoPath(raw, fullPath, st.Name())

	return renderJSON(w, r, mediaInfoResponse{
		Path:    reqPath,
		Name:    st.Name(),
		Text:    cleaned,
		Summary: parseMediaInfoSummary(cleaned),
	})
})

func sanitizeMediaInfoPath(report, fullPath, baseName string) string {
	lines := strings.Split(report, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Complete name") && strings.Contains(line, fullPath) {
			lines[i] = strings.Replace(line, fullPath, baseName, 1)
			break
		}
	}
	return strings.Join(lines, "\n")
}

// --- BDInfo Async Task Manager ---

type bdInfoTask struct {
	ID           string    `json:"id"`
	UserID       uint      `json:"-"`
	Path         string    `json:"path"`
	TargetPath   string    `json:"targetPath"`
	Status       string    `json:"status"` // "running", "done", "error"
	Progress     int       `json:"progress"`
	Stage        string    `json:"stage"`
	QuickSummary string    `json:"quickSummary,omitempty"`
	FullReport   string    `json:"fullReport,omitempty"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt,omitempty"`
	ElapsedSec   float64   `json:"elapsedSec"`
}

type bdInfoScanRequest struct {
	Path  string `json:"path"`
	Force bool   `json:"force"`
}

type bdInfoManager struct {
	mu      sync.RWMutex
	tasks   map[string]*bdInfoTask
	byPath  map[string]string // fullPath -> latest taskID
}

var globalBDInfo = &bdInfoManager{
	tasks:  map[string]*bdInfoTask{},
	byPath: map[string]string{},
}

func newTaskID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func detectBDMVRoot(fullPath string) (string, error) {
	st, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if strings.EqualFold(filepath.Ext(fullPath), ".iso") {
			return fullPath, nil
		}
		return "", errors.New("BDInfo requires a BDMV folder, Blu-ray root directory, or .iso file")
	}

	// Case 1: Selected directory itself contains BDMV/index.bdmv
	if _, err := os.Stat(filepath.Join(fullPath, "BDMV", "index.bdmv")); err == nil {
		return fullPath, nil
	}
	// Case 2: Selected directory IS the BDMV folder itself
	if strings.EqualFold(filepath.Base(fullPath), "BDMV") {
		if _, err := os.Stat(filepath.Join(fullPath, "index.bdmv")); err == nil {
			return filepath.Dir(fullPath), nil
		}
	}
	// Case 3: Selected directory has a single child folder containing BDMV/index.bdmv
	entries, err := os.ReadDir(fullPath)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidate := filepath.Join(fullPath, e.Name())
				if _, err := os.Stat(filepath.Join(candidate, "BDMV", "index.bdmv")); err == nil {
					return candidate, nil
				}
			}
		}
	}
	return "", errors.New("no BDMV/index.bdmv structure found in selected directory")
}

func extractBDInfoQuickSummary(fullReport string) string {
	idx := strings.Index(fullReport, "QUICK SUMMARY:")
	if idx < 0 {
		return strings.TrimSpace(fullReport)
	}
	rest := fullReport[idx:]
	// Quick summary usually ends before "FILES:" or "CHAPTERS:" or "STREAM DIAGNOSTICS:"
	for _, stop := range []string{"\nFILES:", "\nCHAPTERS:", "\nSTREAM DIAGNOSTICS:", "\n\n\n"} {
		if sIdx := strings.Index(rest, stop); sIdx > 0 {
			rest = rest[:sIdx]
			break
		}
	}
	return strings.TrimSpace(rest)
}

var pctRegex = regexp.MustCompile(`(\d{1,3})%`)

func (m *bdInfoManager) startScan(userID uint, reqPath, fullPath string, force bool) (*bdInfoTask, error) {
	bin, err := resolveBDInfoBin()
	if err != nil {
		return nil, err
	}

	targetPath, err := detectBDMVRoot(fullPath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	// Reuse running task or recent cached completed task (30 min).
	if !force {
		if oldID, ok := m.byPath[targetPath]; ok {
			if old, exists := m.tasks[oldID]; exists && old.UserID == userID {
				if old.Status == "running" || (old.Status == "done" && time.Since(old.FinishedAt) < 30*time.Minute) {
					cp := *old
					cp.ElapsedSec = time.Since(old.StartedAt).Seconds()
					if old.Status == "done" {
						cp.ElapsedSec = old.FinishedAt.Sub(old.StartedAt).Seconds()
					}
					m.mu.Unlock()
					return &cp, nil
				}
			}
		}
	}

	task := &bdInfoTask{
		ID:         newTaskID(),
		UserID:     userID,
		Path:       reqPath,
		TargetPath: filepath.Base(targetPath),
		Status:     "running",
		Progress:   5,
		Stage:      "Starting BDInfo scanner...",
		StartedAt:  time.Now(),
	}
	m.tasks[task.ID] = task
	m.byPath[targetPath] = task.ID
	m.mu.Unlock()

	go m.runTask(task.ID, bin, targetPath)

	cp := *task
	return &cp, nil
}

func (m *bdInfoManager) runTask(taskID, bin, targetPath string) {
	tmpDir, err := os.MkdirTemp("", "fb-bdinfo-*")
	if err != nil {
		m.failTask(taskID, fmt.Sprintf("failed to create temp dir: %v", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// BDInfoCLI / wrapper convention: `bdinfo <targetPath> <outputDir>`
	// Support passing custom extra args via FB_BDINFO_ARGS if desired.
	args := []string{targetPath, tmpDir}
	if extra := strings.TrimSpace(os.Getenv("FB_BDINFO_ARGS")); extra != "" {
		args = append(strings.Fields(extra), args...)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.failTask(taskID, err.Error())
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		m.failTask(taskID, fmt.Sprintf("failed to start bdinfo: %v", err))
		return
	}

	var outBuf bytes.Buffer
	reader := bufio.NewReader(stdoutPipe)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			outBuf.WriteString(line)
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				m.updateProgress(taskID, trimmed)
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				break
			}
			break
		}
	}

	runErr := cmd.Wait()

	// Look for generated report file in tmpDir (BDINFO.*.txt or *.txt).
	var reportText string
	entries, _ := os.ReadDir(tmpDir)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			if b, err := os.ReadFile(filepath.Join(tmpDir, e.Name())); err == nil && len(b) > 0 {
				reportText = string(b)
				break
			}
		}
	}
	if strings.TrimSpace(reportText) == "" {
		// Some bdinfo wrappers print the report directly to stdout.
		if strings.Contains(outBuf.String(), "DISC INFO:") || strings.Contains(outBuf.String(), "PLAYLIST REPORT:") {
			reportText = outBuf.String()
		}
	}

	if runErr != nil && strings.TrimSpace(reportText) == "" {
		tail := strings.TrimSpace(outBuf.String())
		if len(tail) > 500 {
			tail = tail[len(tail)-500:]
		}
		m.failTask(taskID, fmt.Sprintf("bdinfo failed: %v\n%s", runErr, tail))
		return
	}
	if strings.TrimSpace(reportText) == "" {
		m.failTask(taskID, "bdinfo completed without producing a report")
		return
	}

	quick := extractBDInfoQuickSummary(reportText)

	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.Status = "done"
		t.Progress = 100
		t.Stage = "Completed"
		t.FullReport = strings.TrimSpace(reportText)
		t.QuickSummary = quick
		t.FinishedAt = time.Now()
		t.ElapsedSec = t.FinishedAt.Sub(t.StartedAt).Seconds()
	}
	m.mu.Unlock()
}

func (m *bdInfoManager) updateProgress(taskID, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[taskID]
	if !ok {
		return
	}
	if len(line) > 120 {
		line = line[:120]
	}
	t.Stage = line
	if matches := pctRegex.FindStringSubmatch(line); len(matches) == 2 {
		if pct, err := strconv.Atoi(matches[1]); err == nil && pct >= 0 && pct <= 99 {
			if pct > t.Progress {
				t.Progress = pct
			}
		}
	} else if t.Progress < 90 {
		t.Progress++
	}
}

func (m *bdInfoManager) failTask(taskID, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[taskID]; ok {
		t.Status = "error"
		t.Error = msg
		t.Stage = "Failed"
		t.FinishedAt = time.Now()
		t.ElapsedSec = t.FinishedAt.Sub(t.StartedAt).Seconds()
	}
}

func (m *bdInfoManager) getTask(taskID string, userID uint) (*bdInfoTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[taskID]
	if !ok || t.UserID != userID {
		return nil, false
	}
	cp := *t
	if cp.Status == "running" {
		cp.ElapsedSec = time.Since(cp.StartedAt).Seconds()
	}
	return &cp, true
}

var bdInfoScanHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var req bdInfoScanRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	reqPath := slashClean(req.Path)
	if !d.Check(reqPath) {
		return http.StatusForbidden, nil
	}
	fullPath := d.user.FullPath(reqPath)

	task, err := globalBDInfo.startScan(d.user.ID, reqPath, fullPath, req.Force)
	if err != nil {
		return http.StatusBadRequest, err
	}
	return renderJSON(w, r, task)
})

var bdInfoTaskHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	id := mux.Vars(r)["id"]
	task, ok := globalBDInfo.getTask(id, d.user.ID)
	if !ok {
		return http.StatusNotFound, nil
	}
	return renderJSON(w, r, task)
})
