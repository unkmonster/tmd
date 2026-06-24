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
	"github.com/unkmonster/tmd/internal/scheduler"
)

func (s *Server) handleGetSchedules(w http.ResponseWriter, _ *http.Request) {
	sched := s.getScheduler()
	if sched == nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"scheduler_running": false,
			"entries":           []scheduler.ScheduleStatus{},
			"active":            0,
			"total":             0,
		}))
		return
	}

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
	exists := true
	if _, err := os.Stat(schedulesPath); os.IsNotExist(err) {
		exists = false
	}

	statuses := sched.GetStatuses()
	active := 0
	for _, st := range statuses {
		if st.Entry.Enabled {
			active++
		}
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"scheduler_running": sched.IsRunning(),
		"entries":           statuses,
		"active":            active,
		"total":             len(statuses),
		"exists":            exists,
	}))
}

func (s *Server) handleGetSchedulesRaw(w http.ResponseWriter, _ *http.Request) {
	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")

	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	data, err := os.ReadFile(schedulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(SchedulesRawResponse{
				Content: "",
				Path:    schedulesPath,
				Exists:  false,
			}))
			return
		}
		log.Errorf("[schedules] Failed to read schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read schedules", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(SchedulesRawResponse{
		Content: string(data),
		Path:    schedulesPath,
		Exists:  true,
	}))
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var entry scheduler.ScheduleEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	cfg, err := s.readScheduleConfigLocked(schedulesPath)
	if err != nil {
		log.Errorf("[schedules] Failed to read schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read schedules", err.Error())
		return
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = scheduler.NewEntryID(entry, usedScheduleIDs(cfg.Schedules))
	}
	cfg.Schedules = append(cfg.Schedules, entry)
	cfg, err = normalizeAndValidateScheduleConfig(cfg)
	if err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, cfg)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}
	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedule saved, but reload failed: " + err.Error(),
			"backup":  backupName,
			"entry":   cfg.Schedules[len(cfg.Schedules)-1],
		}))
		return
	}

	s.writeJSON(w, http.StatusCreated, NewSuccessResponse(map[string]interface{}{
		"message": "Schedule created successfully.",
		"backup":  backupName,
		"entry":   cfg.Schedules[len(cfg.Schedules)-1],
	}))
}

func (s *Server) handleReplaceSchedules(w http.ResponseWriter, r *http.Request) {
	var req SchedulesReplaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	cfg, err := normalizeAndValidateScheduleConfig(scheduler.ScheduleConfig{Schedules: req.Entries})
	if err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, cfg)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}
	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedules saved, but reload failed: " + err.Error(),
			"backup":  backupName,
			"entries": cfg.Schedules,
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedules saved and reloaded successfully.",
		"backup":  backupName,
		"entries": cfg.Schedules,
	}))
}

func (s *Server) handleUpdateSchedulesRaw(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		s.writeError(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	var testConf scheduler.ScheduleConfig
	if err := yaml.Unmarshal([]byte(req.Content), &testConf); err != nil {
		log.Errorf("[schedules] Invalid YAML format: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid YAML format", err.Error())
		return
	}
	testConf, err := normalizeAndValidateScheduleConfig(testConf)
	if err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")

	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, testConf)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}

	log.Infoln("[WebUI] schedules saved via raw editor")

	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedules saved, but reload failed: " + err.Error(),
			"backup":  backupName,
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedules saved and reloaded successfully.",
		"backup":  backupName,
	}))
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var entry scheduler.ScheduleEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if entry.ID != "" && entry.ID != id {
		s.writeError(w, http.StatusBadRequest, "Schedule id in body must match path id")
		return
	}
	entry.ID = id

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	if _, err := os.Stat(schedulesPath); os.IsNotExist(err) {
		s.writeError(w, http.StatusNotFound, "Schedules config file not found")
		return
	}
	cfg, err := s.readScheduleConfigLocked(schedulesPath)
	if err != nil {
		log.Errorf("[schedules] Failed to read schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read schedules", err.Error())
		return
	}
	idx := findScheduleIndex(cfg.Schedules, id)
	if idx < 0 {
		s.writeError(w, http.StatusNotFound, "Schedule not found")
		return
	}
	cfg.Schedules[idx] = entry
	cfg, err = normalizeAndValidateScheduleConfig(cfg)
	if err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, cfg)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}
	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedule saved, but reload failed: " + err.Error(),
			"backup":  backupName,
			"entry":   cfg.Schedules[idx],
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedule updated successfully.",
		"backup":  backupName,
		"entry":   cfg.Schedules[idx],
	}))
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")

	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	if _, err := os.Stat(schedulesPath); os.IsNotExist(err) {
		s.writeError(w, http.StatusNotFound, "Schedules config file not found")
		return
	}
	cfg, err := s.readScheduleConfigLocked(schedulesPath)
	if err != nil {
		log.Errorf("[schedules] Failed to read schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read schedules", err.Error())
		return
	}
	idx := findScheduleIndex(cfg.Schedules, id)
	if idx < 0 {
		s.writeError(w, http.StatusNotFound, "Schedule not found")
		return
	}
	cfg.Schedules = append(cfg.Schedules[:idx], cfg.Schedules[idx+1:]...)
	if err := validateScheduleConfig(cfg); err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, cfg)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}
	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedule deleted, but reload failed: " + err.Error(),
			"backup":  backupName,
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedule deleted successfully.",
		"backup":  backupName,
	}))
}

