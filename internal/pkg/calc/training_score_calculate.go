package calc

import (
	"math"
)

// Config 定义评分系统的基础配置参数
const (
	StartAnger      = 50
	SuccessAngerThr = 10 // 成功阈值 (<=)
	FailAngerThr    = 90 // 失败阈值 (>=)

	MinTurnThr  = 8    // 最小轮次 (抢答区)
	OptTurnThr  = 15   // 最佳轮次上限 (黄金区)
	DecayFactor = 0.95 // 拖沓区的衰减系数
)

// ScoreInput 定义计算所需的输入参数
type ScoreInput struct {
	EndAnger  int // 最终怒气值
	PeakAnger int // 历史最高怒气值 (用于判断过程是否惊险)
	Turns     int // 对话总轮次
}

// CalculateFinalScore 计算最终得分
// 返回值保留一位小数，且按 0.5 分粒度对齐 (例如 85.3 -> 85.5, 85.1 -> 85.0)
func CalculateFinalScore(in ScoreInput) float64 {
	// 1. 计算第一维度：情绪质量分 (S_emotion)
	sEmotion := calcEmotionScore(in.EndAnger, in.PeakAnger)

	// 如果情绪分为 0 (直接失败)，则无需计算效率分，直接返回 0
	if sEmotion == 0 {
		return 0.0
	}

	// 2. 计算第二维度：效率系数 (K_efficiency)
	kEfficiency := calcEfficiencyFactor(in.Turns)

	// 3. 综合计算
	rawScore := sEmotion * kEfficiency

	// 4. 格式化输出：按 0.5 粒度四舍五入
	return roundToHalf(rawScore)
}

// calcEmotionScore 计算情绪维度得分
func calcEmotionScore(endAnger, peakAnger int) float64 {
	// 场景 A: 失败结局 (怒气值爆表)
	if endAnger >= FailAngerThr {
		return 0.0
	}

	// 场景 B: 成功结局 (怒气值降至安全区)
	if endAnger <= SuccessAngerThr {
		// 基础分 100，扣除惊吓分
		// 逻辑：如果最高怒气曾飙升超过初始值(50)，则扣分
		// math.Max 确保如果 peakAnger < 50 (全程非常顺利)，不会算出一堆超过 100 的分
		penalty := math.Max(0, float64(peakAnger-StartAnger))
		score := 100.0 - penalty

		// 保底机制：既然成功了，最低给 60 分 (防止 penalty 过大导致低于及格线，视需求可调整)
		if score < 60.0 {
			score = 60.0
		}
		return score
	}

	// 场景 C: 僵持/超时结局 (10 < end < 90)
	// 基础分不及格，根据结束时的怒气值给辛苦分
	// 逻辑：40 + (50 - end)/2
	// 如果 end=50, 得 40 分; 如果 end=20, 得 55 分; 如果 end=80, 得 25 分
	return 40.0 + float64(StartAnger-endAnger)/2.0
}

// calcEfficiencyFactor 计算效率系数
func calcEfficiencyFactor(turns int) float64 {
	// 区间 1: 抢答区 (太快)
	if turns < MinTurnThr {
		// 线性回归：turns=4 -> 0.9, turns=8 -> 1.0
		return 0.8 + 0.025*float64(turns)
	}

	// 区间 2: 黄金区 (完美)
	if turns <= OptTurnThr {
		return 1.0
	}

	// 区间 3: 拖沓区 (指数衰减)
	// 超过 15 轮的部分，每轮衰减 5%
	overTurns := float64(turns - OptTurnThr)
	return math.Pow(DecayFactor, overTurns)
}

// roundToHalf 将分数舍入到最近的 0.5
func roundToHalf(val float64) float64 {
	return math.Round(val*2) / 2
}
