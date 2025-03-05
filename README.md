# LLM-Test: 大语言模型API性能测试工具

[![Go版本](https://img.shields.io/badge/Go-1.18+-blue.svg)](https://golang.org/doc/devel/release.html)
[![许可证](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)

LLM-Test是一个用于测试大语言模型API性能的工具，支持多种模型、可配置的并发度和详细的性能报告。

> **声明**：本代码仓库中99%以上的代码是由Cursor（基于Claude 3.7 Sonnet）生成。

## 功能特点

- 支持多种LLM模型（OpenAI、Anthropic、Gemini等）
- 可配置的并发度测试，支持模型特定的并发度设置
- 详细的性能指标（延迟、吞吐量、成功率、Token处理速度等）
- 延迟百分位数统计（P50、P90、P99等）
- 支持多种输出格式（文本、CSV、JSON）
- 代理支持，可为不同模型配置不同代理
- 流式输出支持
- 可配置的测试参数（持续时间、预热时间、超时等）

## 安装

### 前提条件

- Go 1.18或更高版本

### 从源码安装

```bash
git clone https://github.com/lemonlinger/llm-test.git
cd llm-test
go build
```

## 快速开始

1. 创建配置文件（参考`config.yaml.example`示例）
2. 运行测试

```bash
./llm-test -c config.yaml
```

## 配置文件

`config.yaml`文件包含所有测试配置，包括测试参数、模型配置和提示词设置。

### 基本配置

```yaml
# 测试配置
test:
  # 基础并发数
  concurrency: 10
  # 测试持续时间 (单位：秒)
  duration: 30s
  # 每个并发级别的预热时间 (单位：秒)
  warmup_duration: 0s
  # 每个请求的超时时间 (单位：秒)
  request_timeout: 120s
  # 递增的并发数列表，如果设置了此项，将按照此列表依次测试不同并发度
  concurrency_levels: [10, 20, 50, 100]
  # 是否显示进度条
  show_progress: true
  # 请求失败重试次数
  max_retries: 3
  # 延迟百分位计算列表
  latency_percentiles: [50, 90, 95, 99]

# 模型配置列表
models:
  - name: model-name
    type: openai
    api_key: your-api-key
    base_url: https://api.example.com/v1
    params:
      model: model-id
      temperature: 0.7
      max_tokens: 3000
    # 模型特定的并发度配置（覆盖全局配置）
    concurrency_levels: [5, 10, 20]
    # 使用代理
    proxy_name: "proxy-name"

# 代理配置
proxies:
  - name: "proxy-name"
    url: "http://proxy.example.com:8080"

# 提示词配置
prompt:
  # 系统消息
  system_message: "你是一个助手，请提供简洁明了的回答。"
  # 用户消息
  user_message: "请解释量子计算的基本原理。"
  # 是否启用流式输出
  stream: false
```

### 模型特定配置

每个模型可以有自己的配置，包括API密钥、基础URL、模型参数和并发度设置。

```yaml
models:
  - name: model-a
    type: openai
    api_key: key-a
    concurrency_levels: [5, 10, 20]
    proxy_name: "proxy-a"
  
  - name: model-b
    type: anthropic
    api_key: key-b
    concurrency_levels: [10, 20, 50]
    proxy_name: "proxy-b"
```

## 输出报告

测试完成后，工具会生成详细的性能报告，包括：

- 每个模型在不同并发度下的性能数据
- 平均延迟和延迟百分位数据
- 请求成功率
- 每秒请求数(RPS)和每秒Token数(TPS)
- Token使用统计

### 示例报告

```
# LLM API 性能测试报告摘要

| 模型 | 并发度 | 成功/总请求 | 成功率 | 平均延迟 | 平均输入Token | 平均输出Token | 平均总Token | RPS | TPS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| model-a | 10 | 31/31 | 100.00% | 43.29 s | 729.00 | 1115.06 | 1844.06 | 0.18 | 326.33 |
| model-a | 20 | 62/62 | 100.00% | 44.80 s | 729.00 | 1178.71 | 1907.71 | 0.38 | 723.54 |
| model-b | 10 | 30/32 | 93.75% | 38.96 s | 727.00 | 941.00 | 1668.00 | 0.21 | 351.37 |
| model-b | 20 | 54/62 | 87.10% | 41.30 s | 727.00 | 897.35 | 1624.35 | 0.35 | 564.73 |
```

## 命令行选项

```
用法: llm-test [选项]

选项:
  -c, --config string   配置文件路径 (默认 "config.yaml")
  -f, --format string   报告格式: text, csv, json (默认 "text")
  -o, --output string   输出文件路径 (默认输出到控制台)
  -h, --help            显示帮助信息
```

## 贡献

欢迎贡献代码、报告问题或提出改进建议。请遵循以下步骤：

1. Fork项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## 许可证

本项目采用MIT许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。 