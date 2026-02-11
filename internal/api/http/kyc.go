package api

import (
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/maxonlinux/meta-terminal-go/internal/kyc"
	"github.com/maxonlinux/meta-terminal-go/internal/users"
	"github.com/maxonlinux/meta-terminal-go/pkg/config"
	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

const (
	kycStatusPending  = "PENDING"
	kycStatusApproved = "APPROVED"
	kycStatusRejected = "REJECTED"
)

type KYCHandler struct {
	repo  *kyc.Repository
	users *users.Service
}

func NewKYCHandler(repo *kyc.Repository, usersService *users.Service) *KYCHandler {
	return &KYCHandler{repo: repo, users: usersService}
}

type KYCFileResponse struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type KYCResponse struct {
	ID           int64             `json:"id"`
	UserID       uint64            `json:"userId"`
	DocType      string            `json:"docType"`
	Country      string            `json:"country"`
	Status       string            `json:"status"`
	RejectReason *string           `json:"rejectReason"`
	CreatedAt    uint64            `json:"createdAt"`
	UpdatedAt    uint64            `json:"updatedAt"`
	Files        []KYCFileResponse `json:"files"`
}

type KYCListItem struct {
	KYCResponse
	User map[string]interface{} `json:"user"`
}

type KYCUpdateRequest struct {
	Status       string  `json:"status"`
	RejectReason *string `json:"rejectReason"`
}

func (h *KYCHandler) GetUserKYC(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	rec, files, err := h.repo.GetRequestByUser(claims.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	}
	if rec == nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, toKYCResponse(rec, files))
}

func (h *KYCHandler) SubmitKYC(c *echo.Context) error {
	claims := getUser(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	if h.repo == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
	}
	if existing, _, err := h.repo.GetRequestByUser(claims.UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	} else if existing != nil {
		if existing.Status == kycStatusPending {
			return c.JSON(http.StatusConflict, map[string]string{"error": "kyc already pending"})
		}
		if existing.Status == kycStatusApproved {
			return c.JSON(http.StatusConflict, map[string]string{"error": "kyc already approved"})
		}
	}
	if err := c.Request().ParseMultipartForm(25 << 20); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid form"})
	}

	docType := strings.TrimSpace(c.FormValue("docType"))
	country := strings.TrimSpace(c.FormValue("country"))
	if docType == "" || country == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "docType and country are required"})
	}

	frontFile, frontHeader, _ := c.Request().FormFile("front")
	if frontFile == nil || frontHeader == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "front file is required"})
	}
	defer func() {
		_ = frontFile.Close()
	}()

	selfieFile, selfieHeader, _ := c.Request().FormFile("selfie")
	if selfieFile == nil || selfieHeader == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "selfie file is required"})
	}
	defer func() {
		_ = selfieFile.Close()
	}()

	backFile, backHeader, _ := c.Request().FormFile("back")
	if backFile != nil {
		defer func() {
			_ = backFile.Close()
		}()
	}

	kycID := snowflake.Next()
	now := utils.NowNano()

	baseDir := filepath.Join(kycDataDir(), "users", formatUserID(claims.UserID), formatInt64(kycID))
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create kyc dir"})
	}

	files := make([]kyc.FileRecord, 0, 3)
	frontRecord, err := saveKYCFile(baseDir, kycID, "front", frontHeader, frontFile, now)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	files = append(files, frontRecord)

	selfieRecord, err := saveKYCFile(baseDir, kycID, "selfie", selfieHeader, selfieFile, now)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	files = append(files, selfieRecord)

	if backFile != nil && backHeader != nil {
		backRecord, err := saveKYCFile(baseDir, kycID, "back", backHeader, backFile, now)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		files = append(files, backRecord)
	}

	rec := kyc.RequestRecord{
		ID:        kycID,
		UserID:    claims.UserID,
		DocType:   docType,
		Country:   country,
		Status:    kycStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.repo.CreateRequest(rec, files); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save kyc"})
	}

	return c.JSON(http.StatusOK, toKYCResponse(&rec, files))
}

