package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lemonlinger/llm-test/engine"
)

// ModelPerformance 存储模型在不同并发度下的性能数据
type ModelPerformance struct {
	ModelName        string
	ConcurrencyLevel int
	RequestsPerSec   float64
	TokensPerSec     float64
	AvgLatency       time.Duration
	SuccessRate      float64
	AvgInputTokens   float64
	AvgOutputTokens  float64
	AvgTotalTokens   float64
}

// Reporter 报告生成器结构体
type Reporter struct {
	format string
}

// NewReporter 创建新的报告生成器
func NewReporter(format string) *Reporter {
	return &Reporter{
		format: format,
	}
}

// GenerateReport 生成测试报告
func (r *Reporter) GenerateReport(results map[string]*engine.TestResult) (string, error) {
	switch r.format {
	case "json":
		return r.generateJSONReport(results)
	case "csv":
		return r.generateCSVReport(results)
	default:
		return r.generateTextReport(results)
	}
}

// 生成文本格式报告
func (r *Reporter) generateTextReport(results map[string]*engine.TestResult) (string, error) {
	var sb strings.Builder

	// 收集所有测试结果
	allResults := make([]*engine.TestResult, 0, len(results))
	for _, result := range results {
		allResults = append(allResults, result)
	}

	// 获取所有使用的百分位
	allPercentiles := getAllPercentiles(allResults)

	// 报告摘要
	sb.WriteString("# LLM API 性能测试报告\n\n")

	// 报告设置
	sb.WriteString("## 测试设置\n\n")
	sb.WriteString("\n")

	// 详细结果
	sb.WriteString("## 测试结果\n\n")

	// 生成单个合并表格（标准Markdown格式）
	// 表头
	sb.WriteString("| 模型 | 并发度 | 成功/总请求 | 成功率 | 平均延迟 | 平均输入Token | 平均输出Token | 平均总Token | RPS | TPS")

	// 添加百分位列
	for _, p := range allPercentiles {
		sb.WriteString(fmt.Sprintf(" | P%d", p))
	}
	sb.WriteString(" |\n")

	// 分隔线
	sb.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | ---")
	for range allPercentiles {
		sb.WriteString(" | ---")
	}
	sb.WriteString(" |\n")

	// 按模型名称和并发度排序
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].ModelName != allResults[j].ModelName {
			return allResults[i].ModelName < allResults[j].ModelName
		}
		return allResults[i].ConcurrencyLevel < allResults[j].ConcurrencyLevel
	})

	// 内容
	for _, result := range allResults {
		// 计算成功率
		successRate := 0.0
		if result.TotalRequests > 0 {
			successRate = float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
		}

		sb.WriteString(fmt.Sprintf("| %s | %d | %d/%d | %.2f%% | %s | %.2f | %.2f | %.2f | %.2f | %.2f",
			result.ModelName,
			result.ConcurrencyLevel,
			result.SuccessRequests, result.TotalRequests,
			successRate,
			formatDuration(result.AvgLatency),
			result.AvgInputTokens,
			result.AvgOutputTokens,
			result.AvgTotalTokens,
			result.RequestsPerSec,
			result.TokensPerSec))

		// 添加百分位数据
		for _, p := range allPercentiles {
			if latency, ok := result.LatencyPercentiles[p]; ok {
				sb.WriteString(fmt.Sprintf(" | %s", formatDuration(latency)))
			} else {
				sb.WriteString(" | -")
			}
		}

		sb.WriteString(" |\n")
	}

	sb.WriteString("\n")

	return sb.String(), nil
}

// 获取所有结果中使用的百分位值，并按升序排序
func getAllPercentiles(results []*engine.TestResult) []int {
	// 使用map去重
	percentileMap := make(map[int]struct{})

	for _, result := range results {
		if result.LatencyPercentiles != nil {
			for p := range result.LatencyPercentiles {
				percentileMap[p] = struct{}{}
			}
		}
	}

	// 转换为切片并排序
	percentiles := make([]int, 0, len(percentileMap))
	for p := range percentileMap {
		percentiles = append(percentiles, p)
	}

	sort.Ints(percentiles)
	return percentiles
}

