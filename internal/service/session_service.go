package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/calc"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/logger"
	"training_with_ai/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 1. 定义接口
type SessionService interface {
	CreateSession(ctx context.Context, userID uint64, req *dto.CreateSessionReq) (*dto.CreateSessionResp, error)
	OpenStream(ctx context.Context, sessionID string) (<-chan string, error)
	SendMessage(ctx context.Context, sessionID string, msg string) (*dto.ChatResponse, error)
	TerminateSession(ctx context.Context, sessionID string) error
	GetRecordDetail(ctx context.Context, recordID int64, isAdmin bool) (interface{}, error)
	ListRecords(ctx context.Context, userID int64, isAdmin bool, req dto.AdminRecordFilterReq) (*dto.PageResp, error)
	DeleteRecord(ctx context.Context, recordID int64, userID int64, isAdmin bool) error
}

// 2. 定义私有结构体
type sessionService struct {
	// 注入刚才定义的 Repository 接口
	repo       repository.SessionRepository
	promptRepo repository.PromptRepository

	streamMu sync.Mutex
	streams  map[string]chan string

	// 如果你封装了大模型调用的工具，也可以在这里注入，例如：
	// llmClient pkg.LLMClient
}

// 3. 构造函数：注入 repo 接口，返回 service 接口
func NewSessionService(repo repository.SessionRepository, promptRepo repository.PromptRepository) SessionService {
	return &sessionService{
		repo:       repo,
		promptRepo: promptRepo,
		streams:    make(map[string]chan string),
	}
}

// ---------------- 以下为接口方法的具体实现示例 ----------------
// TODO: 这里的实现只是示例，实际项目中可能需要更复杂的逻辑和错误处理
func (s *sessionService) CreateSession(ctx context.Context, userID uint64, req *dto.CreateSessionReq) (*dto.CreateSessionResp, error) {
	selected, err := s.promptRepo.GetPromptsByIDs(ctx, req.PromptIDs)
	if err != nil {
		return nil, err
	}
	if len(selected) != len(req.PromptIDs) {
		return nil, constants.ErrPromptSelectionBad
	}

	roleCount := 0
	sceneCount := 0
	for _, prompt := range selected {
		if prompt.CategoryID == 6 {
			roleCount++
			continue
		}
		if prompt.CategoryID >= 7 {
			sceneCount++
			continue
		}
		return nil, constants.ErrPromptSelectionBad
	}
	if roleCount != 1 || sceneCount != 1 {
		return nil, constants.ErrPromptSelectionBad
	}

	basePromptIDs := []uint64{}
	for _, categoryID := range []uint{0, 1, 2} {
		prompts, err := s.promptRepo.GetPromptsByCategory(ctx, categoryID)
		if err != nil {
			return nil, err
		}
		if len(prompts) != 1 {
			return nil, constants.ErrPromptSelectionBad
		}
		basePromptIDs = append(basePromptIDs, uint64(prompts[0].ID))
	}

	usedPromptIDs := append([]uint64{}, basePromptIDs...)
	usedPromptIDs = append(usedPromptIDs, req.PromptIDs...)

	initAnger := initialAngerForDifficulty(req.Difficulty)
	sessionID := uuid.NewString()
	cache := dto.SessionCache{
		SessionID:     sessionID,
		UserID:        userID,
		UsedPromptIDs: usedPromptIDs,
		Difficulty:    req.Difficulty,
		Status:        "ongoing",
		CurrentAnger:  initAnger,
		MaxAnger:      initAnger,
		TurnCount:     0,
		CreatedAt:     time.Now().UTC(),
		LastActiveAt:  time.Now().UTC(),
		DialogueLog:   []dto.DialogueRound{},
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetSessionCache(ctx, sessionID, data, 2*time.Hour); err != nil {
		return nil, err
	}

	return &dto.CreateSessionResp{SessionID: sessionID}, nil
}

func (s *sessionService) OpenStream(ctx context.Context, sessionID string) (<-chan string, error) {
	if _, err := s.getSessionCache(ctx, sessionID); err != nil {
		return nil, err
	}

	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if ch, ok := s.streams[sessionID]; ok {
		return ch, nil
	}
	ch := make(chan string, 8)
	s.streams[sessionID] = ch
	return ch, nil
}

func (s *sessionService) SendMessage(ctx context.Context, sessionID string, msg string) (*dto.ChatResponse, error) {
	logger.Info("SendMessage called", zap.String("sessionID", sessionID), zap.String("msg", msg))
	cache, err := s.getSessionCache(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	angerBefore := cache.CurrentAnger
	label := "一般积极"
	angerDelta := calcAngerDelta(angerBefore, label)
	angerAfter := clampAnger(angerBefore + angerDelta)

	cache.TurnCount++
	if angerAfter > cache.MaxAnger {
		cache.MaxAnger = angerAfter
	}
	cache.CurrentAnger = angerAfter
	cache.LastActiveAt = time.Now().UTC()
	cache.Status = resolveStatus(angerAfter, cache.TurnCount)

	round := dto.DialogueRound{
		Round:             cache.TurnCount,
		UserMsg:           msg,
		CustomerMsg:       "",
		CustomerSentiment: label,
		AngerBefore:       angerBefore,
		AngerDelta:        angerDelta,
		AngerAfter:        angerAfter,
		ExpertCritique:    "",
		ReferenceAnswer:   "",
	}
	cache.DialogueLog = append(cache.DialogueLog, round)

	if err := s.saveSessionCache(ctx, sessionID, cache); err != nil {
		return nil, err
	}

	resp := &dto.ChatResponse{
		Round:        round,
		Status:       cache.Status,
		CurrentAnger: cache.CurrentAnger,
		MaxAnger:     cache.MaxAnger,
		TurnCount:    cache.TurnCount,
	}

	if cache.Status != "ongoing" {
		if err := s.finishSession(ctx, cache); err != nil {
			return resp, err
		}
	}

	s.publishStream(sessionID, resp)
	return resp, nil
}

func (s *sessionService) TerminateSession(ctx context.Context, sessionID string) error {
	// 业务逻辑：异常退出，将 Redis 中的该会话 TTL 设置为 5 分钟 (300秒)
	return s.repo.UpdateSessionTTL(ctx, sessionID, 300*time.Second)
}

func (s *sessionService) GetRecordDetail(ctx context.Context, recordID int64, isAdmin bool) (interface{}, error) {
	record, err := s.repo.GetRecordByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constants.ErrSessionNotFound
		}
		return nil, err
	}

	if isAdmin {
		return toAdminRecordDetail(record), nil
	}

	return toUserRecordDetail(ctx, s.promptRepo, record)
}

