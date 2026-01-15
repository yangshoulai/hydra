package migration

import (
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/yangshoulai/hydra/internal/models"
	"gorm.io/gorm"
)

// v1_1_2_update_token_and_key_preview_format 更新 token_preview 格式
// 从 "前8位+***+后4位" 改为 "前6位+**********+后4位"
func v1_1_2_update_token_and_key_preview_format() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "v1.1.2-update-token-preview-format",
		Migrate: func(tx *gorm.DB) error {
			// 更新 access_tokens 表的 token_preview
			if tx.Migrator().HasTable(&models.AccessToken{}) {
				// 查询所有令牌
				type AccessTokenPreview struct {
					ID          uint
					TokenPreview string
				}

				var tokens []AccessTokenPreview
				if err := tx.Table("access_tokens").Select("id, token_preview").Find(&tokens).Error; err != nil {
					return err
				}

				// 更新每个令牌的预览
				for _, token := range tokens {
					oldPreview := token.TokenPreview

					// 检查是否需要转换（包含***且不是新格式）
					if strings.Contains(oldPreview, "***") && !strings.Contains(oldPreview, "**********") {
						// 旧格式：前8位 + *** + 后4位
						// 提取前6位和后4位
						prefix := oldPreview[:min(6, len(oldPreview))]
						suffix := oldPreview[max(len(oldPreview)-4, 0):]

						// 生成新格式：前6位 + ********** + 后4位
						newPreview := prefix + "**********" + suffix

						// 更新数据库
						if err := tx.Table("access_tokens").
							Where("id = ?", token.ID).
							Update("token_preview", newPreview).Error; err != nil {
							return err
						}
					}
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：从新格式改回旧格式
			if tx.Migrator().HasTable(&models.AccessToken{}) {
				// 查询所有令牌
				type AccessTokenPreview struct {
					ID          uint
					TokenPreview string
				}

				var tokens []AccessTokenPreview
				if err := tx.Table("access_tokens").Select("id, token_preview").Find(&tokens).Error; err != nil {
					return err
				}

				// 更新每个令牌的预览
				for _, token := range tokens {
					newPreview := token.TokenPreview

					// 检查是否是新格式（包含**********）
					if strings.Contains(newPreview, "**********") {
						// 新格式：前6位 + ********** + 后4位
						// 提取前6位和后4位
						prefix := newPreview[:min(6, len(newPreview))]
						suffix := newPreview[max(len(newPreview)-4, 0):]

						// 生成旧格式：前8位 + *** + 后4位
						// 由于我们丢失了第7-8位的信息，用前6位的后2位补充
						oldPrefix := prefix + prefix[max(len(prefix)-2, 0):]
						oldPreview := oldPrefix + "***" + suffix

						// 更新数据库
						if err := tx.Table("access_tokens").
							Where("id = ?", token.ID).
							Update("token_preview", oldPreview).Error; err != nil {
							return err
						}
					}
				}
			}

			return nil
		},
	}
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
