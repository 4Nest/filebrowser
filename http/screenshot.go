package fbhttp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type screenshotCaptureRequest struct {
	Path       string   `json:"path"`
	Count      int      `json:"count"`      // 1..12, default 6
	Format     string   `json:"format"`     // png | jpg | webp
	Quality    int      `json:"quality"`    // 70..100 for jpg/webp
	StartPct   int      `json:"startPct"`   // 0..90, default 10
	EndPct     int      `json:"endPct"`     // 10..100, default 90
	Timestamps []string `json:"timestamps"` // optional explicit timestamps (e.g. "00:05:12")
	HDRToneMap string   `json:"hdrToneMap"` // auto | on | off
	Width      int      `json:"width"`      // 0 = native
}

type screenshotItem struct {
	ID         string `json:"id"`
	Index      int    `json:"index"`
	Timestamp  string `json:"timestamp"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mimeType"`
	Size       int64  `json:"size"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	PreviewURL string `json:"previewUrl"`
}

type screenshotCaptureResponse struct {
	Path     string           `json:"path"`
	Source   string           `json:"source"`
	Duration float64          `json:"duration"`
	IsHDR    bool             `json:"isHdr"`
	ToneMap  bool             `json:"toneMapApplied"`
	Items    []screenshotItem `json:"items"`
}

type cachedShot struct {
	ID        string
	UserID    uint
	Filename  string
	MimeType  string
	Data      []byte
	CreatedAt time.Time
}

type shotStore struct {
	mu    sync.RWMutex
	items map[string]*cachedShot
}

var globalShots = &shotStore{
	items: map[string]*cachedShot{},
}

func (s *shotStore) put(shot *cachedShot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Opportunistic cleanup of shots older than 30 minutes.
	now := time.Now()
	for k, v := range s.items {
		if now.Sub(v.CreatedAt) > 30*time.Minute {
			delete(s.items, k)
		}
	}
	s.items[shot.ID] = shot
}

func (s *shotStore) get(id string, userID uint) (*cachedShot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[id]
	if !ok || v.UserID != userID {
		return nil, false
	}
	return v, true
}

func newShotID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func resolveFFBin(name, envKey string) (string, error) {
	if custom := strings.TrimSpace(os.Getenv(envKey)); custom != "" {
		return custom, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	for _, c := range []string{"/usr/bin/" + name, "/usr/local/bin/" + name} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("%s not found on server (install ffmpeg or set %s)", name, envKey)
}

type ffprobeResult struct {
	Duration float64
	Width    int
	Height   int
	IsHDR    bool
}

func probeVideo(ctx context.Context, ffprobeBin, filePath string) (*ffprobeResult, error) {
	cmd := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,color_transfer,color_primaries:format=duration",
		"-of", "json",
		filePath,
	)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %s", strings.TrimSpace(errBuf.String()))
	}

	var parsed struct {
		Streams []struct {
			Width          int    `json:"width"`
			Height         int    `json:"height"`
			ColorTransfer  string `json:"color_transfer"`
			ColorPrimaries string `json:"color_primaries"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		return nil, err
	}

	res := &ffprobeResult{}
	if d, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
		res.Duration = d
	}
	if len(parsed.Streams) > 0 {
		st := parsed.Streams[0]
		res.Width = st.Width
		res.Height = st.Height
		tr := strings.ToLower(st.ColorTransfer)
		pr := strings.ToLower(st.ColorPrimaries)
		if tr == "smpte2084" || tr == "arib-std-b67" || pr == "bt2020" {
			res.IsHDR = true
		}
	}
	return res, nil
}

