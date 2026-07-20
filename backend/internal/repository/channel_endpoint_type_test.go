package repository

import (
	"context"
	"testing"

	"github.com/yangshoulai/hydra/internal/endpoint"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFindByModelMatchesEndpointTypeExactly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Channel{},
		&models.ChannelKey{},
		&models.ChannelModelConfig{},
		&models.ChannelModelConfigEndpointType{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	ctx := context.Background()
	channelRepo := NewChannelRepository(db)
	keyRepo := NewChannelKeyRepository(db)
	configRepo := NewChannelModelConfigRepository(db)

	channel := &models.Channel{
		Name:    "test-channel",
		BaseURL: "https://example.com",
		Status:  "active",
		Weight:  100,
	}
	if err := channelRepo.Create(ctx, channel); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := keyRepo.Create(ctx, &models.ChannelKey{
		ChannelID:       channel.ID,
		ChannelKeyValue: "sk-test",
		Status:          "active",
		ChannelKeyGroup: "Default",
	}); err != nil {
		t.Fatalf("create key: %v", err)
	}

	nearMiss := &models.ChannelModelConfig{
		ChannelID:     channel.ID,
		Model:         "gpt-test",
		ChannelModel:  "near-miss",
		Weight:        100,
		Status:        "active",
		EndpointTypes: models.EndpointTypes{endpoint.TypeOpenAIResponses + "Legacy"},
		KeyGroups:     models.KeyGroups{"Default"},
	}
	exact := &models.ChannelModelConfig{
		ChannelID:     channel.ID,
		Model:         "gpt-test",
		ChannelModel:  "exact",
		Weight:        100,
		Status:        "active",
		EndpointTypes: models.EndpointTypes{endpoint.TypeOpenAIResponses},
		KeyGroups:     models.KeyGroups{"Default"},
	}
	if err := configRepo.Create(ctx, nearMiss); err != nil {
		t.Fatalf("create near miss config: %v", err)
	}
	if err := configRepo.Create(ctx, exact); err != nil {
		t.Fatalf("create exact config: %v", err)
	}

	channels, err := channelRepo.FindByModel(ctx, "gpt-test", endpoint.TypeOpenAIResponses, false)
	if err != nil {
		t.Fatalf("find by model: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected one channel, got %d", len(channels))
	}
	if len(channels[0].ModelConfigs) != 1 {
		t.Fatalf("expected only exact config to be preloaded, got %d", len(channels[0].ModelConfigs))
	}
	if channels[0].ModelConfigs[0].ID != exact.ID {
		t.Fatalf("expected exact config id %d, got %d", exact.ID, channels[0].ModelConfigs[0].ID)
	}
}
