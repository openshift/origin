package gitserver

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func hooksHandler(config *Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		segments := strings.Split(r.URL.Path[1:], "/")
		for _, s := range segments {
			if len(s) == 0 || s == "." || s == ".." {
				http.NotFound(w, r)
				return
			}
		}
		if !config.AllowPush {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		switch len(segments) {
		case 2:
			path := filepath.Join(config.Home, segments[0], "hooks", segments[1])
			if segments[0] == "hooks" {
				path = filepath.Join(config.HookDirectory, segments[1])
			}

			switch r.Method {
			// TODO: support HEAD or prevent GET for security
			case "GET":
				w.Header().Set("Content-Type", "text/plain")
				http.ServeFile(w, r, path)

			case "DELETE":
				if err := os.Remove(path); err != nil {
					log.Printf("error: attempted to remove %s: %v", path, err)
				}
				w.WriteHeader(http.StatusNoContent)

			case "PUT":
				if stat, err := os.Stat(path); err == nil {
					if stat.IsDir() || (stat.Mode()&0111) == 0 {
						http.Error(w, fmt.Errorf("only executable hooks can be changed: %v", stat).Error(), http.StatusInternalServerError)
						return
					}
					// unsymlink and overwrite
					if (stat.Mode() & os.ModeSymlink) != 0 {
						os.Remove(path)
					}
				}
				f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0750)
				if err != nil {
					http.Error(w, fmt.Errorf("unable to open hook file: %v", err).Error(), http.StatusInternalServerError)
					return
				}
				defer f.Close()
				max := config.MaxHookBytes + 1
				body := io.LimitReader(r.Body, max)
				buf := make([]byte, max)
				n, err := io.ReadFull(body, buf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					http.Error(w, fmt.Errorf("unable to read hook: %v", err).Error(), http.StatusInternalServerError)
					return
				}
				if int64(n) == max {
					http.Error(w, fmt.Errorf("hook was too long, truncated to %d bytes", config.MaxHookBytes).Error(), 422)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				if _, err := f.Write(buf[:n]); err != nil {
					http.Error(w, fmt.Errorf("unable to write hook: %v", err).Error(), http.StatusInternalServerError)
					return
				}

			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}

		default:
			http.NotFound(w, r)
		}
	})
}