// If user selects a BDMV directory, auto-pick the largest .m2ts stream file.
func resolveScreenshotTarget(fullPath string) (string, error) {
	st, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return fullPath, nil
	}

	streamDirs := []string{
		filepath.Join(fullPath, "BDMV", "STREAM"),
		filepath.Join(fullPath, "STREAM"),
		fullPath,
	}
	var bestPath string
	var bestSize int64
	for _, dir := range streamDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".m2ts" || ext == ".mkv" || ext == ".mp4" || ext == ".ts" {
				if info, err := e.Info(); err == nil && info.Size() > bestSize {
					bestSize = info.Size()
					bestPath = filepath.Join(dir, e.Name())
				}
			}
		}
		if bestPath != "" {
			return bestPath, nil
		}
	}
	return "", errors.New("未在所选目录中找到可截图的视频文件或 BDMV/STREAM/*.m2ts")
}

func formatTimestamp(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	total := int(sec)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

var screenshotCaptureHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var req screenshotCaptureRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	reqPath := slashClean(req.Path)
	if !d.Check(reqPath) {
		return http.StatusForbidden, nil
	}

	ffmpegBin, err := resolveFFBin("ffmpeg", "FB_FFMPEG_BIN")
	if err != nil {
		return http.StatusServiceUnavailable, err
	}
	ffprobeBin, err := resolveFFBin("ffprobe", "FB_FFPROBE_BIN")
	if err != nil {
		return http.StatusServiceUnavailable, err
	}

	targetPath, err := resolveScreenshotTarget(d.user.FullPath(reqPath))
	if err != nil {
		return http.StatusBadRequest, err
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	info, err := probeVideo(ctx, ffprobeBin, targetPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if req.Count <= 0 {
		req.Count = 6
	}
	if req.Count > 12 {
		req.Count = 12
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format != "jpg" && format != "webp" {
		format = "png"
	}
	if req.Quality <= 0 || req.Quality > 100 {
		req.Quality = 92
	}
	if req.StartPct < 0 || req.StartPct >= 95 {
		req.StartPct = 10
	}
	if req.EndPct <= req.StartPct || req.EndPct > 100 {
		req.EndPct = 90
	}
	hdrMode := strings.ToLower(strings.TrimSpace(req.HDRToneMap))
	if hdrMode == "" {
		hdrMode = "auto"
	}
	applyToneMap := hdrMode == "on" || (hdrMode == "auto" && info.IsHDR)

	var tsList []string
	for _, rawTS := range req.Timestamps {
		t := strings.TrimSpace(rawTS)
		if t != "" {
			tsList = append(tsList, t)
		}
	}
	if len(tsList) == 0 {
		dur := info.Duration
		if dur <= 1 {
			dur = 60
		}
		startSec := dur * float64(req.StartPct) / 100.0
		endSec := dur * float64(req.EndPct) / 100.0
		if req.Count == 1 {
			tsList = append(tsList, formatTimestamp((startSec+endSec)/2))
		} else {
			step := (endSec - startSec) / float64(req.Count-1)
			for i := 0; i < req.Count; i++ {
				tsList = append(tsList, formatTimestamp(startSec+step*float64(i)))
			}
		}
	}

	var vfParts []string
	if applyToneMap {
		vfParts = append(vfParts,
			"zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p",
		)
	}
	if req.Width > 0 && (info.Width == 0 || req.Width < info.Width) {
		vfParts = append(vfParts, fmt.Sprintf("scale=%d:-2:flags=lanczos", req.Width))
	}
	vf := strings.Join(vfParts, ",")

	ext := "." + format
	mimeType := "image/png"
	if format == "jpg" {
		mimeType = "image/jpeg"
	} else if format == "webp" {
		mimeType = "image/webp"
	}

	tmpDir, err := os.MkdirTemp("", "fb-shots-*")
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer os.RemoveAll(tmpDir)

	type jobRes struct {
		item screenshotItem
		shot *cachedShot
		err  error
	}
	results := make([]jobRes, len(tsList))
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup

	baseName := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(targetPath))
	basePrefix := strings.TrimSuffix(d.server.BaseURL, "/")

	for idx, ts := range tsList {
		wg.Add(1)
		go func(i int, timestamp string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			outName := fmt.Sprintf("shot_%02d%s", i+1, ext)
			outPath := filepath.Join(tmpDir, outName)

			args := []string{"-y", "-ss", timestamp, "-i", targetPath, "-frames:v", "1"}
			if vf != "" {
				args = append(args, "-vf", vf)
			}
			switch format {
			case "jpg":
				// map 70..100 to qscale 5..2
				q := 2
				if req.Quality < 95 {
					q = 3
				}
				if req.Quality < 85 {
					q = 4
				}
				if req.Quality < 75 {
					q = 5
				}
				args = append(args, "-q:v", strconv.Itoa(q))
			case "webp":
				args = append(args, "-quality", strconv.Itoa(req.Quality))
			}
			args = append(args, outPath)

			cmd := exec.CommandContext(ctx, ffmpegBin, args...)
			var errBuf bytes.Buffer
			cmd.Stderr = &errBuf
			if err := cmd.Run(); err != nil {
				// Fallback without zscale tonemap if host ffmpeg lacks libzimg
				if applyToneMap && strings.Contains(errBuf.String(), "zscale") {
					fallbackArgs := []string{"-y", "-ss", timestamp, "-i", targetPath, "-frames:v", "1"}
					if req.Width > 0 {
						fallbackArgs = append(fallbackArgs, "-vf", fmt.Sprintf("scale=%d:-2:flags=lanczos", req.Width))
					}
					fallbackArgs = append(fallbackArgs, outPath)
					if err2 := exec.CommandContext(ctx, ffmpegBin, fallbackArgs...).Run(); err2 != nil {
						results[i] = jobRes{err: fmt.Errorf("ffmpeg failed at %s: %v", timestamp, err2)}
						return
					}
				} else {
					results[i] = jobRes{err: fmt.Errorf("ffmpeg failed at %s: %s", timestamp, strings.TrimSpace(errBuf.String()))}
					return
				}
			}

			data, err := os.ReadFile(outPath)
			if err != nil {
				results[i] = jobRes{err: err}
				return
			}

			id := newShotID()
			dlName := fmt.Sprintf("%s_%02d_%s%s", baseName, i+1, strings.ReplaceAll(timestamp, ":", "-"), ext)
			shot := &cachedShot{
				ID:        id,
				UserID:    d.user.ID,
				Filename:  dlName,
				MimeType:  mimeType,
				Data:      data,
				CreatedAt: time.Now(),
			}
			outW, outH := info.Width, info.Height
			if req.Width > 0 && info.Width > 0 && req.Width < info.Width {
				outH = int(float64(info.Height) * float64(req.Width) / float64(info.Width))
				outW = req.Width
			}
			results[i] = jobRes{
				shot: shot,
				item: screenshotItem{
					ID:         id,
					Index:      i + 1,
					Timestamp:  timestamp,
					Filename:   dlName,
					MimeType:   mimeType,
					Size:       int64(len(data)),
					Width:      outW,
					Height:     outH,
					PreviewURL: fmt.Sprintf("%s/api/screenshots/file/%s", basePrefix, id),
				},
			}
		}(idx, ts)
	}
	wg.Wait()

	var items []screenshotItem
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		globalShots.put(r.shot)
		items = append(items, r.item)
	}
	if len(items) == 0 && firstErr != nil {
		return http.StatusInternalServerError, firstErr
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })

	return renderJSON(w, r, screenshotCaptureResponse{
		Path:     reqPath,
		Source:   filepath.Base(targetPath),
		Duration: info.Duration,
		IsHDR:    info.IsHDR,
		ToneMap:  applyToneMap,
		Items:    items,
	})
})

var screenshotFileHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	id := mux.Vars(r)["id"]
	shot, ok := globalShots.get(id, d.user.ID)
	if !ok {
		return http.StatusNotFound, nil
	}
	w.Header().Set("Content-Type", shot.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=1800")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, shot.Filename))
	}
	_, _ = w.Write(shot.Data)
	return 0, nil
})

// --- Multi-Host Image Uploader ---

type imageHostConfig struct {
	Provider    string            `json:"provider"`    // picx | chevereto | pixhost | lsky | custom
	Endpoint    string            `json:"endpoint"`    // API upload URL
	APIKey      string            `json:"apiKey"`      // X-API-Key or Bearer token
	FieldName   string            `json:"fieldName"`   // multipart file field (e.g. "source", "file", "img")
	AuthHeader  string            `json:"authHeader"`  // custom auth header name
	AuthPrefix  string            `json:"authPrefix"`  // e.g. "Bearer "
	URLJsonPath string            `json:"urlJsonPath"` // dot path for custom provider, e.g. "data.url"
	ExtraFields map[string]string `json:"extraFields"`
}

type screenshotUploadRequest struct {
	ShotIDs []string        `json:"shotIds"`
	Host    imageHostConfig `json:"host"`
}

type uploadedShotResult struct {
	ShotID   string `json:"shotId"`
	Filename string `json:"filename"`
	URL      string `json:"url,omitempty"`
	ThumbURL string `json:"thumbUrl,omitempty"`
	BBCode   string `json:"bbcode,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	Error    string `json:"error,omitempty"`
}

type screenshotUploadResponse struct {
	Results  []uploadedShotResult `json:"results"`
	BBCode   string               `json:"bbcode"`
	Markdown string               `json:"markdown"`
	Direct   string               `json:"direct"`
}

func jsonPathString(root interface{}, dotPath string) string {
	cur := root
	for _, part := range strings.Split(dotPath, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur, ok = m[part]
		if !ok {
			return ""
		}
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func uploadSingleShot(ctx context.Context, shot *cachedShot, cfg imageHostConfig) (imgURL, thumbURL string, err error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	endpoint := strings.TrimSpace(cfg.Endpoint)
	fieldName := strings.TrimSpace(cfg.FieldName)
	authHeader := strings.TrimSpace(cfg.AuthHeader)
	authPrefix := cfg.AuthPrefix
	extra := map[string]string{}
	for k, v := range cfg.ExtraFields {
		extra[k] = v
	}

	switch provider {
	case "picx":
		if endpoint == "" {
			endpoint = "https://picx.cx/api/1/upload"
		}
		if fieldName == "" {
			fieldName = "source"
		}
		authHeader = "X-API-Key"
		authPrefix = ""
		extra["format"] = "json"
	case "chevereto":
		if endpoint == "" {
			return "", "", errors.New("请填写 Chevereto 图床 API 上传地址")
		}
		if fieldName == "" {
			fieldName = "source"
		}
		if authHeader == "" {
			authHeader = "X-API-Key"
		}
		extra["format"] = "json"
	case "pixhost":
		if endpoint == "" {
			endpoint = "https://api.pixhost.to/images"
		}
		fieldName = "img"
		extra["content_type"] = "0"
		extra["max_th_size"] = "420"
	case "lsky":
		if endpoint == "" {
			return "", "", errors.New("请填写兰空图床 (Lsky Pro) API 上传地址")
		}
		if fieldName == "" {
			fieldName = "file"
		}
		authHeader = "Authorization"
		authPrefix = "Bearer "
	case "custom":
		if endpoint == "" {
			return "", "", errors.New("请填写自定义图床上传地址")
		}
		if fieldName == "" {
			fieldName = "file"
		}
	default:
		return "", "", fmt.Errorf("不支持的图床类型: %s", cfg.Provider)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range extra {
		_ = mw.WriteField(k, v)
	}
	part, err := mw.CreateFormFile(fieldName, shot.Filename)
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(shot.Data); err != nil {
		return "", "", err
	}
	if err := mw.Close(); err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FileBrowser-Nest/2.52")
	if authHeader != "" && strings.TrimSpace(cfg.APIKey) != "" {
		req.Header.Set(authHeader, authPrefix+strings.TrimSpace(cfg.APIKey))
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBytes))
		if len(msg) > 260 {
			msg = msg[:260]
		}
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	var parsed interface{}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", "", fmt.Errorf("无法解析图床响应: %s", string(respBytes))
	}

	switch provider {
	case "picx", "chevereto":
		imgURL = jsonPathString(parsed, "image.url")
		if imgURL == "" {
			imgURL = jsonPathString(parsed, "image.display_url")
		}
		thumbURL = jsonPathString(parsed, "image.thumb.url")
		if imgURL == "" {
			errMsg := jsonPathString(parsed, "error.message")
			if errMsg == "" {
				errMsg = "图床未返回图片地址"
			}
			return "", "", errors.New(errMsg)
		}
	case "pixhost":
		showURL := jsonPathString(parsed, "show_url")
		thURL := jsonPathString(parsed, "th_url")
		if showURL == "" && thURL == "" {
			return "", "", errors.New("Pixhost 未返回图片地址")
		}
		// Convert Pixhost thumbnail URL to direct image URL when possible
		if thURL != "" {
			imgURL = strings.Replace(thURL, "https://t", "https://img", 1)
			imgURL = strings.Replace(imgURL, "/thumbs/", "/images/", 1)
			thumbURL = thURL
		} else {
			imgURL = showURL
		}
	case "lsky":
		imgURL = jsonPathString(parsed, "data.links.url")
		if imgURL == "" {
			imgURL = jsonPathString(parsed, "data.url")
		}
		thumbURL = jsonPathString(parsed, "data.links.thumbnail_url")
		if imgURL == "" {
			errMsg := jsonPathString(parsed, "message")
			if errMsg == "" {
				errMsg = "兰空图床上传失败"
			}
			return "", "", errors.New(errMsg)
		}
	case "custom":
		if cfg.URLJsonPath != "" {
			imgURL = jsonPathString(parsed, cfg.URLJsonPath)
		}
		if imgURL == "" {
			for _, p := range []string{"image.url", "data.links.url", "data.url", "url", "link"} {
				if v := jsonPathString(parsed, p); v != "" {
					imgURL = v
					break
				}
			}
		}
		if imgURL == "" {
			return "", "", errors.New("未能从响应 JSON 中提取图片 URL，请检查 JSON 路径配置")
		}
	}

	return imgURL, thumbURL, nil
}

var screenshotUploadHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var req screenshotUploadRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	if len(req.ShotIDs) == 0 {
		return http.StatusBadRequest, errors.New("请至少选择一张截图")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	results := make([]uploadedShotResult, len(req.ShotIDs))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for i, id := range req.ShotIDs {
		shot, ok := globalShots.get(id, d.user.ID)
		if !ok {
			results[i] = uploadedShotResult{ShotID: id, Error: "截图缓存已过期，请重新截取"}
			continue
		}
		wg.Add(1)
		go func(idx int, s *cachedShot) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			imgURL, thumbURL, err := uploadSingleShot(ctx, s, req.Host)
			if err != nil {
				results[idx] = uploadedShotResult{
					ShotID:   s.ID,
					Filename: s.Filename,
					Error:    err.Error(),
				}
				return
			}
			results[idx] = uploadedShotResult{
				ShotID:   s.ID,
				Filename: s.Filename,
				URL:      imgURL,
				ThumbURL: thumbURL,
				BBCode:   fmt.Sprintf("[img]%s[/img]", imgURL),
				Markdown: fmt.Sprintf("![%s](%s)", s.Filename, imgURL),
			}
		}(i, shot)
	}
	wg.Wait()

	var bbLines, mdLines, directLines []string
	for _, res := range results {
		if res.URL != "" {
			bbLines = append(bbLines, res.BBCode)
			mdLines = append(mdLines, res.Markdown)
			directLines = append(directLines, res.URL)
		}
	}

	return renderJSON(w, r, screenshotUploadResponse{
		Results:  results,
		BBCode:   strings.Join(bbLines, "\n"),
		Markdown: strings.Join(mdLines, "\n"),
		Direct:   strings.Join(directLines, "\n"),
	})
})
