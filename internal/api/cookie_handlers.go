package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
		log.Errorf("[cookies] Failed to read cookies: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read cookies", err.Error())
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
		log.Errorf("[cookies] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	var testCookies []*config.Cookie
	if err := yaml.Unmarshal([]byte(req.Content), &testCookies); err != nil {
		log.Errorf("[cookies] Invalid YAML format: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid YAML format", err.Error())
		return
	}
	for i, c := range testCookies {
		if c == nil {
			continue
		}
		if strings.TrimSpace(c.AuthToken) == "" && strings.TrimSpace(c.Ct0) == "" {
			s.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("Account #%d: Auth Token and CT0 cannot both be empty", i+1))
			return
		}
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	backupName, err := config.CreateBackup(cookiesPath)
	if err != nil {
		log.Warnf("[cookies] Failed to create cookies backup: %v", err)
	}

	if err := config.WriteAdditionalCookies(cookiesPath, testCookies); err != nil {
		log.Errorf("[cookies] Failed to write cookies: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to write cookies", err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via raw editor")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":  backupName,
	}))
}

func (s *Server) handleGetCookies(w http.ResponseWriter, _ *http.Request) {
	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")
	exists := true

	if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
		exists = false
	}

	cookies, err := config.ReadAdditionalCookies(cookiesPath)
	if err != nil {
		log.Errorf("[cookies] Failed to read cookies: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read cookies", err.Error())
		return
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
		log.Errorf("[cookies] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	cookiesPath := filepath.Join(s.appRootPath, "additional_cookies.yaml")

	existingCookies, err := config.ReadAdditionalCookies(cookiesPath)
	if err != nil {
		log.Errorf("[cookies] Failed to read existing cookies: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read existing cookies", err.Error())
		return
	}

	cookies := make([]*config.Cookie, 0, len(req.Cookies))
	for i, c := range req.Cookies {
		sourceIndex := i
		if c.Index != nil {
			sourceIndex = *c.Index
		}

		authToken, err := resolveCookieSaveValue(c.AuthToken, existingCookies, sourceIndex, func(cookie *config.Cookie) string {
			return cookie.AuthToken
		})
		if err != nil {
			log.Debugf("[cookies] Account #%d Auth Token: %v", i+1, err)
			s.writeErrorDetail(w, http.StatusBadRequest, fmt.Sprintf("Account #%d: Invalid Auth Token", i+1), err.Error())
			return
		}
		ct0, err := resolveCookieSaveValue(c.Ct0, existingCookies, sourceIndex, func(cookie *config.Cookie) string {
			return cookie.Ct0
		})
		if err != nil {
			log.Debugf("[cookies] Account #%d CT0: %v", i+1, err)
			s.writeErrorDetail(w, http.StatusBadRequest, fmt.Sprintf("Account #%d: Invalid CT0", i+1), err.Error())
			return
		}

		if strings.TrimSpace(authToken) == "" && strings.TrimSpace(ct0) == "" {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Account #%d: Auth Token and CT0 cannot both be empty", i+1))
			return
		}

		cookies = append(cookies, &config.Cookie{
			AuthToken: authToken,
			Ct0:       ct0,
		})
	}
	backupName, err := config.CreateBackup(cookiesPath)
	if err != nil {
		log.Warnf("[cookies] Failed to create cookies backup: %v", err)
	}

	if err := config.WriteAdditionalCookies(cookiesPath, cookies); err != nil {
		log.Errorf("[cookies] Failed to save cookies: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save cookies", err.Error())
		return
	}

	log.Infoln("[WebUI] additional cookies saved via structured form")

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Additional cookies saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":  backupName,
	}))
}

func resolveCookieSaveValue(value string, existingCookies []*config.Cookie, sourceIndex int, get func(*config.Cookie) string) (string, error) {
	if value != "__KEEP_OLD__" {
		// 拒绝掩码值：防止前端误发送 maskSensitive 输出的占位文本
		if isMaskedValue(value) {
			return "", fmt.Errorf("contains a masked placeholder value (use the actual value or leave empty to keep current)")
		}
		return value, nil
	}
	if sourceIndex < 0 || sourceIndex >= len(existingCookies) || existingCookies[sourceIndex] == nil {
		return "", fmt.Errorf("cannot keep original value: account not found")
	}
	return get(existingCookies[sourceIndex]), nil
}
