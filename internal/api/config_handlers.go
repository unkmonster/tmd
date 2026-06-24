package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/unkmonster/tmd/internal/config"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	resp := ConfigResponse{
		RootPath:           s.config.RootPath,
		MaxDownloadRoutine: s.config.MaxDownloadRoutine,
		MaxFileNameLen:     s.config.MaxFileNameLen,
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}

func (s *Server) handleGetConfigRaw(w http.ResponseWriter, _ *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")
	data, err := os.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			defaultConf := config.Config{}
			yamlData, err := config.MarshalConf(&defaultConf)
			if err != nil {
				s.writeError(w, http.StatusInternalServerError, "Failed to marshal default config: "+err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigRawResponse{
				Content: string(yamlData),
				Path:    confPath,
				Exists:  false,
			}))
			return
		}
		s.writeError(w, http.StatusInternalServerError, "Failed to read config: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigRawResponse{
		Content: string(data),
		Path:    confPath,
		Exists:  true,
	}))
}

func (s *Server) handleUpdateConfigRaw(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	testConf, err := config.ParseConfYAML([]byte(req.Content))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid config: "+err.Error())
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupName, err := config.CreateBackup(confPath)
	if err != nil {
		log.Warnf("Failed to create config backup: %v", err)
	}

	if err := config.WriteConf(confPath, testConf); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to write config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via raw editor")

	// 检测 api_key 是否被修改，以确定生效消息
	oldAPIKey := s.config.APIKey
	*s.config = *testConf
	apiKeyChanged := oldAPIKey != s.config.APIKey

	yamlPreview, err := config.MarshalConf(testConf)
	if err != nil {
		log.Warnf("Failed to marshal yaml preview: %v", err)
	}

	message := "Configuration saved successfully. Please restart TMD manually for changes to take effect."
	if apiKeyChanged {
		message = "Configuration saved. API Key has been updated and takes effect immediately. Other changes may require a restart."
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      message,
		"backup":       backupName,
		"yaml_preview": string(yamlPreview),
	}))
}

type configFieldMeta struct {
	Name        string
	Label       string
	Prompt      string
	Default     string
	Type        string
	Placeholder string
	Required    bool
	Group       string
	IsSensitive bool
}

func buildConfigFieldMeta(fieldDefs []config.FieldDef) []configFieldMeta {
	metaCache := make([]configFieldMeta, len(fieldDefs))
	for i, fd := range fieldDefs {
		meta := configFieldMeta{
			Name:     fd.Name,
			Prompt:   fd.Prompt,
			Default:  fd.Default,
			Required: fd.Default == "",
		}
		switch fd.Name {
		case "root_path":
			meta.Label, meta.Type, meta.Group = "Storage Path", "text", "basic"
		case "auth_token":
			meta.Label, meta.Type, meta.Group = "Auth Token", "password", "cookie"
			meta.IsSensitive = true
		case "ct0":
			meta.Label, meta.Type, meta.Group = "CT0", "password", "cookie"
			meta.IsSensitive = true
		case "max_download_routine":
			meta.Label, meta.Type, meta.Group = "Max Concurrent Downloads", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("1-100, default %s", fd.Default)
		case "max_file_name_len":
			meta.Label, meta.Type, meta.Group = "Max File Name Length", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("%d-%d, default %s", config.MinFileNameLen, config.MaxFileNameLen, fd.Default)
		case "proxy_url":
			meta.Label, meta.Type, meta.Group, meta.Placeholder = "Proxy URL", "text", "advanced", "http://127.0.0.1:7897 or leave empty"
		case "api_key":
			meta.Label, meta.Type, meta.Group = "API Key", "password", "security"
			meta.IsSensitive = true
			meta.Placeholder = "Leave empty to disable HTTP auth"
		default:
			meta.Label, meta.Type, meta.Group = fd.Name, "text", "basic"
		}
		metaCache[i] = meta
	}
	return metaCache
}

