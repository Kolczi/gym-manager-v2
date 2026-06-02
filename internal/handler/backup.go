package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type BackupHandler struct {
	templates *template.Template
}

func NewBackupHandler(templates *template.Template) *BackupHandler {
	return &BackupHandler{templates: templates}
}

type BackupFile struct {
	Name string
	Size string
	Date string
}

func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	dir := backupDir()
	var backups []BackupFile

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "gym_manager_") && strings.HasSuffix(e.Name(), ".sql.gz") {
			info, _ := e.Info()
			size := "?"
			if info != nil {
				mb := float64(info.Size()) / 1024 / 1024
				if mb >= 1 {
					size = fmt.Sprintf("%.1f MB", mb)
				} else {
					size = fmt.Sprintf("%.0f KB", float64(info.Size())/1024)
				}
			}
			name := e.Name()
			date := ""
			parts := strings.TrimPrefix(name, "gym_manager_")
			parts = strings.TrimSuffix(parts, ".sql.gz")
			if len(parts) >= 15 {
				date = parts[6:8] + "." + parts[4:6] + "." + parts[0:4] + " " + parts[9:11] + ":" + parts[11:13]
			}
			backups = append(backups, BackupFile{Name: name, Size: size, Date: date})
		}
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].Name > backups[j].Name })

	data := map[string]any{
		"Title":           "Backup",
		"ContentTemplate": "backup_list",
		"Backups": backups,
		"Dir":     dir,
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "backup_list", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	scriptPath := findScript()
	if scriptPath == "" {
		http.Error(w, "backup script not found", 500)
		return
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "GYM_DB_NAME=gym_manager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, "Backup failed: "+string(out), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/backup")
	w.WriteHeader(http.StatusCreated)
}

func backupDir() string {
	if d := os.Getenv("GYM_BACKUP_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "gym-backups")
}

func findScript() string {
	candidates := []string{
		"scripts/backup.sh",
		filepath.Join(os.Getenv("HOME"), "projects/gym-manager-v2/scripts/backup.sh"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
