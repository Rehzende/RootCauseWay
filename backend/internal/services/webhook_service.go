package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// WebhookRepository defines the DB operations for webhooks
type WebhookRepository interface {
	Create(ctx context.Context, webhook *models.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	GetByToken(ctx context.Context, token string) (*models.Webhook, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type WebhookService struct {
	repo WebhookRepository
}

func NewWebhookService(repo WebhookRepository) *WebhookService {
	return &WebhookService{repo: repo}
}

func (s *WebhookService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateWebhookRequest) (*models.Webhook, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	secret, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	webhook := &models.Webhook{
		ID:            uuid.New(),
		OrgID:         orgID,
		Name:          req.Name,
		Source:        req.Source,
		SoftwareID:    req.SoftwareID,
		EndpointToken: token,
		Secret:        secret,
		Enabled:       true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.Create(ctx, webhook); err != nil {
		return nil, err
	}
	return webhook, nil
}

func (s *WebhookService) GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WebhookService) GetByToken(ctx context.Context, token string) (*models.Webhook, error) {
	return s.repo.GetByToken(ctx, token)
}

func (s *WebhookService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, page, perPage)
}

func (s *WebhookService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