func (h *KYCHandler) ListRequests(c *echo.Context) error {
	status := strings.ToUpper(strings.TrimSpace(c.QueryParam("status")))
	if status != "" && status != kycStatusPending && status != kycStatusApproved && status != kycStatusRejected {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid status"})
	}
	limit, offset := parsePagination(c)
	query := strings.TrimSpace(c.QueryParam("q"))
	items, err := h.repo.ListRequests(status, limit, offset, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	}
	resp := make([]KYCListItem, 0, len(items))
	for _, item := range items {
		files, err := h.repo.ListFiles(item.ID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc files"})
		}
		user, _ := h.users.GetProfile(item.UserID)
		userPayload := map[string]interface{}{
			"id":       uint64(item.UserID),
			"username": "",
			"email":    "",
			"phone":    "",
		}
		if user != nil {
			userPayload["username"] = user.Username
			userPayload["email"] = user.Email
			userPayload["phone"] = user.Phone
		}
		resp = append(resp, KYCListItem{
			KYCResponse: toKYCResponse(&item, files),
			User:        userPayload,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *KYCHandler) GetRequest(c *echo.Context) error {
	kycID, err := parseKYCIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid kyc id"})
	}
	rec, files, err := h.repo.GetRequest(kycID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	}
	if rec == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "kyc not found"})
	}
	return c.JSON(http.StatusOK, toKYCResponse(rec, files))
}

func (h *KYCHandler) GetFile(c *echo.Context) error {
	kycID, err := parseKYCIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid kyc id"})
	}
	fileID, err := parseKYCIDParam(c.Param("fileId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid file id"})
	}
	_, files, err := h.repo.GetRequest(kycID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	}
	if files == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}
	for _, file := range files {
		if file.ID == fileID {
			return c.Attachment(file.Path, file.Filename)
		}
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
}

func (h *KYCHandler) UpdateRequest(c *echo.Context) error {
	kycID, err := parseKYCIDParam(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid kyc id"})
	}
	var req KYCUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status != kycStatusApproved && status != kycStatusRejected {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid status"})
	}
	if status == kycStatusRejected && (req.RejectReason == nil || strings.TrimSpace(*req.RejectReason) == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "rejectReason is required"})
	}
	if status == kycStatusApproved {
		req.RejectReason = nil
	}
	updatedAt := utils.NowNano()
	if err := h.repo.UpdateStatus(kycID, status, req.RejectReason, updatedAt); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update kyc"})
	}
	rec, files, err := h.repo.GetRequest(kycID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load kyc"})
	}
	if rec == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "kyc not found"})
	}
	return c.JSON(http.StatusOK, toKYCResponse(rec, files))
}

func saveKYCFile(baseDir string, kycID int64, kind string, header *multipart.FileHeader, file multipart.File, now uint64) (kyc.FileRecord, error) {
	allowed := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"application/pdf": true,
	}
	contentType := header.Header.Get("Content-Type")
	if !allowed[contentType] {
		return kyc.FileRecord{}, echo.NewHTTPError(http.StatusBadRequest, "unsupported file type")
	}
	if header.Size > 15<<20 {
		return kyc.FileRecord{}, echo.NewHTTPError(http.StatusBadRequest, "file too large")
	}

	filename := sanitizeFilename(header.Filename)
	fileID := snowflake.Next()
	path := filepath.Join(baseDir, kind+"_"+formatInt64(fileID)+"_"+filename)
	out, err := os.Create(path)
	if err != nil {
		return kyc.FileRecord{}, err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, file); err != nil {
		return kyc.FileRecord{}, err
	}

	return kyc.FileRecord{
		ID:          fileID,
		KYCID:       kycID,
		Kind:        kind,
		Filename:    filename,
		ContentType: contentType,
		Size:        header.Size,
		Path:        path,
		CreatedAt:   now,
	}, nil
}

func toKYCResponse(rec *kyc.RequestRecord, files []kyc.FileRecord) KYCResponse {
	resp := KYCResponse{
		ID:           rec.ID,
		UserID:       uint64(rec.UserID),
		DocType:      rec.DocType,
		Country:      rec.Country,
		Status:       rec.Status,
		RejectReason: rec.RejectReason,
		CreatedAt:    rec.CreatedAt,
		UpdatedAt:    rec.UpdatedAt,
		Files:        make([]KYCFileResponse, 0, len(files)),
	}
	for _, file := range files {
		resp.Files = append(resp.Files, KYCFileResponse{
			ID:          file.ID,
			Kind:        file.Kind,
			Filename:    file.Filename,
			ContentType: file.ContentType,
			Size:        file.Size,
		})
	}
	return resp
}

func kycDataDir() string {
	dataDir := config.Load().DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Join(dataDir, "kyc")
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func formatUserID(id types.UserID) string {
	return strconv.FormatUint(uint64(id), 10)
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func parseKYCIDParam(value string) (int64, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