func (s *Server) handleSetScheduleEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ScheduleEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	if _, err := os.Stat(schedulesPath); os.IsNotExist(err) {
		s.writeError(w, http.StatusNotFound, "Schedules config file not found")
		return
	}
	cfg, err := s.readScheduleConfigLocked(schedulesPath)
	if err != nil {
		log.Errorf("[schedules] Failed to read schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to read schedules", err.Error())
		return
	}
	idx := findScheduleIndex(cfg.Schedules, id)
	if idx < 0 {
		s.writeError(w, http.StatusNotFound, "Schedule not found")
		return
	}
	cfg.Schedules[idx].Enabled = req.Enabled
	if err := validateScheduleConfig(cfg); err != nil {
		log.Errorf("[schedules] Invalid schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid schedule configuration", err.Error())
		return
	}

	backupName, err := s.writeScheduleConfigLocked(schedulesPath, cfg)
	if err != nil {
		log.Errorf("[schedules] Failed to save schedule configuration: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to save schedule configuration", err.Error())
		return
	}
	if err := s.reloadSchedulesLocked(schedulesPath); err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
			"message": "Schedule updated, but reload failed: " + err.Error(),
			"backup":  backupName,
			"entry":   cfg.Schedules[idx],
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedule updated successfully.",
		"backup":  backupName,
		"entry":   cfg.Schedules[idx],
	}))
}

func (s *Server) handleReloadSchedules(w http.ResponseWriter, _ *http.Request) {
	s.schedulesMu.Lock()
	defer s.schedulesMu.Unlock()

	if err := s.reloadSchedulesLocked(filepath.Join(s.appRootPath, "schedules.yaml")); err != nil {
		log.Errorf("[schedules] Failed to reload schedules: %v", err)
		s.writeErrorDetail(w, http.StatusInternalServerError, "Failed to reload schedules", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedules reloaded successfully.",
	}))
}

func (s *Server) handleTriggerSchedule(w http.ResponseWriter, r *http.Request) {
	sched := s.getScheduler()
	if sched == nil {
		s.writeError(w, http.StatusBadRequest, "Scheduler not initialized")
		return
	}

	id := r.PathValue("id")

	taskID, err := sched.TriggerByID(id)
	if err != nil {
		log.Errorf("[schedules] Failed to trigger schedule: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Failed to trigger schedule", err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"message": "Schedule triggered successfully.",
		"task_id": taskID,
	}))
}

func (s *Server) readScheduleConfigLocked(schedulesPath string) (scheduler.ScheduleConfig, error) {
	data, err := os.ReadFile(schedulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return scheduler.ScheduleConfig{}, nil
		}
		return scheduler.ScheduleConfig{}, err
	}

	var cfg scheduler.ScheduleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return scheduler.ScheduleConfig{}, fmt.Errorf("invalid YAML format: %w", err)
	}
	return normalizeAndValidateScheduleConfig(cfg)
}

