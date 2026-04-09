package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"training_with_ai/internal/model/dto"
	"training_with_ai/internal/model/entity"
	"training_with_ai/internal/pkg/calc"
	"training_with_ai/internal/pkg/chroma"
	"training_with_ai/internal/pkg/constants"
	"training_with_ai/internal/pkg/llm"
	"training_with_ai/internal/pkg/logger"
	"training_with_ai/internal/repository"

	config "training_with_ai/configs"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 1. 定义接口
type SessionService interface {
	CreateSession(ctx context.Context, userID uint64, req *dto.CreateSessionReq) (*dto.CreateSessionResp, error)
	OpenStream(ctx context.Context, sessionID string) (<-chan dto.SessionSSEEvent, error)
	HandleStreamDisconnect(ctx context.Context, sessionID string)
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
	streams  map[string]chan dto.SessionSSEEvent
}

// 3. 构造函数：注入 repo 接口，返回 service 接口
func NewSessionService(repo repository.SessionRepository, promptRepo repository.PromptRepository) SessionService {
	return &sessionService{
		repo:       repo,
		promptRepo: promptRepo,
		streams:    make(map[string]chan dto.SessionSSEEvent),
	}
}

// ---------------- 以下为接口方法的具体实现示例 ----------------
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

	basePrompts := make(map[uint]entity.Prompt)
	for _, categoryID := range []uint{0, 1, 2} {
		prompts, err := s.promptRepo.GetPromptsByCategory(ctx, categoryID)
		if err != nil {
			return nil, err
		}
		if len(prompts) != 1 {
			return nil, constants.ErrPromptSelectionBad
		}
		basePrompts[categoryID] = prompts[0]
	}

	rolePrompt, scenePrompt, err := splitRoleScenePrompt(selected)
	if err != nil {
		return nil, err
	}

	fullPrompt := buildFullPrompt(basePrompts, rolePrompt, scenePrompt)
	preview := strings.Join([]string{rolePrompt.Content, scenePrompt.Content}, "\n")
	usedPromptIDs := []uint64{
		uint64(basePrompts[0].ID),
		uint64(basePrompts[1].ID),
		uint64(basePrompts[2].ID),
		uint64(rolePrompt.ID),
		uint64(scenePrompt.ID),
	}

	initAnger := initialAngerForDifficulty(req.Difficulty)
	sessionID := uuid.NewString()
	cache := dto.SessionCache{
		SessionID:     sessionID,
		UserID:        userID,
		UsedPromptIDs: usedPromptIDs,
		PromptText:    fullPrompt,
		Preview:       preview,
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

func (s *sessionService) OpenStream(ctx context.Context, sessionID string) (<-chan dto.SessionSSEEvent, error) {
	cache, err := s.getSessionCache(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if ch, ok := s.streams[sessionID]; ok {
		return ch, nil
	}
	ch := make(chan dto.SessionSSEEvent, 8)
	s.streams[sessionID] = ch
	if cache.Preview != "" {
		select {
		case ch <- dto.SessionSSEEvent{Event: "preview", Preview: cache.Preview}:
		default:
		}
	}
	return ch, nil
}

// HandleStreamDisconnect 仅在 SSE 异常断开时调用：标记异常并将 Redis TTL 缩短至 5 分钟
func (s *sessionService) HandleStreamDisconnect(ctx context.Context, sessionID string) {
	cache, err := s.getSessionCache(ctx, sessionID)
	if err == nil {
		if cache.Status == "ongoing" {
			cache.Status = "abnormal"
			cache.LastActiveAt = time.Now().UTC()
			if data, marshalErr := json.Marshal(cache); marshalErr == nil {
				_ = s.repo.SetSessionCache(ctx, sessionID, data, 5*time.Minute)
			}
		}
	}

	s.closeStream(sessionID)
}

func (s *sessionService) SendMessage(ctx context.Context, sessionID string, msg string) (*dto.ChatResponse, error) {
	logger.Info("SendMessage called", zap.String("sessionID", sessionID), zap.String("msg", msg))
	cache, err := s.getSessionCache(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if cache.Status != "ongoing" {
		return nil, constants.ErrSessionNotOngoing
	}

	client, err := s.getLLMClient()
	if err != nil {
		return nil, err
	}

	history := buildDialogueHistory(cache.DialogueLog)
	customerMsgRaw, err := client.GenerateCustomerReply(ctx, cache.PromptText, history, msg)
	if err != nil {
		return nil, constants.ErrLLMRequestFailed
	}

	customerMsg, label := extractEmotionTag(customerMsgRaw)
	label = normalizeEmotionLabel(label)
	if label == "" {
		label = "一般积极"
		logger.Debugw("emotion tag not found, fallback used",
			"session_id", sessionID,
			"round", cache.TurnCount+1,
		)
	}
	if customerMsg == "" {
		customerMsg = strings.TrimSpace(customerMsgRaw)
	}

	angerBefore := cache.CurrentAnger
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
		CustomerMsg:       customerMsg,
		CustomerSentiment: label,
		AngerBefore:       angerBefore,
		AngerDelta:        angerDelta,
		AngerAfter:        angerAfter,
		ExpertCritique:    "",
		PolishReply:       "",
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
	if cache.Status == "ongoing" {
		dialogueText := buildRoundDialogueText(msg, customerMsg)
		go s.runAsyncReview(sessionID, round.Round, dialogueText)
	}
	return resp, nil
}

func (s *sessionService) TerminateSession(ctx context.Context, sessionID string) error {
	cache, err := s.getSessionCache(ctx, sessionID)
	if err != nil {
		return err
	}

	if cache.Status == "ongoing" {
		cache.Status = "abnormal"
		cache.LastActiveAt = time.Now().UTC()
		data, marshalErr := json.Marshal(cache)
		if marshalErr != nil {
			return marshalErr
		}
		if err := s.repo.SetSessionCache(ctx, sessionID, data, 5*time.Minute); err != nil {
			return err
		}
	} else {
		if err := s.repo.UpdateSessionTTL(ctx, sessionID, 5*time.Minute); err != nil {
			return err
		}
	}

	s.closeStream(sessionID)
	return nil
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
		return toAdminRecordDetail(ctx, s.promptRepo, record)
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

	var startTime *time.Time
	if req.StartTime != "" {
		parsed, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			return nil, constants.ErrParamInvalid
		}
		startTime = &parsed
	}
	var endTime *time.Time
	if req.EndTime != "" {
		parsed, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			return nil, constants.ErrParamInvalid
		}
		endTime = &parsed
	}

	records, total, err := s.repo.ListRecords(ctx, filterUserID, req.Username, req.MinScore, req.MaxScore, promptID, startTime, endTime, req.Page, req.Size)
	if err != nil {
		return nil, err
	}

	items := make([]dto.RecordListItemResp, 0, len(records))
	for _, record := range records {
		item := dto.RecordListItemResp{
			ID:         formatRecordID(record.ID),
			Score:      record.Score,
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

func (s *sessionService) getLLMClient() (*llm.Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, constants.ErrLLMConfigMissing
	}
	client, err := llm.NewClient(cfg.LLM)
	if err != nil {
		return nil, constants.ErrLLMConfigMissing
	}
	return client, nil
}

func (s *sessionService) getChromaClient() (*chroma.Client, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, constants.ErrServerInternal
	}
	client, err := chroma.NewClient(cfg.Chroma)
	if err != nil {
		return nil, constants.ErrServerInternal
	}
	return client, nil
}

func splitRoleScenePrompt(selected []entity.Prompt) (entity.Prompt, entity.Prompt, error) {
	var rolePrompt entity.Prompt
	var scenePrompt entity.Prompt
	for _, prompt := range selected {
		if prompt.CategoryID == 6 {
			rolePrompt = prompt
			continue
		}
		if prompt.CategoryID >= 7 {
			scenePrompt = prompt
		}
	}
	if rolePrompt.ID == 0 || scenePrompt.ID == 0 {
		return entity.Prompt{}, entity.Prompt{}, constants.ErrPromptSelectionBad
	}
	return rolePrompt, scenePrompt, nil
}

func buildFullPrompt(basePrompts map[uint]entity.Prompt, rolePrompt entity.Prompt, scenePrompt entity.Prompt) string {
	parts := []string{
		basePrompts[0].Content,
		basePrompts[1].Content,
		basePrompts[2].Content,
		rolePrompt.Content,
		scenePrompt.Content,
	}
	return strings.Join(parts, "\n\n")
}

func buildDialogueHistory(logs []dto.DialogueRound) string {
	if len(logs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(logs)*2)
	for _, round := range logs {
		if round.UserMsg != "" {
			parts = append(parts, "客服: "+round.UserMsg)
		}
		if round.CustomerMsg != "" {
			parts = append(parts, "顾客: "+round.CustomerMsg)
		}
	}
	return strings.Join(parts, "\n")
}

func buildRoundDialogueText(userMsg string, customerMsg string) string {
	return "客服: " + userMsg + "\n顾客: " + customerMsg
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
		Score:         score,
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
	if !ok {
		s.streamMu.Unlock()
		return
	}
	event := dto.SessionSSEEvent{
		Event:        "reply",
		Round:        resp.Round.Round,
		CustomerMsg:  resp.Round.CustomerMsg,
		CurrentAnger: resp.CurrentAnger,
		MaxAnger:     resp.MaxAnger,
		TurnCount:    resp.TurnCount,
		Status:       resp.Status,
	}

	select {
	case ch <- event:
	default:
	}
	s.streamMu.Unlock()
}

func (s *sessionService) publishReview(sessionID string, round int, review string, polish string, answer string, cache *dto.SessionCache) {
	s.streamMu.Lock()
	ch, ok := s.streams[sessionID]
	if !ok {
		s.streamMu.Unlock()
		return
	}
	event := dto.SessionSSEEvent{
		Event:           "review",
		Round:           round,
		ExpertCritique:  review,
		PolishReply:     polish,
		ReferenceAnswer: answer,
		CurrentAnger:    cache.CurrentAnger,
		MaxAnger:        cache.MaxAnger,
		TurnCount:       cache.TurnCount,
		Status:          cache.Status,
	}

	select {
	case ch <- event:
	default:
	}
	s.streamMu.Unlock()
}

func (s *sessionService) closeStream(sessionID string) {
	s.streamMu.Lock()
	ch, ok := s.streams[sessionID]
	if ok {
		delete(s.streams, sessionID)
	}
	s.streamMu.Unlock()
	if ok {
		close(ch)
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
	case "一般消极":
		return int(2 * negMult)
	case "消极":
		return int(5 * negMult)
	default:
		return 0
	}
}

func extractEmotionTag(text string) (string, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ""
	}
	lower := strings.ToLower(trimmed)
	idx := strings.LastIndex(lower, "[emotion:")
	if idx == -1 {
		return trimmed, ""
	}
	if !strings.HasSuffix(lower, "]") {
		return trimmed, ""
	}
	label := strings.TrimSpace(trimmed[idx+len("[Emotion:") : len(trimmed)-1])
	content := strings.TrimSpace(trimmed[:idx])
	return content, label
}

func normalizeEmotionLabel(label string) string {
	switch strings.TrimSpace(label) {
	case "积极":
		return "积极"
	case "一般积极":
		return "一般积极"
	case "一般消极":
		return "一般消极"
	case "消极":
		return "消极"
	case "一般负面":
		return "一般消极"
	case "负面":
		return "消极"
	default:
		return ""
	}
}

func (s *sessionService) runAsyncReview(sessionID string, round int, dialogueText string) {
	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
		err := s.evaluateRound(ctx, sessionID, round, dialogueText)
		cancel()
		if err == nil {
			return
		}
		logger.Warn("async critique failed",
			zap.String("session_id", sessionID),
			zap.Int("round", round),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
	}
}

func (s *sessionService) evaluateRound(ctx context.Context, sessionID string, round int, dialogueText string) error {
	cache, err := s.getSessionCache(ctx, sessionID)
	if err != nil {
		return err
	}
	if round <= 0 || round > len(cache.DialogueLog) {
		return fmt.Errorf("round not found")
	}

	client, err := s.getLLMClient()
	if err != nil {
		return err
	}

	query, err := client.RewriteQuery(ctx, dialogueText)
	if err != nil {
		return err
	}
	embedding, err := client.Embed(ctx, query)
	if err != nil {
		return err
	}

	chromaClient, err := s.getChromaClient()
	if err != nil {
		return err
	}
	items, err := chromaClient.Query(ctx, embedding)
	if err != nil {
		return err
	}
	etiquetteText := ""
	if len(items) > 0 {
		etiquetteText = strings.Join(items, "\n\n")
	}
	review, err := client.Critique(ctx, dialogueText, etiquetteText)
	if err != nil {
		return err
	}

	idx := round - 1
	updated := cache.DialogueLog[idx]
	updated.ExpertCritique = review.Critique
	updated.PolishReply = review.PolishReply
	updated.ReferenceAnswer = review.ReferenceAnswer
	cache.DialogueLog[idx] = updated
	cache.LastActiveAt = time.Now().UTC()

	if err := s.saveSessionCache(ctx, sessionID, cache); err != nil {
		return err
	}

	s.publishReview(sessionID, round, review.Critique, review.PolishReply, review.ReferenceAnswer, cache)
	return nil
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
	if turns > 100 {
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
			PolishReply:       round.PolishReply,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}
	return result
}

func toAdminRecordDetail(ctx context.Context, promptRepo repository.PromptRepository, record *entity.TrainingRecord) (*dto.AdminRecordDetailResp, error) {
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
			PolishReply:       round.PolishReply,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}

	ids := toUint64Slice(record.UsedPromptIDs)
	prompts, err := promptRepo.GetPromptsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	promptByID := make(map[uint64]entity.Prompt, len(prompts))
	for _, prompt := range prompts {
		promptByID[uint64(prompt.ID)] = prompt
	}
	notes := make([]string, 0, len(prompts))
	previewParts := make([]string, 0, 2)
	for _, id := range ids {
		prompt, ok := promptByID[id]
		if !ok {
			continue
		}
		if prompt.Note != nil {
			notes = append(notes, *prompt.Note)
		}
		if prompt.CategoryID >= 6 && prompt.Content != "" {
			previewParts = append(previewParts, prompt.Content)
		}
	}
	preview := strings.Join(previewParts, "\n")

	return &dto.AdminRecordDetailResp{
		ID:            formatRecordID(record.ID),
		UserID:        uint64(record.UserID),
		Username:      record.User.Username,
		Score:         record.Score,
		UsedPromptIDs: toUint64Slice(record.UsedPromptIDs),
		PromptsNotes:  notes,
		Preview:       preview,
		DialogueLog:   log,
		FinishedAt:    record.FinishedAt,
		Duration:      record.Duration,
	}, nil
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
			PolishReply:       round.PolishReply,
			ReferenceAnswer:   round.ReferenceAnswer,
		})
	}

	ids := toUint64Slice(record.UsedPromptIDs)
	prompts, err := promptRepo.GetPromptsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	promptByID := make(map[uint64]entity.Prompt, len(prompts))
	for _, prompt := range prompts {
		promptByID[uint64(prompt.ID)] = prompt
	}
	notes := make([]string, 0, len(prompts))
	previewParts := make([]string, 0, 2)
	for _, id := range ids {
		prompt, ok := promptByID[id]
		if !ok {
			continue
		}
		if prompt.Note != nil {
			notes = append(notes, *prompt.Note)
		}
		if prompt.CategoryID >= 6 && prompt.Content != "" {
			previewParts = append(previewParts, prompt.Content)
		}
	}
	preview := strings.Join(previewParts, "\n")

	return &dto.UserRecordDetailResp{
		ID:            formatRecordID(record.ID),
		Score:         record.Score,
		UsedPromptIDs: ids,
		Preview:       preview,
		DialogueLog:   log,
		FinishedAt:    record.FinishedAt,
		Duration:      record.Duration,
		PromptsNotes:  notes,
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
