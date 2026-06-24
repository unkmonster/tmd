package api

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	log "github.com/sirupsen/logrus"
)

//go:embed web/*
var webFS embed.FS

var (
	themeMu      sync.RWMutex
	frontendTheme = "web1" // web1 或 web2，运行时热切换
	devWebRoot   string    // TMD_DEV=1 时设为本地 web 目录路径
)

func init() {
	if os.Getenv("TMD_DEV") == "1" {
		_, filename, _, ok := runtime.Caller(0)
		if ok {
			devWebRoot = filepath.Join(filepath.Dir(filename), "web")
		}
	}
}


func readFrontendFile(name string) ([]byte, error) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	if devWebRoot != "" {
		return os.ReadFile(filepath.Join(devWebRoot, frontendTheme, name))
	}
	return webFS.ReadFile("web/" + frontendTheme + "/" + name)
}

func setFrontendTheme(theme string) bool {
	themeMu.Lock()
	defer themeMu.Unlock()
	if theme == "" || strings.ContainsAny(theme, "/\\..") {
		return false
	}
	// 验证目录存在（开发模式走本地 FS，否则走 embed FS）
	if devWebRoot != "" {
		info, err := os.Stat(filepath.Join(devWebRoot, theme, "index.html"))
		if err != nil || info.IsDir() {
			return false
		}
	} else {
		if _, err := webFS.ReadFile("web/" + theme + "/index.html"); err != nil {
			return false
		}
	}
	frontendTheme = theme
	return true
}

func getFrontendTheme() string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return frontendTheme
}

// listThemes 列出所有可用主题目录（开发模式走本地 FS，否则走 embed FS）
func listThemes() []string {
	if devWebRoot != "" {
		entries, err := os.ReadDir(devWebRoot)
		if err != nil {
			return nil
		}
		var themes []string
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(devWebRoot, e.Name(), "index.html")); err == nil {
					themes = append(themes, e.Name())
				}
			}
		}
		return themes
	}
	entries, err := webFS.ReadDir("web")
	if err != nil {
		return nil
	}
	var themes []string
	for _, e := range entries {
		if e.IsDir() {
			// 验证目录中有 index.html
			if _, err := webFS.ReadFile("web/" + e.Name() + "/index.html"); err == nil {
				themes = append(themes, e.Name())
			}
		}
	}
	return themes
}

// handleWeb 返回 Web 管理页面
func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	data, err := readFrontendFile("index.html")
	if err != nil {
		log.Errorf("[web] Failed to load index.html: %v", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to load web page")
		return
	}

	// 注入主题切换器（所有前端通用，无需修改各主题的 HTML）
	switcher := themeSwitcherHTML()
	data = bytes.Replace(data, []byte("</body>"), []byte(switcher+"</body>"), 1)

	s.writeCachedContent(w, r, data, "text/html; charset=utf-8", "no-cache")
}

// handleStatic 静态文件服务
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	reqPath := r.PathValue("path")
	if reqPath == "" {
		http.NotFound(w, r)
		return
	}

	// 使用 path.Clean 来规范化路径，自动处理掉所有的 "." 和 ".." 以及多余的斜杠
	cleanPath := path.Clean("/" + reqPath)

	// 确保规范化后的路径不会逃逸出根目录
	if strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	cleanPath = strings.TrimPrefix(cleanPath, "/")

	data, err := readFrontendFile(cleanPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(cleanPath, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".js"):
		contentType = "application/javascript; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".json"):
		contentType = "application/json"
	case strings.HasSuffix(cleanPath, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(cleanPath, ".jpg"), strings.HasSuffix(cleanPath, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(cleanPath, ".svg"):
		contentType = "image/svg+xml"
	}

	s.writeCachedContent(w, r, data, contentType, "no-cache")
}

func (s *Server) writeCachedContent(w http.ResponseWriter, r *http.Request, data []byte, contentType, cacheControl string) {
	etag := contentETag(data)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)

	if ifNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if _, err := w.Write(data); err != nil {
		return
	}
}

