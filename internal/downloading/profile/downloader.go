package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/unkmonster/tmd/internal/database"
	"github.com/unkmonster/tmd/internal/downloader"
	"github.com/unkmonster/tmd/internal/twitter"
	"github.com/unkmonster/tmd/internal/utils"
)

const (
	profileDirName    = ".loongtweet"
	profileSubDirName = ".profile"
	versionsDirName   = ".versions"
)

var validImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

func imageExtFromURL(rawURL string) string {
	ext, err := utils.GetExtFromUrl(rawURL)
	if err != nil {
		ext = pathpkg.Ext(rawURL)
	}
	ext = strings.ToLower(ext)
	if validImageExts[ext] {
		return ext
	}
	return ".jpg"
}

func ensureProfileDirs(userDir string) (string, error) {
	profileDir := filepath.Join(userDir, profileDirName, profileSubDirName)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create profile dir: %w", err)
	}
	versionsDir := filepath.Join(profileDir, versionsDirName)
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create versions dir: %w", err)
	}
	return profileDir, nil
}

type ProfileDownloader struct {
	config     *Config
	storage    *FileStorageManager
	client     *resty.Client
	db         *sqlx.DB
	downloader downloader.Downloader
	fileWriter downloader.FileWriter
	progress   func(DownloadProgress)
}

func validateAndDefaultConfig(config *Config, storage *FileStorageManager, dwn downloader.Downloader, fw downloader.FileWriter) *Config {
	if config == nil {
		config = DefaultConfig()
	}
	if config.FileDownloadTimeout <= 0 {
		config.FileDownloadTimeout = DefaultConfig().FileDownloadTimeout
	}
	if config.MaxDownloadRoutine <= 0 {
		config.MaxDownloadRoutine = DefaultConfig().MaxDownloadRoutine
	}
	if storage == nil {
		panic("profile: storage cannot be nil")
	}
	if dwn == nil {
		panic("profile: downloader cannot be nil")
	}
	if fw == nil {
		panic("profile: fileWriter cannot be nil")
	}
	return config
}

// getClient 获取可用的HTTP客户端
func getClient(clients []*resty.Client) *resty.Client {
	// 优先选择没有错误的客户端
	for _, client := range clients {
		if client != nil && twitter.GetClientError(client) == nil {
			return client
		}
	}
	// 如果没有可用客户端，返回第一个（如果有）
	if len(clients) > 0 {
		return clients[0]
	}
	return nil
}

func NewProfileDownloaderWithDB(config *Config, storage *FileStorageManager, clients []*resty.Client, db *sqlx.DB, dwn downloader.Downloader, fw downloader.FileWriter) *ProfileDownloader {
	config = validateAndDefaultConfig(config, storage, dwn, fw)
	return &ProfileDownloader{
		config:     config,
		storage:    storage,
		client:     getClient(clients),
		db:         db,
		downloader: dwn,
		fileWriter: fw,
	}
}

func (pd *ProfileDownloader) SetProgressCallback(cb func(DownloadProgress)) {
	pd.progress = cb
}

type DownloadRequest struct {
	ScreenName  string // 用户屏幕名 (@username)
	UserTitle   string // 用于目录名，格式: Name(ScreenName)
	Name        string // 纯净的显示名称(仅Name，不含ScreenName)
	UserID      uint64 // 用户ID（必需）
	AvatarURL   string // 头像URL
	BannerURL   string // 横幅URL（可选）
	Description string // 用户简介
	Location    string // 位置
	URL         string // 个人链接
	Verified    bool   // 是否认证
	Protected   bool   // 是否受保护
	CreatedAt   string // 账号创建时间
}

type indexedRequest struct {
	index   int
	request DownloadRequest
}

