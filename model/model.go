package model

import (
	"context"
	"fmt"
	"time"

	"github.com/lemonlinger/llm-test/config"
)

// contextKey 用于上下文传值
type contextKey string

// 上下文键定义
const (
	ProxyURLContextKey contextKey = "proxy_url"
)

// LLMResponse 定义模型响应结构
type LLMResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
	// 流式响应专用指标
	TimeToFirstToken time.Duration // 首个token的响应时间
	TokensPerSecond  float64       // 流式响应的token生成速率
}

// LLMModel 定义大语言模型接口
type LLMModel interface {
	// 获取模型名称
	GetName() string
	// 生成响应
	GenerateResponse(ctx context.Context, systemMessage, userMessage string, stream bool) (*LLMResponse, error)
	// 获取模型特定的并发度配置
	GetConcurrencyLevels() []int
	// 获取模型特定的流式输出设置
	GetStreamSetting() *bool
	// 获取模型使用的代理名称
	GetProxyName() string
}

// 初始化所有配置的模型
func InitializeModels(modelConfigs []config.ModelConfig, proxies []config.ProxyConfig) ([]LLMModel, error) {
	models := make([]LLMModel, 0, len(modelConfigs))

	for _, cfg := range modelConfigs {
		if cfg.Skip {
			continue
		}

		var model LLMModel
		var err error

		switch cfg.Type {
		case "openai":
			model, err = NewOpenAIModel(cfg, proxies)
		case "anthropic":
			model, err = NewAnthropicModel(cfg, proxies)
		case "gemini":
			model, err = NewGeminiModel(cfg, proxies)
		default:
			return nil, fmt.Errorf("不支持的模型类型: %s", cfg.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("初始化模型 %s 失败: %w", cfg.Name, err)
		}

		models = append(models, model)
	}

	return models, nil
}

// BaseModel 提供基本的模型实现
type BaseModel struct {
	config config.ModelConfig
}

// GetName 返回模型名称
func (m *BaseModel) GetName() string {
	return m.config.Name
}

// GetConcurrencyLevels 返回模型特定的并发度配置
func (m *BaseModel) GetConcurrencyLevels() []int {
	return m.config.ConcurrencyLevels
}

// GetStreamSetting 返回模型特定的流式输出设置
func (m *BaseModel) GetStreamSetting() *bool {
	return m.config.Stream
}

// GetProxyName 返回模型使用的代理名称
func (m *BaseModel) GetProxyName() string {
	return m.config.ProxyName
}
