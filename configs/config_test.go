package config_test

import (
	"testing"
	config "training_with_ai/configs"

	"github.com/stretchr/testify/assert"
	// 使用testify的断言
)

func TestLoadConfig(t *testing.T) {
	cfg, err := config.LoadConfig()
	print(cfg.App.Port)
	assert.NoError(t, err)
}
