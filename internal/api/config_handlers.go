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
			yamlData, err := yaml.Marshal(defaultConf)
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

	var testConf config.Config
	if err := yaml.Unmarshal([]byte(req.Content), &testConf); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid YAML format: "+err.Error())
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupPath := confPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(confPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0600); writeErr != nil {
			log.Warnf("Failed to create config backup: %v", writeErr)
		}
	}

	if err := os.WriteFile(confPath, []byte(req.Content), 0600); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to write config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via raw editor")

	*s.config = testConf

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      "Configuration saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":       filepath.Base(backupPath),
		"yaml_preview": req.Content,
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
			meta.Label, meta.Type, meta.Group = "存储路径", "text", "basic"
		case "auth_token":
			meta.Label, meta.Type, meta.Group = "Auth Token", "password", "cookie"
			meta.IsSensitive = true
		case "ct0":
			meta.Label, meta.Type, meta.Group = "CT0", "password", "cookie"
			meta.IsSensitive = true
		case "max_download_routine":
			meta.Label, meta.Type, meta.Group = "最大并发下载", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("1-100, 默认 %s", fd.Default)
		case "max_file_name_len":
			meta.Label, meta.Type, meta.Group = "最大文件名长度", "number", "advanced"
			meta.Placeholder = fmt.Sprintf("%d-%d, 默认 %s", config.MinFileNameLen, config.MaxFileNameLen, fd.Default)
		case "proxy_url":
			meta.Label, meta.Type, meta.Group, meta.Placeholder = "代理地址", "text", "advanced", "http://127.0.0.1:7897 或留空"
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

	currentConf := s.config
	if currentConf == nil {
		var err error
		currentConf, err = config.ReadConf(confPath)
		if err != nil {
			if os.IsNotExist(err) {
				exists = false
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
		if !ok || strings.TrimSpace(userVal) == "" || userVal == "__KEEP_OLD__" {
			userVal = config.GetFieldValue(newConf, fd)
			if userVal == "" {
				userVal = fd.Default
			}
		}

		if err := fd.Setter(newConf, userVal); err != nil {
			s.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("字段 %s 无效: %s", fd.Name, err.Error()))
			return
		}
	}

	confPath := filepath.Join(s.appRootPath, "conf.yaml")

	backupPath := confPath + ".backup." + strconv.FormatInt(time.Now().Unix(), 10)
	if data, err := os.ReadFile(confPath); err == nil {
		if writeErr := os.WriteFile(backupPath, data, 0600); writeErr != nil {
			log.Warnf("Failed to create config backup: %v", writeErr)
		}
	}

	if err := config.WriteConf(confPath, newConf); err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	log.Infoln("[WebUI] config saved via structured form")

	*s.config = *newConf

	yamlPreview, err := yaml.Marshal(newConf)
	if err != nil {
		log.Warnf("Failed to marshal yaml preview: %v", err)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message":      "Configuration saved successfully. Please restart TMD manually for changes to take effect.",
		"backup":       filepath.Base(backupPath),
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