func (pd *ProfileDownloader) Download(ctx context.Context, req DownloadRequest) (*DownloadResult, error) {
	startTime := time.Now()
	result := &DownloadResult{
		ScreenName: req.ScreenName,
		Files:      make([]FileResult, 0),
	}

	// 数据完整性检查
	if req.UserID == 0 {
		result.Error = fmt.Errorf("incomplete profile data: UserID is required")
		if pd.db != nil {
			database.MarkUserInaccessible(pd.db, 0, req.ScreenName)
		}
		return result, result.Error
	}
	if req.Name == "" {
		result.Error = fmt.Errorf("incomplete profile data: Name is required")
		if pd.db != nil {
			database.MarkUserInaccessible(pd.db, req.UserID, req.ScreenName)
		}
		return result, result.Error
	}

	// 直接使用传递的数据，不再调用API
	profile := &ProfileInfo{
		ID:          req.UserID,
		Name:        req.Name,
		ScreenName:  req.ScreenName,
		AvatarURL:   req.AvatarURL,
		BannerURL:   req.BannerURL,
		Description: req.Description,
		Location:    req.Location,
		URL:         req.URL,
		Verified:    req.Verified,
		Protected:   req.Protected,
		CreatedAt:   req.CreatedAt,
	}
	log.Debugln("using provided profile data for:", req.ScreenName)

	result.Profile = profile
	log.Debugln("profile fetched:", profile.Name, "(id:", profile.ID, ")")

	userTitle := req.UserTitle
	if userTitle == "" {
		userTitle = fmt.Sprintf("%s(%s)", profile.Name, req.ScreenName)
	}
	userTitle = utils.WinFileNameWithMaxLen(userTitle, pd.config.MaxFileNameLen)

	var userDir string
	var err error

	if pd.db != nil && profile.ID != 0 {
		userDir, err = pd.syncUserDirectory(profile, userTitle, req.ScreenName)
		if err != nil {
			result.Error = fmt.Errorf("failed to sync directory: %w", err)
			return result, result.Error
		}
	} else {
		userDir, err = pd.storage.EnsureDirectory(userTitle)
		if err != nil {
			result.Error = fmt.Errorf("failed to create directory: %w", err)
			return result, result.Error
		}
	}

	log.Debugln("directory ready:", userDir)

	fetchedAt := time.Now()

	if profile.AvatarURL != "" {
		avatarResult := pd.downloadAvatar(ctx, userTitle, req.ScreenName, profile.AvatarURL, fetchedAt)
		result.Files = append(result.Files, avatarResult)
	}

	if profile.BannerURL != "" {
		bannerResult := pd.downloadFile(ctx, userTitle, req.ScreenName, FileTypeBanner, profile.BannerURL, ".jpg", fetchedAt, "banner")
		result.Files = append(result.Files, bannerResult)
	}

	descResult := pd.saveContent(userTitle, FileTypeDescription, []byte(profile.Description), fetchedAt)
	result.Files = append(result.Files, descResult)

	profileResult := pd.saveProfileJSON(userTitle, req.ScreenName, profile, fetchedAt)
	result.Files = append(result.Files, profileResult)

	result.Success = true
	for _, file := range result.Files {
		if file.Status == StatusFailed {
			result.Success = false
			result.Error = fmt.Errorf("some files failed to download")
			break
		}
	}

	result.DownloadTime = time.Since(startTime)

	return result, nil
}

func (pd *ProfileDownloader) syncUserDirectory(profile *ProfileInfo, userTitle, screenName string) (string, error) {
	if err := database.SyncUser(pd.db, profile.ID, profile.Name, screenName, profile.Protected, 0, true); err != nil {
		return "", err
	}

	entity, err := database.LocateUserEntity(pd.db, profile.ID, pd.storage.usersBasePath)
	if err != nil {
		return "", err
	}

	expectedTitle := userTitle

	if entity == nil {
		entity = &database.UserEntity{
			UserId:    profile.ID,
			ParentDir: pd.storage.usersBasePath,
			Name:      expectedTitle,
		}
		userDir := filepath.Join(pd.storage.usersBasePath, expectedTitle)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return "", err
		}
		if err := database.CreateUserEntity(pd.db, entity); err != nil {
			return "", err
		}
		log.Infoln("new user directory created:", userDir)
		return ensureProfileDirs(userDir)
	}

	oldUserDir, err := entity.Path()
	if err != nil {
		return "", err
	}
	if entity.Name == expectedTitle {
		if err := os.MkdirAll(oldUserDir, 0755); err != nil && !os.IsExist(err) {
			return "", err
		}
		return ensureProfileDirs(oldUserDir)
	}

	newUserDir := filepath.Join(pd.storage.usersBasePath, expectedTitle)
	if err := os.Rename(oldUserDir, newUserDir); err != nil {
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(newUserDir, 0755); mkdirErr != nil {
				return "", mkdirErr
			}
		} else {
			return "", err
		}
	}

	entity.Name = expectedTitle
	if err := database.UpdateUserEntity(pd.db, entity); err != nil {
		return "", err
	}

	log.Infoln("user directory renamed:", oldUserDir, "->", newUserDir)
	return ensureProfileDirs(newUserDir)
}

