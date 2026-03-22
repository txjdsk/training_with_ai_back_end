package config

import (
	"errors"

	"github.com/spf13/viper"
)

type Config struct {
	App   AppConfig   `mapstructure:"app"`
	DB    DBConfig    `mapstructure:"db"`
	Redis RedisConfig `mapstructure:"redis"`
	Log   LogConfig   `mapstructure:"log"`
	LLM   LLMConfig   `mapstructure:"llm"`
	JWT   JWTConfig   // JWT 敏感配置仅从.env/环境变量取，不解析YAML
}

type AppConfig struct {
	Port string `mapstructure:"port"` // YAML: app.port
	Env  string `mapstructure:"env"`  // YAML: app.env（dev/prod）
}

type DBConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`     // YAML: redis.addr
	Password string `mapstructure:"password"` // 当前无密码
	DB       int    `mapstructure:"db"`       // YAML: redis.db
}

type LogConfig struct {
	LogPath          string `mapstructure:"log_path"`
	LogLevel         string `mapstructure:"log_level"`
	MaxSize          int    `mapstructure:"max_size"`
	MaxBackups       int    `mapstructure:"max_backups"`
	MaxAge           int    `mapstructure:"max_age"`
	Compress         bool   `mapstructure:"compress"`
	RequestLogDetail bool   `mapstructure:"request_log_detail"`
}

type JWTConfig struct {
	SecretKey   string
	ExpireHours int
}

type LLMConfig struct {
	BaseURL        string  `mapstructure:"base_url"`
	Model          string  `mapstructure:"model"`
	Temperature    float64 `mapstructure:"temperature"`
	MaxTokens      int     `mapstructure:"max_tokens"`
	TimeoutSeconds int     `mapstructure:"timeout_seconds"`
	APIKey         string
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env") // 指定配置文件路径
	viper.AutomaticEnv()        // 允许读取环境变量

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	yamlConfigPath := viper.GetString("CONFIG_PATH")

	if yamlConfigPath != "" {
		viper.SetConfigFile(yamlConfigPath)
	} else {
		viper.SetConfigFile("./config.yaml") // 默认配置文件路径
	}

	yamlViper := viper.New()
	yamlViper.SetConfigFile(yamlConfigPath)
	yamlViper.SetConfigType("yaml")

	if err := yamlViper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config

	if err := yamlViper.Unmarshal(&cfg); err != nil {
		return nil, errors.New("Failed to unmarshal YAML config")
	}

	cfg.DB.DSN = viper.GetString("DB_DSN")
	if cfg.DB.DSN == "" {
		return nil, errors.New("DB_DSN is required")
	}

	cfg.JWT.SecretKey = viper.GetString("JWT_SECRET_KEY")
	if cfg.JWT.SecretKey == "" {
		return nil, errors.New("JWT_SECRET_KEY is required")
	}

	cfg.LLM.APIKey = viper.GetString("LLM_API_KEY")
	if cfg.LLM.APIKey == "" {
		return nil, errors.New("LLM_API_KEY is required")
	}

	cfg.JWT.ExpireHours = yamlViper.GetInt("jwt.expire_hours")
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 72 // 兜底默认72小时
	}

	if cfg.App.Port == "" {
		cfg.App.Port = "8080" // 兜底默认端口
	}
	if cfg.Log.LogPath == "" {
		cfg.Log.LogPath = "./logs/app.log" // 兜底默认日志路径
	}

	return &cfg, nil
}