func buildConfigFieldItems(conf *config.Config) []ConfigFieldItem {
	fieldDefs := config.GetFieldDefs()
	metaCache := buildConfigFieldMeta(fieldDefs)
	items := make([]ConfigFieldItem, 0, len(metaCache))
	for i, m := range metaCache {
		val := config.GetFieldValue(conf, fieldDefs[i])
		item := ConfigFieldItem{
			Name:        m.Name,
			Label:       m.Label,
			Prompt:      m.Prompt,
			Value:       val,
			Default:     m.Default,
			Type:        m.Type,
			Placeholder: m.Placeholder,
			Required:    m.Required,
			Group:       m.Group,
		}
		if m.IsSensitive {
			item.Value = maskSensitive(val)
		}
		items = append(items, item)
	}
	return items
}

func (s *Server) handleGetConfigFields(w http.ResponseWriter, _ *http.Request) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")
	exists := true

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		exists = false
	}

	currentConf := s.config
	if currentConf == nil {
		var err error
		currentConf, err = config.ReadConf(confPath)
		if err != nil {
			if os.IsNotExist(err) {
				currentConf = &config.Config{}
			} else {
				s.writeError(w, http.StatusInternalServerError, "Failed to read config: "+err.Error())
				return
			}
		}
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ConfigFieldsResponse{
		Exists: exists,
		Fields: buildConfigFieldItems(currentConf),
	}))
}

func (s *Server) handleSaveConfigFields(w http.ResponseWriter, r *http.Request) {
	var req ConfigFieldsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	newConf := &config.Config{}
	if s.config != nil {
		*newConf = *s.config
	}

	fieldDefs := config.GetFieldDefs()

	for _, fd := range fieldDefs {
		userVal, ok := req.Fields[fd.Name]

		// __CLEAR__ sentinel: explicitly set field to empty (e.g. clearing api_key)
		if ok && userVal == "__CLEAR__" {
			userVal = ""
		} else if !ok || strings.TrimSpace(userVal) == "" || userVal == "__KEEP_OLD__" {
			userVal = config.GetFieldValue(newConf, fd)
			if userVal == "" {
				userVal = fd.Default
			}
		}

		// 拒绝掩码值：防止前端误发送 maskSensitive 输出的占位文本（如 "abc•••xyz"）
		if isMaskedValue(userVal) {
			s.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("Field %q contains a masked placeholder value. Leave it empty to keep the current value, or enter the actual value.", fd.Name))
			return
		}

		if err := fd.Setter(newConf, userVal); err != nil {
			s.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("Invalid field %s: %s", fd.Name, err.Error()))
			return
		}
	}

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupName, err := config.CreateBackup(confPath)
	if err != nil {
		log.Warnf("Failed to create config backup: %v", err)
	}

	if err := config.WriteConf(confPath, newConf); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via structured form")

	// 检测 api_key 是否被修改，以确定生效消息
	// 使用生效值比较而非原始表单值，避免 web2 空字符串误判为变更
	apiKeyChanged := newConf.APIKey != s.config.APIKey

	*s.config = *newConf

	yamlPreview, err := config.MarshalConf(newConf)
	if err != nil {
		log.Warnf("Failed to marshal yaml preview: %v", err)
	}

	message := "Configuration saved successfully. Please restart TMD manually for changes to take effect."
	if apiKeyChanged {
		message = "Configuration saved. API Key has been updated and takes effect immediately."
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      message,
		"backup":       backupName,
		"yaml_preview": string(yamlPreview),
		"fields":       buildConfigFieldItems(newConf),
	}))
}

func maskSensitive(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= 6 {
		return "***"
	}
	return string(runes[:3]) + "•••" + string(runes[len(runes)-3:])
}

// maskedValueMarker 是 maskSensitive 输出的掩码标记字符（U+2022 BULLET）。
const maskedValueMarker = "•••"

// isMaskedValue 检查字符串是否为 maskSensitive 产生的掩码值
//（如 "abc•••xyz" 或 "***"），用于阻止用户误将占位文本当作真实值提交。
func isMaskedValue(s string) bool {
	return s == "***" || strings.Contains(s, maskedValueMarker)
}