func contentETag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func ifNoneMatch(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

// themeSwitcherHTML 返回一个在所有主题中通用的浮动主题切换器 UI
// 通过 handleWeb 注入到各主题的 HTML 中，无需修改主题文件
func themeSwitcherHTML() string {
	return `<div id="tmd-theme-switcher" style="position:fixed;bottom:16px;left:16px;z-index:9999;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:12px;line-height:1.4">
<style>
#tmd-theme-switcher *{box-sizing:border-box;margin:0;padding:0}
#tmd-theme-switcher .ts-btn{width:36px;height:36px;border-radius:50%;border:1px solid rgba(255,255,255,.2);background:rgba(20,20,30,.85);backdrop-filter:blur(8px);color:#fff;cursor:pointer;display:flex;align-items:center;justify-content:center;font-size:16px;transition:transform .2s,box-shadow .2s;box-shadow:0 2px 12px rgba(0,0,0,.3)}
#tmd-theme-switcher .ts-btn:hover{transform:scale(1.1);box-shadow:0 4px 16px rgba(0,0,0,.4)}
#tmd-theme-switcher .ts-panel{display:none;margin-top:8px;background:rgba(20,20,30,.92);backdrop-filter:blur(12px);border:1px solid rgba(255,255,255,.12);border-radius:10px;padding:12px;min-width:160px;box-shadow:0 8px 32px rgba(0,0,0,.4);color:#ddd}
#tmd-theme-switcher .ts-panel.open{display:block}
#tmd-theme-switcher .ts-label{font-size:10px;text-transform:uppercase;letter-spacing:.5px;color:#888;margin-bottom:6px}
#tmd-theme-switcher .ts-current{font-size:13px;font-weight:600;color:#fff;margin-bottom:8px}
#tmd-theme-switcher .ts-list{display:flex;flex-direction:column;gap:4px}
#tmd-theme-switcher .ts-opt{display:flex;align-items:center;gap:6px;padding:5px 8px;border-radius:6px;border:none;background:transparent;color:#bbb;cursor:pointer;font-size:12px;text-align:left;transition:background .15s,color .15s}
#tmd-theme-switcher .ts-opt:hover{background:rgba(255,255,255,.08);color:#fff}
#tmd-theme-switcher .ts-opt.active{color:#fff;background:rgba(37,99,235,.3)}
#tmd-theme-switcher .ts-opt .ts-dot{width:6px;height:6px;border-radius:50%;background:currentColor;flex-shrink:0}
#tmd-theme-switcher .ts-opt.active .ts-dot{background:#3b82f6}
</style>
<div class="ts-btn" id="ts-toggle" onclick="toggleThemeSwitcher()" title="Switch theme">🎨</div>
<div class="ts-panel" id="ts-panel">
<div class="ts-label">Theme</div>
<div class="ts-current" id="ts-current">loading...</div>
<div class="ts-list" id="ts-list"></div>
</div>
<script>
async function toggleThemeSwitcher(){
var p=document.getElementById('ts-panel');p.classList.toggle('open');
if(p.classList.contains('open')&&!window._tsLoaded){window._tsLoaded=1;loadThemes()}
}
async function loadThemes(){
try{
var r=await fetch('/api/v1/config/themes');var d=await r.json();
if(!d.success)throw new Error(d.error);
var themes=d.data.themes;var cur=d.data.current;
document.getElementById('ts-current').textContent='Current: '+cur;
var list=document.getElementById('ts-list');list.innerHTML='';
themes.forEach(function(t){
var b=document.createElement('button');
b.className='ts-opt'+(t===cur?' active':'');
b.innerHTML='<span class="ts-dot"></span>'+t;
b.onclick=function(){switchTheme(t)};
list.appendChild(b);
})
}catch(e){document.getElementById('ts-current').textContent='Error loading themes'}
}
async function switchTheme(t){
var r=await fetch('/api/v1/config/theme',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({theme:t})});
var d=await r.json();
if(d.success)window.location.reload(true)}
</script>
</div>`
}
