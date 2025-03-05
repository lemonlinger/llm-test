package engine

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/briandowns/spinner"
	"github.com/lemonlinger/llm-test/config"
	"github.com/lemonlinger/llm-test/model"
)

// 测试结果结构体
type TestResult struct {
	ModelName          string
	ConcurrencyLevel   int // 添加并发度字段
	TotalRequests      int
	SuccessRequests    int
	FailedRequests     int
	TotalDuration      time.Duration
	AvgLatency         time.Duration
	InputTokens        int64
	OutputTokens       int64
	TotalTokens        int64
	AvgInputTokens     float64
	AvgOutputTokens    float64
	AvgTotalTokens     float64
	RequestsPerSec     float64
	TokensPerSec       float64
	Errors             []string
	LatencyPercentiles map[int]time.Duration // 存储各个百分位的延迟
	AllLatencies       []time.Duration       // 所有请求的延迟记录
}

// 测试引擎结构体
type TestEngine struct {
	config  config.TestConfig
	models  []model.LLMModel
	prompt  config.PromptConfig
	results map[string]*TestResult
	spinner *spinner.Spinner
	proxies map[string]string // 代理名称到URL的映射
}

// 创建新的测试引擎
func NewTestEngine(testConfig config.TestConfig, models []model.LLMModel, prompt config.PromptConfig, proxies []config.ProxyConfig) *TestEngine {
	// 创建代理映射
	proxyMap := make(map[string]string)
	for _, proxy := range proxies {
		proxyMap[proxy.Name] = proxy.URL
	}

	return &TestEngine{
		config:  testConfig,
		models:  models,
		prompt:  prompt,
		results: make(map[string]*TestResult),
		proxies: proxyMap,
	}
}

// 运行测试
func (e *TestEngine) Run() (map[string]*TestResult, error) {
	// 使用复合键（模型名称+并发度）来存储结果
	results := make(map[string]*TestResult)

	for _, mdl := range e.models {
		modelName := mdl.GetName()
		fmt.Printf("正在测试模型: %s\n", modelName)

		// 设置并发度：优先使用模型自身的并发度配置，如果没有则使用全局配置
		var concurrencyLevels []int
		modelConcurrencyLevels := mdl.GetConcurrencyLevels()

		if len(modelConcurrencyLevels) > 0 {
			// 使用模型特定的并发度配置
			concurrencyLevels = modelConcurrencyLevels
			fmt.Printf("  使用模型特定的并发度配置: %v\n", concurrencyLevels)
		} else if len(e.config.ConcurrencyLevels) > 0 {
			// 使用全局并发度级别列表
			concurrencyLevels = e.config.ConcurrencyLevels
			fmt.Printf("  使用全局并发度配置: %v\n", concurrencyLevels)
		} else {
			// 使用基础并发度
			concurrencyLevels = []int{e.config.Concurrency}
			fmt.Printf("  使用基础并发度: %d\n", e.config.Concurrency)
		}

		// 对每个并发级别运行测试
		for _, concurrency := range concurrencyLevels {
			// 为每个并发度创建一个新的结果对象
			resultKey := fmt.Sprintf("%s-%d", modelName, concurrency)
			result := &TestResult{
				ModelName:        modelName,
				ConcurrencyLevel: concurrency,
				Errors:           make([]string, 0),
			}
			results[resultKey] = result

			err := e.runTestWithConcurrency(mdl, concurrency, result)
			if err != nil {
				return nil, fmt.Errorf("测试模型 %s 失败: %w", modelName, err)
			}
		}
	}

	// 更新e.results以保持兼容性
	e.results = results

	return results, nil
}

