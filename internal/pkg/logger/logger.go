package logger

import (
	config "training_with_ai/configs"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Log   *zap.Logger        = zap.NewNop()
	Sugar *zap.SugaredLogger = Log.Sugar()
)

func InitLogger() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	level, err := zapcore.ParseLevel(cfg.Log.LogLevel)
	if err != nil {
		level = zapcore.InfoLevel // 解析失败默认 info
	}
	// 配置 lumberjack 进行日志切割
	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Log.LogPath,    // 日志文件路径
		MaxSize:    cfg.Log.MaxSize,    // 每个文件最大 100 MB
		MaxBackups: cfg.Log.MaxBackups, // 最多保留 30 个备份
		MaxAge:     cfg.Log.MaxAge,     // 最多保留 30 天
		Compress:   cfg.Log.Compress,   // 是否压缩
	})

	// 编码器配置 (JSON 格式，适合生产环境)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder   // 人类可读的时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // 大写日志级别
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // 短文件名和行号
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writeSyncer,
		level,
	)

	Log = zap.New(core, zap.AddCaller()) // AddCaller 会记录是哪行代码打印的日志
	Sugar = Log.Sugar()                  // 方便使用的 SugarLogger
}

func Sync() {
	if Log.Core().Enabled(zapcore.DebugLevel) || Log != zap.NewNop() {
		if err := Log.Sync(); err != nil {
			Sugar.Warn("Failed to sync logger", "error", err)
		}
	}
}

func getLogger() *zap.Logger {
	return Log
}

func getSugar() *zap.SugaredLogger {
	return Sugar
}

// 类型安全风格
func Debug(msg string, fields ...zap.Field) { getLogger().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { getLogger().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { getLogger().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { getLogger().Error(msg, fields...) }

// 键值对风格
func Debugw(msg string, keysAndValues ...any) { getSugar().Debugw(msg, keysAndValues...) }
func Infow(msg string, keysAndValues ...any)  { getSugar().Infow(msg, keysAndValues...) }
func Warnw(msg string, keysAndValues ...any)  { getSugar().Warnw(msg, keysAndValues...) }
func Errorw(msg string, keysAndValues ...any) { getSugar().Errorw(msg, keysAndValues...) }