func (s *Server) writeScheduleConfigLocked(schedulesPath string, cfg scheduler.ScheduleConfig) (string, error) {
	backupName, err := config.CreateBackup(schedulesPath)
	if err != nil {
		log.Warnf("Failed to create schedules backup: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return backupName, fmt.Errorf("failed to marshal schedules: %w", err)
	}
	if err := os.WriteFile(schedulesPath, data, 0600); err != nil {
		return backupName, fmt.Errorf("failed to write schedules: %w", err)
	}
	return backupName, nil
}

func (s *Server) reloadSchedulesLocked(schedulesPath string) error {
	if sched := s.getScheduler(); sched != nil {
		if err := sched.Reload(); err != nil {
			return err
		}
		if !sched.IsRunning() && hasEnabledScheduleStatus(sched.GetStatuses()) {
			sched.Start()
			s.handleScheduleStatusChange(sched.GetStatuses())
		}
		return nil
	}

	newSched, err := scheduler.New(schedulesPath, s.scheduledDownload)
	if err != nil {
		return fmt.Errorf("scheduler initialization failed: %w", err)
	}
	newSched.OnStatusChange = s.handleScheduleStatusChange
	newSched.Start()

	s.schedulerMu.Lock()
	if s.scheduler == nil {
		s.scheduler = newSched
		s.schedulerMu.Unlock()
		return nil
	}
	existingSched := s.scheduler
	s.schedulerMu.Unlock()
	existingSched.OnStatusChange = s.handleScheduleStatusChange
	if err := existingSched.Reload(); err != nil {
		return err
	}
	if !existingSched.IsRunning() && hasEnabledScheduleStatus(existingSched.GetStatuses()) {
		existingSched.Start()
		s.handleScheduleStatusChange(existingSched.GetStatuses())
	}
	return nil
}

func hasEnabledScheduleStatus(statuses []scheduler.ScheduleStatus) bool {
	for _, status := range statuses {
		if status.Entry.Enabled {
			return true
		}
	}
	return false
}

func normalizeAndValidateScheduleConfig(cfg scheduler.ScheduleConfig) (scheduler.ScheduleConfig, error) {
	entries, err := scheduler.NormalizeEntries(cfg.Schedules)
	if err != nil {
		return scheduler.ScheduleConfig{}, err
	}
	for i, entry := range entries {
		if err := scheduler.ValidateEntry(entry); err != nil {
			return scheduler.ScheduleConfig{}, fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
		if _, err := scheduler.ParseSchedule(entry.Schedule); err != nil {
			return scheduler.ScheduleConfig{}, fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
	}
	return scheduler.ScheduleConfig{Schedules: entries}, nil
}

// validateScheduleConfig 只做校验不做 ID 自动分配，用于 delete/enable 等局部操作。
func validateScheduleConfig(cfg scheduler.ScheduleConfig) error {
	used := make(map[string]struct{}, len(cfg.Schedules))
	for i, entry := range cfg.Schedules {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return fmt.Errorf("schedule #%d (%s): missing id", i+1, entry.Name)
		}
		if !scheduler.ScheduleIDPattern.MatchString(id) {
			return fmt.Errorf("schedule #%d (%s): invalid id %q (use letters, numbers, '_' or '-')", i+1, entry.Name, id)
		}
		if _, exists := used[id]; exists {
			return fmt.Errorf("schedule #%d (%s): duplicate id %q", i+1, entry.Name, id)
		}
		used[id] = struct{}{}

		if err := scheduler.ValidateEntry(entry); err != nil {
			return fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
		if _, err := scheduler.ParseSchedule(entry.Schedule); err != nil {
			return fmt.Errorf("schedule #%d (%s): %w", i+1, entry.Name, err)
		}
	}
	return nil
}

func findScheduleIndex(entries []scheduler.ScheduleEntry, id string) int {
	for i, entry := range entries {
		if entry.ID == id {
			return i
		}
	}
	return -1
}

func usedScheduleIDs(entries []scheduler.ScheduleEntry) map[string]struct{} {
	used := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.ID != "" {
			used[entry.ID] = struct{}{}
		}
	}
	return used
}

func (s *Server) handleValidateSchedule(w http.ResponseWriter, r *http.Request) {
	var req ScheduleValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("[schedules] Invalid request body: %v", err)
		s.writeErrorDetail(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	var entries []scheduler.ScheduleEntry

	if req.Raw != "" {
		var cfg scheduler.ScheduleConfig
		if err := yaml.Unmarshal([]byte(req.Raw), &cfg); err != nil {
			s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleValidateResponse{
				Valid:  false,
				Errors: []string{"Invalid YAML format: " + err.Error()},
			}))
			return
		}
		entries = cfg.Schedules
	} else if req.Entry != nil {
		entries = []scheduler.ScheduleEntry{*req.Entry}
	} else if len(req.Entries) > 0 {
		entries = req.Entries
	} else {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleValidateResponse{
			Valid: true,
		}))
		return
	}

	_, err := normalizeAndValidateScheduleConfig(scheduler.ScheduleConfig{Schedules: entries})
	if err != nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleValidateResponse{
			Valid:  false,
			Errors: []string{err.Error()},
		}))
		return
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleValidateResponse{
		Valid: true,
	}))
}

func (s *Server) handleTriggerAllSchedules(w http.ResponseWriter, _ *http.Request) {
	sched := s.getScheduler()
	if sched == nil {
		s.writeError(w, http.StatusBadRequest, "Scheduler not initialized")
		return
	}

	statuses := sched.GetStatuses()
	var enabled []scheduler.ScheduleStatus
	for _, st := range statuses {
		if st.Entry.Enabled {
			enabled = append(enabled, st)
		}
	}
	if len(enabled) == 0 {
		s.writeError(w, http.StatusBadRequest, "No enabled schedules to trigger")
		return
	}

	results := make([]ScheduleTriggerAllResult, 0, len(enabled))
	succeeded := 0
	failed := 0

	for _, st := range enabled {
		taskID, err := sched.TriggerByID(st.Entry.ID)
		result := ScheduleTriggerAllResult{EntryID: st.Entry.ID}
		if err != nil {
			result.Error = err.Error()
			failed++
		} else {
			result.TaskID = taskID
			succeeded++
		}
		results = append(results, result)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleTriggerAllResponse{
		Total:     len(enabled),
		Succeeded: succeeded,
		Failed:    failed,
		Results:   results,
	}))
}

func (s *Server) handleScheduleStats(w http.ResponseWriter, _ *http.Request) {
	sched := s.getScheduler()
	if sched == nil {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(ScheduleStatsResponse{}))
		return
	}

	statuses := sched.GetStatuses()
	resp := ScheduleStatsResponse{Total: len(statuses)}
	for _, st := range statuses {
		if st.Entry.Enabled {
			resp.Enabled++
		} else {
			resp.Disabled++
		}
		if st.ConsecutiveFailures > 0 {
			resp.Failures++
		}
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(resp))
}
