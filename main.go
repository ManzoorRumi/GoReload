package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var lastChange time.Time

func main() {
	files := []string{}
	filepath.Walk(".", func(p string, i os.FileInfo, _ error) error {
		if i != nil && !i.IsDir() && !strings.HasPrefix(p, ".") {
			files = append(files, p)
		}
		return nil
	})

	fmt.Println("Watching files in folder:")
	for _, f := range files {
		fmt.Println("  \033[90m○\033[0m", f)
	}
	fmt.Print("\nPress Ctrl+C to exit.")

	go watch()
	mux := http.NewServeMux()
	mux.HandleFunc("/lr.js", serveJS)
	mux.HandleFunc("/lr-check", checkReload)
	mux.HandleFunc("/", serve)
	go func() {
		time.Sleep(500 * time.Millisecond)
		open("http://localhost:8080")
	}()
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func serve(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}

	if strings.HasSuffix(p, ".html") {
		data, err := os.ReadFile("." + p)
		if err == nil {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Cache-Control", "no-store, must-revalidate")
			html := string(data)
			// Inject cache-busting for CSS and JS files
			html = strings.ReplaceAll(html, `.css"`, `.css?t=`+fmt.Sprint(time.Now().Unix())+`"`)
			html = strings.ReplaceAll(html, `.js"`, `.js?t=`+fmt.Sprint(time.Now().Unix())+`"`)
			script := `<script src="/lr.js"></script>`
			if strings.Contains(html, "</body>") {
				html = strings.Replace(html, "</body>", script+"</body>", 1)
			} else {
				html = html + script
			}
			w.Write([]byte(html))
			return
		}
	}

	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	http.ServeFile(w, r, "."+p)
}

func serveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	js := `
let last = 0;
setInterval(async () => {
  const r = await fetch('/lr-check');
  const t = await r.text();
  if (last && t !== last) {
    location.reload(true);
  }
  last = t;
}, 500);
`
	w.Write([]byte(js))
}

func checkReload(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(lastChange.Format(time.RFC3339Nano)))
}

func watch() {
	m := map[string]time.Time{}
	// Initialize with current times
	filepath.Walk(".", func(p string, i os.FileInfo, _ error) error {
		if i != nil && !i.IsDir() && !strings.HasPrefix(p, ".") {
			m[p] = i.ModTime()
		}
		return nil
	})

	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		filepath.Walk(".", func(p string, i os.FileInfo, _ error) error {
			if i != nil && !i.IsDir() && !strings.HasPrefix(p, ".") {
				t := i.ModTime()
				if prev, ok := m[p]; ok && t.After(prev) {
					lastChange = time.Now()
					fmt.Printf("\r\033[K  \033[33m⚡\033[0m \033[1m%s\033[0m \033[32m✓\033[0m", p)
				}
				m[p] = t
			}
			return nil
		})
	}
}

func open(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("powershell", "-Command", "Start-Process", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Run()
}