func (s *sessionService) ListRecords(ctx context.Context, userID int64, isAdmin bool, req dto.AdminRecordFilterReq) (*dto.PageResp, error) {
	var filterUserID *int64
	if !isAdmin {
		filterUserID = &userID
	}

	var promptID *uint64
	if req.PromptID != 0 {
		promptID = &req.PromptID
	}

	records, total, err := s.repo.ListRecords(ctx, filterUserID, req.Username, req.MinScore, req.MaxScore, promptID, req.Page, req.Size)
	if err != nil {
		return nil, err
	}

	items := make([]dto.RecordListItemResp, 0, len(records))
	for _, record := range records {
		item := dto.RecordListItemResp{
			ID:         formatRecordID(record.ID),
			Score:      float64(record.Score),
			FinishedAt: record.FinishedAt,
			Duration:   record.Duration,
		}
		if isAdmin {
			item.Username = record.User.Username
		}
		items = append(items, item)
	}

	return &dto.PageResp{Total: total, List: items}, nil
}

func (s *sessionService) DeleteRecord(ctx context.Context, recordID int64, userID int64, isAdmin bool) error {
	record, err := s.repo.GetRecordByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return constants.ErrSessionNotFound
		}
		return err
	}
	if !isAdmin && record.UserID != userID {
		return constants.ErrNoPermission
	}
	return s.repo.DeleteRecord(ctx, recordID)
}

