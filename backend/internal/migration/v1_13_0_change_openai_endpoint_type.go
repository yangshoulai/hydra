package migration

import (
	"gorm.io/gorm"
)

// V1_13_0_ChangeOpenaiEndpointType 把 openai 换成 openai-chat
func V1_13_0_ChangeOpenaiEndpointType(db *gorm.DB) error {
	statements := []string{
		`UPDATE channel_model_configs SET endpoint_types = REPLACE(endpoint_types, '"openai"', '"openai-chat"') WHERE endpoint_types LIKE '%"openai"%'`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
