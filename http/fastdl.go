package fbhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

const (
	fastDLTokenIssuer = "File Browser FastDL"
	fastDLDefaultTTL  = 6 * time.Hour
	fastDLMaxTTL      = 48 * time.Hour
)

type fastDLClaims struct {
	UserID uint     `json:"uid"`
	Paths  []string `json:"paths"`
	Mode   string   `json:"mode"` // "file", "vtar", "manifest"
	jwt.RegisteredClaims
}

type fastDLSignRequest struct {
	Paths    []string `json:"paths"`
	Mode     string   `json:"mode"`     // optional: auto-detects "file" vs "vtar"
	TTLHours int      `json:"ttlHours"` // optional: default 6, max 48
}

type fastDLSignResponse struct {
	Mode             string `json:"mode"`
	Filename         string `json:"filename"`
	URL              string `json:"url"`
	ExpiresAt        string `json:"expiresAt"`
	FileCount        int    `json:"fileCount"`
	TotalSize        int64  `json:"totalSize"`
	Aria2Cmd         string `json:"aria2Cmd"`
	CurlCmd          string `json:"curlCmd"`
	ManifestURL      string `json:"manifestUrl,omitempty"`
	Aria2ManifestCmd string `json:"aria2ManifestCmd,omitempty"`
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = strings.TrimSpace(strings.Split(fwdHost, ",")[0])
	}
	return scheme + "://" + host
}

func signFastDLToken(key []byte, uid uint, paths []string, mode string, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	claims := &fastDLClaims{
		UserID: uid,
		Paths:  paths,
		Mode:   mode,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fastDLTokenIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tk.SignedString(key)
	return signed, exp, err
}

func shellQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func deriveDownloadName(paths []string, isDirOrMulti bool, ext string) string {
	if len(paths) == 1 {
		base := filepath.Base(paths[0])
		if base == "." || base == "/" || base == "" {
			base = "archive"
		}
		if isDirOrMulti {
			return base + ext
		}
		return base
	}
	commonDir := fileutils.CommonPrefix(filepath.Separator, paths...)
	base := filepath.Base(commonDir)
	if base == "." || base == "/" || base == "" {
		base = "files"
	}
	return "_" + base + ext
}

var fastDLSignHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
	}

	var req fastDLSignRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	if len(req.Paths) == 0 {
		return http.StatusBadRequest, fmt.Errorf("paths is required")
	}

	cleaned := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		cp := slashClean(p)
		if !d.Check(cp) {
			return http.StatusForbidden, nil
		}
		if _, err := d.user.Fs.Stat(cp); err != nil {
			return errToStatus(err), err
		}
		cleaned = append(cleaned, cp)
	}

	ttl := fastDLDefaultTTL
	if req.TTLHours > 0 {
		ttl = time.Duration(req.TTLHours) * time.Hour
		if ttl > fastDLMaxTTL {
			ttl = fastDLMaxTTL
		}
	}

	mode := req.Mode
	isSingleFile := false
	if len(cleaned) == 1 {
		st, err := d.user.Fs.Stat(cleaned[0])
		if err != nil {
			return errToStatus(err), err
		}
		isSingleFile = !st.IsDir()
	}

	if mode == "" {
		if isSingleFile {
			mode = "file"
		} else {
			mode = "vtar"
		}
	}
	if mode != "file" && mode != "vtar" && mode != "manifest" {
		return http.StatusBadRequest, fmt.Errorf("unsupported mode: %s", mode)
	}
	if mode == "file" && !isSingleFile {
		mode = "vtar"
	}

	var fileCount int
	var totalSize int64
	var filename string

	if mode == "file" {
		st, err := d.user.Fs.Stat(cleaned[0])
		if err != nil {
			return errToStatus(err), err
		}
		fileCount = 1
		totalSize = st.Size()
		filename = deriveDownloadName(cleaned, false, "")
	} else {
		commonDir := fileutils.CommonPrefix(filepath.Separator, cleaned...)
		vt, err := NewVirtualTar(d, cleaned, commonDir)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		fileCount = vt.FileCount()
		totalSize = vt.Size()
		_ = vt.Close()
		filename = deriveDownloadName(cleaned, true, ".tar")
	}

	token, exp, err := signFastDLToken(d.settings.Key, d.user.ID, cleaned, mode, ttl)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	origin := requestOrigin(r)
	basePrefix := strings.TrimSuffix(d.server.BaseURL, "/")
	dlURL := fmt.Sprintf("%s%s/api/fastdl/%s/%s", origin, basePrefix, token, url.PathEscape(filename))

	resp := fastDLSignResponse{
		Mode:      mode,
		Filename:  filename,
		URL:       dlURL,
		ExpiresAt: exp.Format(time.RFC3339),
		FileCount: fileCount,
		TotalSize: totalSize,
		Aria2Cmd:  fmt.Sprintf("aria2c -x 16 -s 16 -k 1M -c --file-allocation=none -o %s %s", shellQuote(filename), shellQuote(dlURL)),
		CurlCmd:   fmt.Sprintf("curl -L -C - -o %s %s", shellQuote(filename), shellQuote(dlURL)),
	}

	if !isSingleFile {
		manifestName := strings.TrimSuffix(filename, ".tar") + ".aria2.txt"
		mToken, _, err := signFastDLToken(d.settings.Key, d.user.ID, cleaned, "manifest", ttl)
		if err == nil {
			mURL := fmt.Sprintf("%s%s/api/fastdl/%s/%s", origin, basePrefix, mToken, url.PathEscape(manifestName))
			resp.ManifestURL = mURL
			resp.Aria2ManifestCmd = fmt.Sprintf("curl -fsSL %s | aria2c -i - -j 6 -x 16 -s 16 -k 1M -c --file-allocation=none", shellQuote(mURL))
		}
	}

	return renderJSON(w, r, resp)
})

func fastDLServeHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	vars := mux.Vars(r)
	rawToken := vars["token"]
	if rawToken == "" {
		return http.StatusUnauthorized, nil
	}

	var claims fastDLClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(fastDLTokenIssuer),
	)
	tk, err := parser.ParseWithClaims(rawToken, &claims, func(_ *jwt.Token) (interface{}, error) {
		return d.settings.Key, nil
	})
	if err != nil || !tk.Valid {
		return http.StatusUnauthorized, nil
	}

	d.user, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, claims.UserID)
	if err != nil {
		return http.StatusUnauthorized, nil
	}
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
	}

	for _, p := range claims.Paths {
		if !d.Check(p) {
			return http.StatusForbidden, nil
		}
	}

	switch claims.Mode {
	case "file":
		if len(claims.Paths) != 1 {
			return http.StatusBadRequest, nil
		}
		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:         d.user.Fs,
			Path:       claims.Paths[0],
			Modify:     d.user.Perm.Modify,
			Expand:     false,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}
		return rawFileHandler(w, r, file)

	case "vtar":
		commonDir := fileutils.CommonPrefix(filepath.Separator, claims.Paths...)
		vt, err := NewVirtualTar(d, claims.Paths, commonDir)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer vt.Close()

		filename := deriveDownloadName(claims.Paths, true, ".tar")
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(filename))
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("ETag", vt.ETag())
		w.Header().Set("Cache-Control", "private")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, filename, vt.ModTime(), vt)
		return 0, nil

	case "manifest":
		commonDir := fileutils.CommonPrefix(filepath.Separator, claims.Paths...)
		var sources []rawTarSource
		for _, p := range claims.Paths {
			sub, err := collectVirtualTarSources(d, p, commonDir)
			if err != nil {
				continue
			}
			sources = append(sources, sub...)
		}

		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl < time.Minute {
			ttl = time.Minute
		}
		origin := requestOrigin(r)
		basePrefix := strings.TrimSuffix(d.server.BaseURL, "/")
		rootName := strings.TrimSuffix(deriveDownloadName(claims.Paths, true, ".tar"), ".tar")

		var sb strings.Builder
		for _, src := range sources {
			if src.info.IsDir() {
				continue
			}
			fToken, _, err := signFastDLToken(d.settings.Key, d.user.ID, []string{src.fsPath}, "file", ttl)
			if err != nil {
				continue
			}
			base := filepath.Base(src.nameInArchive)
			relDir := filepath.ToSlash(filepath.Dir(src.nameInArchive))
			if relDir == "." {
				relDir = rootName
			} else {
				relDir = rootName + "/" + relDir
			}
			fileURL := fmt.Sprintf("%s%s/api/fastdl/%s/%s", origin, basePrefix, fToken, url.PathEscape(base))
			sb.WriteString(fileURL)
			sb.WriteString("\n")
			sb.WriteString("  dir=")
			sb.WriteString(relDir)
			sb.WriteString("\n")
			sb.WriteString("  out=")
			sb.WriteString(base)
			sb.WriteString("\n")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(sb.String()))
		return 0, nil

	default:
		return http.StatusBadRequest, nil
	}
}