func (pd *ProfileDownloader) DownloadMultiple(ctx context.Context, requests []DownloadRequest) []*DownloadResult {
	if len(requests) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil) // 确保 cancel 在所有情况下都被调用

	results := make([]*DownloadResult, len(requests))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var progressMu sync.Mutex
	completedCount := 0
	failedCount := 0

	numRoutine := min(len(requests), pd.config.MaxDownloadRoutine)

	reqChan := make(chan indexedRequest, len(requests))
	for i, req := range requests {
		reqChan <- indexedRequest{index: i, request: req}
	}
	close(reqChan)

	for i := 0; i < numRoutine; i++ {
		wg.Add(1)
		go pd.profileDownloader(ctx, cancel, &wg, &mu, results, reqChan, len(requests), &progressMu, &completedCount, &failedCount)
	}

	wg.Wait()
	return results
}

func (pd *ProfileDownloader) profileDownloader(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	results []*DownloadResult,
	reqChan <-chan indexedRequest,
	total int,
	progressMu *sync.Mutex,
	completedCount *int,
	failedCount *int,
) {
	defer wg.Done()
	defer func() {
		if p := recover(); p != nil {
			log.Errorf("[profileDownloader] panic recovered: %v", p)
			cancel(fmt.Errorf("panic: %v", p))

			// 把 channel 中剩余的任务标记为失败
			for ir := range reqChan {
				mu.Lock()
				results[ir.index] = &DownloadResult{
					ScreenName: ir.request.ScreenName,
					Success:    false,
					Error:      fmt.Errorf("download cancelled due to panic"),
				}
				mu.Unlock()
			}
		}
	}()

	for {
		select {
		case ir, ok := <-reqChan:
			if !ok {
				return
			}
			result, err := pd.Download(ctx, ir.request)
			if err != nil {
				log.Errorln("profile download failed:", ir.request.ScreenName, "-", err)

				if errors.Is(err, syscall.ENOSPC) {
					cancel(err)
					// 把 channel 中剩余的任务标记为失败
					for remainingIr := range reqChan {
						mu.Lock()
						results[remainingIr.index] = &DownloadResult{
							ScreenName: remainingIr.request.ScreenName,
							Success:    false,
							Error:      fmt.Errorf("download cancelled: disk full"),
						}
						mu.Unlock()
					}
					return
				}
			}

			mu.Lock()
			results[ir.index] = result
			mu.Unlock()
			pd.reportProgress(total, ir.request.ScreenName, result, progressMu, completedCount, failedCount)

		case <-ctx.Done():
			// 把 channel 中剩余的任务标记为失败
			for ir := range reqChan {
				mu.Lock()
				results[ir.index] = &DownloadResult{
					ScreenName: ir.request.ScreenName,
					Success:    false,
					Error:      ctx.Err(),
				}
				mu.Unlock()
			}
			return
		}
	}
}

func (pd *ProfileDownloader) reportProgress(total int, current string, result *DownloadResult, progressMu *sync.Mutex, completedCount *int, failedCount *int) {
	if pd.progress == nil {
		return
	}

	progressMu.Lock()
	*completedCount = *completedCount + 1
	if result == nil || result.Error != nil || !result.Success {
		*failedCount = *failedCount + 1
	}
	progress := DownloadProgress{
		Total:     total,
		Completed: *completedCount,
		Failed:    *failedCount,
		Current:   current,
	}
	progressMu.Unlock()

	pd.progress(progress)
}