func (s *sessionService) getSessionCache(ctx context.Context, sessionID string) (*dto.SessionCache, error) {
	data, err := s.repo.GetSessionCache(ctx, sessionID)
	if err != nil {
		return nil, constants.ErrSessionNotFound
	}
	var cache dto.SessionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func (s *sessionService) saveSessionCache(ctx context.Context, sessionID string, cache *dto.SessionCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return s.repo.SetSessionCache(ctx, sessionID, data, 2*time.Hour)
}

func (s *sessionService) finishSession(ctx context.Context, cache *dto.SessionCache) error {
	input := calc.ScoreInput{
		EndAnger:  cache.CurrentAnger,
		PeakAnger: cache.MaxAnger,
		Turns:     cache.TurnCount,
	}
	score := calc.CalculateFinalScore(input)

	record := &entity.TrainingRecord{
		UserID:        int64(cache.UserID),
		Score:         int16(score),
		UsedPromptIDs: toIntSlice(cache.UsedPromptIDs),
		DialogueLog:   toDialogueMessages(cache.DialogueLog),
		FinishedAt:    time.Now().UTC(),
		Duration:      int(time.Since(cache.CreatedAt).Seconds()),
	}

	if err := s.repo.CreateRecord(ctx, record); err != nil {
		return err
	}

	return s.repo.DeleteSessionCache(ctx, cache.SessionID)
}

func (s *sessionService) publishStream(sessionID string, resp *dto.ChatResponse) {
	s.streamMu.Lock()
	ch, ok := s.streams[sessionID]
	s.streamMu.Unlock()
	if !ok {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	select {
	case ch <- string(data):
	default:
	}
}

func initialAngerForDifficulty(diff string) int {
	switch diff {
	case "low":
		return 40
	case "high":
		return 60
	default:
		return calc.StartAnger
	}
}

func calcAngerDelta(current int, label string) int {
	posMult, negMult := sensitivityMultiplier(current)
	switch label {
	case "积极":
		return int(-5 * posMult)
	case "一般积极":
		return int(-2 * posMult)
	case "一般负面":
		return int(2 * negMult)
	case "负面":
		return int(5 * negMult)
	default:
		return 0
	}
}

func sensitivityMultiplier(anger int) (float64, float64) {
	if anger >= 70 {
		return 0.5, 1.5
	}
	if anger <= 30 {
		return 1.2, 0.8
	}
	return 1.0, 1.0
}

func clampAnger(val int) int {
	if val < 0 {
		return 0
	}
	if val > 100 {
		return 100
	}
	return val
}

func resolveStatus(anger int, turns int) string {
	if anger <= calc.SuccessAngerThr {
		return "success"
	}
	if anger >= calc.FailAngerThr {
		return "failed"
	}
	if turns >= 30 {
		return "timeout"
	}
	return "ongoing"
}

func toIntSlice(values []uint64) []int {
	result := make([]int, 0, len(values))
	for _, value := range values {
		result = append(result, int(value))
	}
	return result
}

func toDialogueMessages(rounds []dto.DialogueRound) []entity.DialogueMessage {
	result := make([]entity.DialogueMessage, 0, len(rounds))
	for _, round := range rounds {
		result = append(result, entity.DialogueMessage{
			Round:             round.Round,
			UserMsg:           round.UserMsg,
			CustomerMsg:       round.CustomerMsg,
			CustomerSentiment: round.CustomerSentiment,
			AngerBefore:       round.AngerBefore,
			AngerDelta:        round.AngerDelta,
			AngerAfter:        round.AngerAfter,
			ExpertCritique:    round.ExpertCritique,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}
	return result
}

func toAdminRecordDetail(record *entity.TrainingRecord) *dto.AdminRecordDetailResp {
	log := make([]dto.DialogueRound, 0, len(record.DialogueLog))
	for _, round := range record.DialogueLog {
		log = append(log, dto.DialogueRound{
			Round:             round.Round,
			UserMsg:           round.UserMsg,
			CustomerMsg:       round.CustomerMsg,
			CustomerSentiment: round.CustomerSentiment,
			AngerBefore:       round.AngerBefore,
			AngerDelta:        round.AngerDelta,
			AngerAfter:        round.AngerAfter,
			ExpertCritique:    round.ExpertCritique,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}

	return &dto.AdminRecordDetailResp{
		ID:            formatRecordID(record.ID),
		UserID:        uint64(record.UserID),
		Username:      record.User.Username,
		Score:         float64(record.Score),
		UsedPromptIDs: toUint64Slice(record.UsedPromptIDs),
		DialogueLog:   log,
		FinishedAt:    record.FinishedAt,
		Duration:      record.Duration,
	}
}

func toUserRecordDetail(ctx context.Context, promptRepo repository.PromptRepository, record *entity.TrainingRecord) (*dto.UserRecordDetailResp, error) {
	log := make([]dto.DialogueRound, 0, len(record.DialogueLog))
	for _, round := range record.DialogueLog {
		log = append(log, dto.DialogueRound{
			Round:             round.Round,
			UserMsg:           round.UserMsg,
			CustomerMsg:       round.CustomerMsg,
			CustomerSentiment: round.CustomerSentiment,
			AngerBefore:       round.AngerBefore,
			AngerDelta:        round.AngerDelta,
			AngerAfter:        round.AngerAfter,
			ExpertCritique:    round.ExpertCritique,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}

	ids := toUint64Slice(record.UsedPromptIDs)
	prompts, err := promptRepo.GetPromptsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	notes := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		if prompt.Note != nil {
			notes = append(notes, *prompt.Note)
		}
	}

	return &dto.UserRecordDetailResp{
		ID:           formatRecordID(record.ID),
		Score:        float64(record.Score),
		DialogueLog:  log,
		FinishedAt:   record.FinishedAt,
		Duration:     record.Duration,
		PromptsNotes: notes,
	}, nil
}

func toUint64Slice(values []int) []uint64 {
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		result = append(result, uint64(value))
	}
	return result
}

func formatRecordID(id int64) string {
	return fmt.Sprintf("%d", id)
}