// 生成CSV格式报告
func (r *Reporter) generateCSVReport(results map[string]*engine.TestResult) (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// 收集所有测试结果
	allResults := make([]*engine.TestResult, 0, len(results))
	for _, result := range results {
		allResults = append(allResults, result)
	}

	// 获取所有使用的百分位
	allPercentiles := getAllPercentiles(allResults)

	// 按模型名称和并发度排序
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].ModelName != allResults[j].ModelName {
			return allResults[i].ModelName < allResults[j].ModelName
		}
		return allResults[i].ConcurrencyLevel < allResults[j].ConcurrencyLevel
	})

	// 写入表头
	headers := []string{
		"模型名称", "并发度", "平均延迟(ms)",
		"平均输入Token", "平均输出Token", "平均总Token",
		"每秒请求数(RPS)", "每秒Token数(TPS)", "成功率(%)",
		"总请求数", "成功请求数", "失败请求数",
	}

	// 添加百分位表头
	for _, p := range allPercentiles {
		headers = append(headers, fmt.Sprintf("P%d(ms)", p))
	}

	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("写入CSV表头失败: %w", err)
	}

	// 写入所有结果数据
	for _, result := range allResults {
		successRate := 0.0
		if result.TotalRequests > 0 {
			successRate = float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
		}

		row := []string{
			result.ModelName,
			fmt.Sprintf("%d", result.ConcurrencyLevel),
			fmt.Sprintf("%d", result.AvgLatency.Milliseconds()),
			fmt.Sprintf("%.2f", result.AvgInputTokens),
			fmt.Sprintf("%.2f", result.AvgOutputTokens),
			fmt.Sprintf("%.2f", result.AvgTotalTokens),
			fmt.Sprintf("%.2f", result.RequestsPerSec),
			fmt.Sprintf("%.2f", result.TokensPerSec),
			fmt.Sprintf("%.2f", successRate),
			fmt.Sprintf("%d", result.TotalRequests),
			fmt.Sprintf("%d", result.SuccessRequests),
			fmt.Sprintf("%d", result.FailedRequests),
		}

		// 添加百分位数据
		for _, p := range allPercentiles {
			if latency, ok := result.LatencyPercentiles[p]; ok {
				row = append(row, fmt.Sprintf("%d", latency.Milliseconds()))
			} else {
				row = append(row, "-")
			}
		}

		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("写入CSV数据失败: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("刷新CSV写入器失败: %w", err)
	}

	return sb.String(), nil
}

// 生成JSON格式报告
func (r *Reporter) generateJSONReport(results map[string]*engine.TestResult) (string, error) {
	// 创建一个结构化的JSON报告
	type LatencyPercentile struct {
		Percentile int   `json:"percentile"`
		LatencyMs  int64 `json:"latency_ms"`
	}

	type ResultRecord struct {
		ModelName        string              `json:"model_name"`
		ConcurrencyLevel int                 `json:"concurrency"`
		AvgLatencyMs     int64               `json:"avg_latency_ms"`
		AvgInputTokens   float64             `json:"avg_input_tokens"`
		AvgOutputTokens  float64             `json:"avg_output_tokens"`
		AvgTotalTokens   float64             `json:"avg_total_tokens"`
		RequestsPerSec   float64             `json:"requests_per_sec"`
		TokensPerSec     float64             `json:"tokens_per_sec"`
		SuccessRate      float64             `json:"success_rate"`
		TotalRequests    int                 `json:"total_requests"`
		SuccessRequests  int                 `json:"success_requests"`
		FailedRequests   int                 `json:"failed_requests"`
		Percentiles      []LatencyPercentile `json:"percentiles,omitempty"`
	}

	type Report struct {
		TestResults []*ResultRecord `json:"test_results"`
	}

	// 填充报告
	report := Report{
		TestResults: make([]*ResultRecord, 0),
	}

	// 收集所有测试结果
	allResults := make([]*engine.TestResult, 0, len(results))
	for _, result := range results {
		allResults = append(allResults, result)
	}

	// 按模型名称和并发度排序
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].ModelName != allResults[j].ModelName {
			return allResults[i].ModelName < allResults[j].ModelName
		}
		return allResults[i].ConcurrencyLevel < allResults[j].ConcurrencyLevel
	})

	// 添加所有测试结果
	for _, result := range allResults {
		// 计算成功率，防止除以零
		successRate := 0.0
		if result.TotalRequests > 0 {
			successRate = float64(result.SuccessRequests) / float64(result.TotalRequests)
		}

		// 创建百分位数据
		percentiles := make([]LatencyPercentile, 0)
		if result.LatencyPercentiles != nil {
			for p, latency := range result.LatencyPercentiles {
				percentiles = append(percentiles, LatencyPercentile{
					Percentile: p,
					LatencyMs:  latency.Milliseconds(),
				})
			}

			// 按百分位排序
			sort.Slice(percentiles, func(i, j int) bool {
				return percentiles[i].Percentile < percentiles[j].Percentile
			})
		}

		resultRecord := &ResultRecord{
			ModelName:        result.ModelName,
			ConcurrencyLevel: result.ConcurrencyLevel,
			AvgLatencyMs:     result.AvgLatency.Milliseconds(),
			AvgInputTokens:   result.AvgInputTokens,
			AvgOutputTokens:  result.AvgOutputTokens,
			AvgTotalTokens:   result.AvgTotalTokens,
			RequestsPerSec:   result.RequestsPerSec,
			TokensPerSec:     result.TokensPerSec,
			SuccessRate:      successRate,
			TotalRequests:    result.TotalRequests,
			SuccessRequests:  result.SuccessRequests,
			FailedRequests:   result.FailedRequests,
			Percentiles:      percentiles,
		}

		report.TestResults = append(report.TestResults, resultRecord)
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	return string(jsonData), nil
}

// 格式化持续时间
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Microseconds()))
	} else if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Milliseconds()))
	} else {
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}
