package admin

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"gorm.io/gorm"

	"lan-im-go/files/storage"
	"lan-im-go/models"
)

// FileService 负责超级管理员后台的文件管理、异常检测和安全清理。
type FileService struct {
	db      *gorm.DB
	storage storage.Provider
	audit   *AuditService
}

func NewFileService(db *gorm.DB, provider storage.Provider, audit *AuditService) *FileService {
	return &FileService{db: db, storage: provider, audit: audit}
}

// FileListQuery 文件列表查询条件。
type FileListQuery struct {
	Page       int
	PageSize   int
	Keyword    string
	UploaderID int64
	RoomID     int64
	FileType   string
	Status     string
	Start      time.Time
	End        time.Time
}

// FileListItem 管理后台列表展示的文件项。
type FileListItem struct {
	models.FileRecord
	Username   string `json:"username"`
	RoomName   string `json:"room_name"`
	Exists     bool   `json:"exists"`
	HasMessage bool   `json:"has_message"`
}

// RecordUpload 保存一条已完成上传的文件记录。
type AuditAction struct {
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}

func (s *FileService) RecordUpload(ctx context.Context, userID int64, req CompleteUploadRequest) (*models.FileRecord, error) {
	var existing models.FileRecord
	err := s.db.WithContext(ctx).Where("object_key = ?", req.ObjectKey).First(&existing).Error
	if err == nil {
		existing.OriginalName = req.OriginalName
		existing.SHA256 = req.SHA256
		existing.Size = req.FileSize
		existing.UploaderID = userID
		existing.RoomID = req.RoomID
		existing.Status = "uploaded"
		existing.UpdatedAt = time.Now()
		if s.storage != nil {
			existing.Backend = string(s.storage.BackendType())
		}
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	backend := "minio"
	if s.storage != nil {
		backend = string(s.storage.BackendType())
	}
	record := &models.FileRecord{
		ObjectKey:    req.ObjectKey,
		OriginalName: req.OriginalName,
		SHA256:       req.SHA256,
		Size:         req.FileSize,
		UploaderID:   userID,
		RoomID:       req.RoomID,
		Backend:      backend,
		Status:       "uploaded",
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// CompleteUploadRequest 与前端上传完成后回调的请求保持一致。
type CompleteUploadRequest struct {
	ObjectKey    string
	OriginalName string
	SHA256       string
	FileSize     int64
	RoomID       int64
}

// ListFiles 分页查询文件记录，并补充存储实际状态。
func (s *FileService) ListFiles(ctx context.Context, q FileListQuery) ([]FileListItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.FileRecord{})
	if q.Keyword != "" {
		like := q.Keyword + "%"
		query = query.Where("original_name LIKE ? OR object_key LIKE ?", like, like)
	}
	if q.UploaderID > 0 {
		query = query.Where("uploader_id = ?", q.UploaderID)
	}
	if q.RoomID > 0 {
		query = query.Where("room_id = ?", q.RoomID)
	}
	if q.FileType != "" {
		query = query.Where("LOWER(original_name) LIKE ?", "%."+strings.ToLower(q.FileType))
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if !q.Start.IsZero() {
		query = query.Where("created_at >= ?", q.Start)
	}
	if !q.End.IsZero() {
		query = query.Where("created_at < ?", q.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []models.FileRecord
	if err := query.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	items := make([]FileListItem, 0, len(records))
	uploaderIDs := make([]int64, 0, len(records))
	roomIDs := make([]int64, 0, len(records))
	for _, record := range records {
		uploaderIDs = append(uploaderIDs, record.UploaderID)
		if record.RoomID > 0 {
			roomIDs = append(roomIDs, record.RoomID)
		}
	}
	usernameMap := s.usernames(ctx, uploaderIDs)
	roomNameMap := s.roomNames(ctx, roomIDs)
	for _, record := range records {
		item := FileListItem{FileRecord: record, HasMessage: record.MessageID > 0}
		item.Username = usernameMap[record.UploaderID]
		item.RoomName = roomNameMap[record.RoomID]
		item.Exists = s.objectExists(ctx, record.ObjectKey)
		items = append(items, item)
	}
	return items, total, nil
}

// GetFile 获取单个文件记录。
func (s *FileService) GetFile(ctx context.Context, id int64) (*FileListItem, error) {
	var record models.FileRecord
	if err := s.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, err
	}
	item := &FileListItem{FileRecord: record, HasMessage: record.MessageID > 0}
	item.Username = s.username(ctx, record.UploaderID)
	item.RoomName = s.roomName(ctx, record.RoomID)
	item.Exists = s.objectExists(ctx, record.ObjectKey)
	return item, nil
}

// DeleteFile 同时删除对象存储中的实际文件和 MySQL 元数据。
func (s *FileService) DeleteFile(ctx context.Context, id int64, action AuditAction) error {
	record, err := s.GetFile(ctx, id)
	if err != nil {
		return err
	}
	// 先删除数据库记录，再删除对象存储文件；若对象删除失败则回补数据库记录，避免产生孤儿对象或悬空记录。
	if err := s.db.WithContext(ctx).Delete(&models.FileRecord{}, id).Error; err != nil {
		return err
	}
	if s.storage != nil {
		delCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := s.storage.Delete(delCtx, record.ObjectKey); err != nil {
			_ = s.db.WithContext(ctx).Create(&record)
			return fmt.Errorf("对象存储文件删除失败: %w", err)
		}
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "file.delete",
			TargetType:    "file",
			TargetID:      fmt.Sprintf("%d", id),
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return nil
}

// FileScanResult 文件异常检测结果。
type FileScanResult struct {
	TotalRecords   int64                `json:"total_records"`
	TotalObjects   int                  `json:"total_objects"`
	MissingRecords []FileListItem       `json:"missing_records"`
	OrphanObjects  []storage.ObjectStat `json:"orphan_objects"`
	StaleRecords   []FileListItem       `json:"stale_records"`
}

// ScanAnomalies 检查数据库记录与对象存储是否一致。
func (s *FileService) ScanAnomalies(ctx context.Context) (*FileScanResult, error) {
	var records []models.FileRecord
	if err := s.db.WithContext(ctx).Order("id DESC").Limit(5000).Find(&records).Error; err != nil {
		return nil, err
	}
	result := &FileScanResult{TotalRecords: int64(len(records)), MissingRecords: []FileListItem{}, OrphanObjects: []storage.ObjectStat{}, StaleRecords: []FileListItem{}}

	knownKeys := make(map[string]struct{}, len(records))
	staleBefore := time.Now().Add(-24 * time.Hour)
	for _, record := range records {
		knownKeys[record.ObjectKey] = struct{}{}
		item := FileListItem{FileRecord: record, HasMessage: record.MessageID > 0}
		item.Username = s.username(ctx, record.UploaderID)
		item.RoomName = s.roomName(ctx, record.RoomID)
		item.Exists = s.objectExists(ctx, record.ObjectKey)
		if !item.Exists {
			result.MissingRecords = append(result.MissingRecords, item)
		}
		if record.MessageID == 0 && record.CreatedAt.Before(staleBefore) {
			result.StaleRecords = append(result.StaleRecords, item)
		}
	}

	if s.storage == nil {
		return result, nil
	}
	objects, err := s.storage.ListObjects(ctx, "", 1000)
	if err != nil {
		return nil, err
	}
	result.TotalObjects = len(objects)
	for _, obj := range objects {
		if _, ok := knownKeys[obj.Key]; !ok {
			result.OrphanObjects = append(result.OrphanObjects, obj)
		}
	}
	return result, nil
}

// CleanupOrphans 清理无消息引用且超过 24 小时的孤立对象，避免误删刚上传或已被引用的文件。
func (s *FileService) CleanupOrphans(ctx context.Context, action AuditAction) (int, error) {
	scan, err := s.ScanAnomalies(ctx)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, obj := range scan.OrphanObjects {
		if s.storage != nil {
			if err := s.storage.Delete(ctx, obj.Key); err != nil {
				continue
			}
		}
		cleaned++
	}
	if cleaned > 0 && s.audit != nil {
		_ = s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "file.cleanup_orphans",
			TargetType:    "file",
			TargetID:      fmt.Sprintf("%d", cleaned),
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return cleaned, nil
}

func (s *FileService) objectExists(ctx context.Context, key string) bool {
	if s.storage == nil {
		return false
	}
	statCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	stat, err := s.storage.Stat(statCtx, key)
	return err == nil && stat.Exists
}

func (s *FileService) usernames(ctx context.Context, userIDs []int64) map[int64]string {
	out := make(map[int64]string, len(userIDs))
	if len(userIDs) == 0 {
		return out
	}
	var users []models.User
	if err := s.db.WithContext(ctx).Select("id, username").Where("id IN ?", userIDs).Find(&users).Error; err == nil {
		for _, user := range users {
			out[user.ID] = user.Username
		}
	}
	return out
}

func (s *FileService) roomNames(ctx context.Context, roomIDs []int64) map[int64]string {
	out := make(map[int64]string, len(roomIDs))
	if len(roomIDs) == 0 {
		return out
	}
	var rooms []models.Room
	if err := s.db.WithContext(ctx).Select("id, name").Where("id IN ?", roomIDs).Find(&rooms).Error; err == nil {
		for _, room := range rooms {
			out[room.ID] = room.Name
		}
	}
	return out
}

func (s *FileService) username(ctx context.Context, userID int64) string {
	var user models.User
	if err := s.db.WithContext(ctx).Select("username").First(&user, userID).Error; err == nil {
		return user.Username
	}
	return ""
}

func (s *FileService) roomName(ctx context.Context, roomID int64) string {
	var room models.Room
	if err := s.db.WithContext(ctx).Select("name").First(&room, roomID).Error; err == nil {
		return room.Name
	}
	return ""
}

// SafeObjectKey 从用户传入的对象键中提取安全的原始文件名。
func SafeObjectKey(key string) string {
	key = strings.TrimPrefix(path.Clean(strings.ReplaceAll(key, "\\\\", "/")), "/")
	if key == "." || key == "" {
		return "file"
	}
	return key
}