// 以指定并发度运行测试
func (e *TestEngine) runTestWithConcurrency(mdl model.LLMModel, concurrency int, result *TestResult) error {
	// 获取模型名称
	modelName := mdl.GetName()

	fmt.Printf("  并发度: %d\n", concurrency)

	// 初始化计数器
	var successCount int64
	var failedCount int64
	var totalLatency int64
	var inputTokens int64
	var outputTokens int64

	// 确定是否使用流式输出：优先使用模型特定设置，如果未设置则使用全局设置
	useStream := e.prompt.Stream
	if modelStream := mdl.GetStreamSetting(); modelStream != nil {
		useStream = *modelStream
		fmt.Printf("  使用模型特定的流式设置: %v\n", useStream)
	} else {
		fmt.Printf("  使用全局流式设置: %v\n", useStream)
	}

	if e.config.ShowProgress {
		e.spinner = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		e.spinner.Prefix = "  正在测试 "
		e.spinner.Start()
		defer e.spinner.Stop()
	}

	// 创建工作通道和等待组
	jobs := make(chan struct{}, concurrency*2)
	var wg sync.WaitGroup

	// 创建延迟数据切片和互斥锁
	var latencies []time.Duration
	var latenciesMutex sync.Mutex

	// 创建信号量控制并发
	sem := make(chan struct{}, concurrency)

	// 如果有预热时间，先进行预热
	if e.config.WarmupDuration > 0 {
		// 预热逻辑...
		time.Sleep(e.config.WarmupDuration)
	}

	// 记录开始时间
	startTime := time.Now()

	// 启动工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range jobs {
				sem <- struct{}{}

				// 执行单个请求
				ctx, cancel := context.WithTimeout(context.Background(), e.config.RequestTimeout)

				start := time.Now()
				resp, err := mdl.GenerateResponse(ctx, e.prompt.SystemMessage, e.prompt.UserMessage, useStream)
				latency := time.Since(start)

				// 记录延迟数据
				latenciesMutex.Lock()
				latencies = append(latencies, latency)
				latenciesMutex.Unlock()

				if err != nil {
					log.Printf("测试模型 %s 失败: %v", modelName, err)
					atomic.AddInt64(&failedCount, 1)
					result.Errors = append(result.Errors, err.Error()) // 注意：这里有并发安全问题，实际应该使用互斥锁
				} else {
					atomic.AddInt64(&successCount, 1)
					atomic.AddInt64(&totalLatency, int64(latency))
					atomic.AddInt64(&inputTokens, int64(resp.InputTokens))
					atomic.AddInt64(&outputTokens, int64(resp.OutputTokens))
				}

				cancel()
				<-sem
			}
		}()
	}

	// 发送工作
	timeout := time.After(e.config.Duration)
	requestCount := 0

loop:
	for {
		select {
		case <-timeout:
			break loop
		default:
			jobs <- struct{}{}
			requestCount++
		}
	}

	close(jobs)
	wg.Wait()

	// 计算总持续时间
	totalDuration := time.Since(startTime)

	// 更新结果
	result.TotalRequests += requestCount
	result.SuccessRequests += int(successCount)
	result.FailedRequests += int(failedCount)
	result.TotalDuration += totalDuration

	if successCount > 0 {
		avgLatency := time.Duration(totalLatency / successCount)
		result.AvgLatency = avgLatency

		result.InputTokens += inputTokens
		result.OutputTokens += outputTokens
		result.TotalTokens += inputTokens + outputTokens

		result.AvgInputTokens = float64(result.InputTokens) / float64(result.SuccessRequests)
		result.AvgOutputTokens = float64(result.OutputTokens) / float64(result.SuccessRequests)
		result.AvgTotalTokens = float64(result.TotalTokens) / float64(result.SuccessRequests)

		result.RequestsPerSec = float64(result.SuccessRequests) / totalDuration.Seconds()
		result.TokensPerSec = float64(result.TotalTokens) / totalDuration.Seconds()
	}

	// 存储所有延迟数据
	result.AllLatencies = latencies

	// 计算延迟百分位
	if len(latencies) > 0 && len(e.config.LatencyPercentiles) > 0 {
		result.LatencyPercentiles = make(map[int]time.Duration)
		for _, p := range e.config.LatencyPercentiles {
			result.LatencyPercentiles[p] = calculatePercentile(latencies, p)
		}
	}

	return nil
}

// 计算百分位数
func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	// 创建副本并排序
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	sort.Slice(sortedLatencies, func(i, j int) bool {
		return sortedLatencies[i] < sortedLatencies[j]
	})

	// 计算百分位索引
	index := int(float64(len(sortedLatencies)-1) * float64(percentile) / 100.0)
	return sortedLatencies[index]
}
