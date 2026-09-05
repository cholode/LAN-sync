package application

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
	"lan-im-go/repository"
	"lan-im-go/services/messages/storage"
)

var (
	ErrFileNotFound   = errors.New("文件不存在")
	ErrFileForbidden  = errors.New("无权访问该文件")
	ErrInvalidFileKey = errors.New("文件标识非法")
)

type CompleteUploadRequest struct {
	ObjectKey    string
	OriginalName string
	SHA256       string
	FileSize     int64
	RoomID       int64
}

// FileService 负责聊天消息链路中的上传登记和下载授权。
// 管理端只消费这里生成的文件记录，用于审核与删除。
type FileService struct {
	db         *gorm.DB
	membership repository.RoomMemberRepository
	storage    storage.Provider
}

func NewFileService(db *gorm.DB, membership repository.RoomMemberRepository, provider storage.Provider) *FileService {
	return &FileService{db: db, membership: membership, storage: provider}
}

func (s *FileService) CompleteUpload(ctx context.Context, userID int64, req CompleteUploadRequest) (*models.FileRecord, error) {
	objectKey, err := normalizeObjectKey(req.ObjectKey)
	if err != nil || !objectKeyBelongsToUser(objectKey, userID) {
		return nil, ErrInvalidFileKey
	}
	if req.RoomID > 0 {
		ok, memberErr := s.membership.CheckIsMember(req.RoomID, userID)
		if memberErr != nil {
			return nil, fmt.Errorf("校验群成员身份: %w", memberErr)
		}
		if !ok {
			return nil, ErrFileForbidden
		}
	}

	var existing models.FileRecord
	err = s.db.WithContext(ctx).Where("object_key = ?", objectKey).First(&existing).Error
	if err == nil {
		if existing.UploaderID != userID {
			return nil, ErrFileForbidden
		}
		existing.OriginalName = path.Base(strings.TrimSpace(req.OriginalName))
		existing.SHA256 = strings.TrimSpace(req.SHA256)
		existing.Size = req.FileSize
		existing.RoomID = req.RoomID
		existing.Status = "uploaded"
		existing.UpdatedAt = time.Now().UTC()
		existing.Backend = string(s.storage.BackendType())
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	record := &models.FileRecord{
		ObjectKey:    objectKey,
		OriginalName: path.Base(strings.TrimSpace(req.OriginalName)),
		SHA256:       strings.TrimSpace(req.SHA256),
		Size:         req.FileSize,
		UploaderID:   userID,
		RoomID:       req.RoomID,
		Backend:      string(s.storage.BackendType()),
		Status:       "uploaded",
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (s *FileService) DownloadURLByID(ctx context.Context, userID, fileID int64) (string, error) {
	var record models.FileRecord
	if err := s.db.WithContext(ctx).First(&record, fileID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", err
	}
	return s.authorizedDownloadURL(ctx, userID, &record)
}

func (s *FileService) DownloadURLByObjectKey(ctx context.Context, userID int64, objectKey string) (string, error) {
	cleanKey, err := normalizeObjectKey(objectKey)
	if err != nil {
		return "", err
	}
	var record models.FileRecord
	if err := s.db.WithContext(ctx).Where("object_key = ?", cleanKey).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", err
	}
	return s.authorizedDownloadURL(ctx, userID, &record)
}

func (s *FileService) authorizedDownloadURL(ctx context.Context, userID int64, record *models.FileRecord) (string, error) {
	if record.Status != "uploaded" {
		return "", ErrFileNotFound
	}
	if record.RoomID > 0 {
		ok, err := s.membership.CheckIsMember(record.RoomID, userID)
		if err != nil {
			return "", fmt.Errorf("校验群成员身份: %w", err)
		}
		if !ok {
			return "", ErrFileForbidden
		}
	} else if record.UploaderID != userID {
		return "", ErrFileForbidden
	}
	url, err := s.storage.GetDownloadURL(ctx, record.ObjectKey)
	if err != nil {
		return "", ErrFileNotFound
	}
	return url, nil
}

func normalizeObjectKey(raw string) (string, error) {
	cleaned := strings.TrimPrefix(path.Clean(strings.TrimSpace(raw)), "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidFileKey
	}
	return cleaned, nil
}

func objectKeyBelongsToUser(objectKey string, userID int64) bool {
	parts := strings.SplitN(objectKey, "/", 3)
	return len(parts) == 3 && parts[1] == strconv.FormatInt(userID, 10)
}