func (pd *ProfileDownloader) downloadAvatar(ctx context.Context, userTitle, screenName, url string, fetchedAt time.Time) FileResult {
	ext := imageExtFromURL(url)
	return pd.downloadFile(ctx, userTitle, screenName, FileTypeAvatar,
		GetHighResAvatarURL(url, pd.config.AvatarQuality), ext, fetchedAt, "avatar")
}

func (pd *ProfileDownloader) downloadFile(ctx context.Context, userTitle, screenName string, fileType FileType, url, defaultExt string, fetchedAt time.Time, label string) FileResult {
	filePath := pd.storage.GetFilePathWithExt(userTitle, fileType, defaultExt)
	downloadCtx := ctx
	if downloadCtx == nil {
		downloadCtx = context.Background()
	}
	if pd.config.FileDownloadTimeout > 0 {
		var cancel context.CancelFunc
		downloadCtx, cancel = context.WithTimeout(downloadCtx, pd.config.FileDownloadTimeout)
		defer cancel()
	}

	downloadReq := downloader.DownloadRequest{
		Context:     downloadCtx,
		Client:      pd.client,
		URL:         url,
		Destination: filePath,
		Options: downloader.DownloadOptions{
			SkipUnchanged: pd.config.SkipUnchanged,
			CreateVersion: pd.config.EnableVersioning,
			SetModTime:    &fetchedAt,
		},
	}

	result, err := pd.downloader.Download(downloadReq)
	if err != nil {
		log.Debugln(label+" download failed:", screenName, "-", err)
		return FileResult{FileType: fileType, FilePath: filePath, Status: StatusFailed, Error: err}
	}

	return FileResult{
		FileType:  fileType,
		Status:    StatusDownloaded,
		FilePath:  result.FilePath,
		OldSize:   result.OldSize,
		NewSize:   result.FileSize,
		Versioned: result.Versioned,
	}
}

func (pd *ProfileDownloader) saveProfileJSON(userTitle, screenName string, profile *ProfileInfo, fetchedAt time.Time) FileResult {
	data, err := ProfileToJSON(profile)
	if err != nil {
		log.Errorln("profile JSON serialize failed:", screenName, "-", err)
		filePath := pd.storage.GetFilePath(userTitle, FileTypeProfile)
		return FileResult{FileType: FileTypeProfile, FilePath: filePath, Status: StatusFailed, Error: err}
	}
	return pd.saveContent(userTitle, FileTypeProfile, data, fetchedAt)
}

func (pd *ProfileDownloader) saveContent(userTitle string, fileType FileType, data []byte, fetchedAt time.Time) FileResult {
	filePath := pd.storage.GetFilePath(userTitle, fileType)

	writeReq := downloader.WriteRequest{
		Path: filePath,
		Data: data,
		Options: downloader.WriteOptions{
			CreateVersion: pd.config.EnableVersioning,
			SkipUnchanged: pd.config.SkipUnchanged,
			ModTime:       &fetchedAt,
		},
	}

	result, err := pd.fileWriter.Write(writeReq)
	if err != nil {
		return FileResult{FileType: fileType, FilePath: filePath, Status: StatusFailed, Error: err}
	}

	return FileResult{
		FileType:  fileType,
		Status:    StatusDownloaded,
		FilePath:  filePath,
		OldSize:   result.OldSize,
		NewSize:   result.NewSize,
		Versioned: result.Versioned,
	}
}

var reNormalAvatarURL = regexp.MustCompile(`_normal(\.[a-zA-Z]+)$`)

// GetHighResAvatarURL 获取高分辨率头像URL
func GetHighResAvatarURL(url string, quality string) string {
	if url == "" {
		return ""
	}
	return reNormalAvatarURL.ReplaceAllString(url, "_"+quality+"$1")
}

// ProfileToJSON 将ProfileInfo转换为JSON
func ProfileToJSON(profile *ProfileInfo) ([]byte, error) {
	return json.MarshalIndent(profile, "", "  ")
}
