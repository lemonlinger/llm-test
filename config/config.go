package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 定义整体配置结构
type Config struct {
	// 测试配置
	Test TestConfig `yaml:"test"`
	// 模型配置列表
	Models []ModelConfig `yaml:"models"`
	// 测试提示词
	Prompt PromptConfig `yaml:"prompt"`
	// 代理配置列表
	Proxies []ProxyConfig `yaml:"proxies"`
}

// TestConfig 定义测试相关配置
type TestConfig struct {
	// 并发数
	Concurrency int `yaml:"concurrency"`
	// 测试持续时间
	Duration time.Duration `yaml:"duration"`
	// 每个并发度的预热时间
	WarmupDuration time.Duration `yaml:"warmup_duration"`
	// 每个请求的超时时间
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// 递增的并发数列表，如果为空则只使用 Concurrency
	ConcurrencyLevels []int `yaml:"concurrency_levels"`
	// 是否显示进度条
	ShowProgress bool `yaml:"show_progress"`
	// 重试次数
	MaxRetries int `yaml:"max_retries"`
	// 需要计算的延迟百分位列表，例如 [50, 90, 95, 99]
	LatencyPercentiles []int `yaml:"latency_percentiles"`
}

// ModelConfig 定义模型相关配置
type ModelConfig struct {
	// 模型名称
	Name string `yaml:"name"`
	// 模型类型 (openai, anthropic, gemini等)
	Type string `yaml:"type"`
	// API密钥
	APIKey string `yaml:"api_key"`
	// API基础URL
	BaseURL string `yaml:"base_url"`
	// 模型参数
	Params map[string]interface{} `yaml:"params"`
	// 是否跳过该模型
	Skip bool `yaml:"skip"`
	// 模型特定的并发度设置，如果不为空则覆盖全局设置
	ConcurrencyLevels []int `yaml:"concurrency_levels,omitempty"`
	// 是否启用流式输出，如果未设置则使用全局prompt.stream
	Stream *bool `yaml:"stream,omitempty"`
	// 使用的代理名称，如果为空则不使用代理
	ProxyName string `yaml:"proxy_name,omitempty"`
}

// PromptConfig 定义提示词配置
type PromptConfig struct {
	// 系统消息
	SystemMessage string `yaml:"system_message"`
	// 用户消息
	UserMessage string `yaml:"user_message"`
	// 是否启用流式输出
	Stream bool `yaml:"stream"`
}

// ProxyConfig 定义代理配置
type ProxyConfig struct {
	// 代理名称
	Name string `yaml:"name"`
	// 代理URL
	URL string `yaml:"url"`
}

// LoadConfig 从文件中加载配置
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if config.Test.Concurrency == 0 {
		config.Test.Concurrency = 1
	}
	if config.Test.Duration == 0 {
		config.Test.Duration = 30 * time.Second
	}
	if config.Test.RequestTimeout == 0 {
		config.Test.RequestTimeout = 30 * time.Second
	}
	if config.Test.MaxRetries == 0 {
		config.Test.MaxRetries = 3
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateConfig 验证配置是否合法
func validateConfig(config *Config) error {
	if len(config.Models) == 0 {
		return fmt.Errorf("至少需要配置一个模型")
	}

	if config.Prompt.UserMessage == "" {
		return fmt.Errorf("用户提示词不能为空")
	}

	for i, model := range config.Models {
		if model.Name == "" {
			return fmt.Errorf("模型 #%d 未指定名称", i+1)
		}
		if model.Type == "" {
			return fmt.Errorf("模型 %s 未指定类型", model.Name)
		}
		if model.APIKey == "" {
			return fmt.Errorf("模型 %s 未指定API密钥", model.Name)
		}
	}

	return nil
}
