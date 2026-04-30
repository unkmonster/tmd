package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/unkmonster/tmd/internal/config"
)

func (s *Server) handleGetCookiesRaw(w http.ResponseWriter, _ *http.Request) {
	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")
	data, err := os.ReadFile(cookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(CookiesRawResponse{
				Content: "",
				Path:    cookiesPath,
				Exists:  false,
			}))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "Failed to read cookies: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(CookiesRawResponse{
		Content: string(data),
		Path:    cookiesPath,
		Exists:  true,
	}))
}

func (s *Server) handleUpdateCookiesRaw(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	var testCookies []*config.Cookie
	if err := yaml.Unmarshal([]byte(req.Content), &testCookies); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid YAML format: "+err.Error())
		return
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	backupPath := cookiesPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(cookiesPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0600); writeErr != nil {
			log.Warnf("Failed to create cookies backup: %v", writeErr)
		}
	}

	if err := config.WriteAdditionalCookies(cookiesPath, testCookies); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to write cookies: "+err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via raw editor")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":  filepath.Base(backupPath),
	}))
}

func (s *Server) handleCookies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetCookies(w, r)
	case http.MethodPut:
		s.handleSaveCookies(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) handleGetCookies(w http.ResponseWriter, _ *http.Request) {
	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")
	exists := true

	cookies, err := config.ReadAdditionalCookies(cookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
			cookies = nil
		} else {
			s.writeError(w, http.StatusInternalServerError, "Failed to read cookies: "+err.Error())
			return
		}
	}

	items := make([]CookieItem, 0, len(cookies))
	for i, c := range cookies {
		items = append(items, CookieItem{
			Index:     i,
			AuthToken: maskSensitive(c.AuthToken),
			Ct0:       maskSensitive(c.Ct0),
		})
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"exists": exists,
		"items":  items,
	}))
}

func (s *Server) handleSaveCookies(w http.ResponseWriter, r *http.Request) {
	var req CookiesSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	existingCookies, err := config.ReadAdditionalCookies(cookiesPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to read existing cookies: "+err.Error())
		return
	}

	cookies := make([]*config.Cookie, 0, len(req.Cookies))
	for i, c := range req.Cookies {
		authToken := c["auth_token"]
		ct0 := c["ct0"]

		if strings.TrimSpace(authToken) == "" && strings.TrimSpace(ct0) == "" {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("账户 #%d 的 Auth Token 和 CT0 不能同时为空", i+1))
			return
		}

		if authToken == "__KEEP_OLD__" && existingCookies != nil && i < len(existingCookies) {
			authToken = existingCookies[i].AuthToken
		}
		if ct0 == "__KEEP_OLD__" && existingCookies != nil && i < len(existingCookies) {
			ct0 = existingCookies[i].Ct0
		}

		cookies = append(cookies, &config.Cookie{
			AuthToken: authToken,
			Ct0:       ct0,
		})
	}

	backupPath := cookiesPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(cookiesPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0600); writeErr != nil {
			log.Warnf("Failed to create cookies backup: %v", writeErr)
		}
	}

	if err := config.WriteAdditionalCookies(cookiesPath, cookies); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to save cookies: "+err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via structured form")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":  filepath.Base(backupPath),
	}))
}
