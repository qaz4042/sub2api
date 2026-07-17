# ChatGPT 5.6 生产分支合并说明

更新时间：2026-07-10

目标分支：`codex/before-fork-changed`

## 背景

本次只排查并合入 `main` 分支中 2026-07-10 当天与 ChatGPT/GPT-5.6 支持直接相关的改动，未合入当天的账单审计、i18n、compact SSE 等无关改动。

## 已合入内容

1. 升级 Codex/OpenAI OAuth 客户端版本到 `0.144.1`
   - 更新 `codexCLIUserAgent`
   - 更新 `codexCLIVersion`
   - 更新用量探测版本 `openAICodexProbeVersion`
   - 同步默认 Codex TUI User-Agent

2. 支持 GPT-5.6 `max` 推理强度
   - `gpt-5.6-sol`
   - `gpt-5.6-terra`
   - `gpt-5.6-luna`
   - 非 GPT-5.6 模型仍按原逻辑把 `max` 折叠为 `xhigh`
   - OpenAI OAuth `/responses/compact` 路径对 GPT-5.6 的 `max` 降级为 `xhigh`，保持 ChatGPT compact 端点兼容

3. 修复 usage metadata 的 reasoning effort 提取
   - effort 提取从单一模型改为模型候选列表
   - 解决 OAuth 上游模型被归一化后，原始模型后缀如 `gpt-5.3-codex-xhigh` 的 effort 丢失问题
   - WS / HTTP bridge / raw chat fallback 路径同步使用映射后模型和原始模型候选

4. 补齐生产分支缺失的 GPT-5.6 基础支持
   - 默认模型列表加入 `gpt-5.6-sol/terra/luna`
   - Codex OAuth 模型归一化保留 GPT-5.6 模型名
   - OpenAI 模型别名识别支持 GPT-5.6
   - GPT-5.6 暂按 GPT-5.4 定价兜底

## 提交

当前生产分支领先远端 4 个提交：

- `b128c633` `fix(openai): 升级 Codex 客户端版本至 0.144.1，修复 gpt-5.6-luna 404`
- `f04daccc` `fix: 兼容 GPT-5.6 max 推理强度`
- `42620c4d` `fix(usage): effort 提取改用模型候选列表，修复后缀模型元数据丢失`
- `0db90aa0` `fix(openai): 补齐 GPT-5.6 基础模型支持`

## 验证

已通过：

```bash
go test ./internal/service -run 'TestCodexVersionConstants_Consistency|TestNormalizeOpenAIReasoningEffortForGPT56|TestNormalizeOpenAICodexCompactReasoningEffort|TestOpenAIGatewayServiceForward.*(GPT56|OAuth|Mapped)|TestExtractOpenAIReasoningEffort|TestOpenAIGatewayServiceForwardOAuthDerivesEffortFromSuffixModel|TestForwardAsRawChatCompletionsPreservesGPT56MaxEffort' -count=1
go test ./internal/pkg/openai -count=1
```

同时执行过：

```bash
git diff --check origin/codex/before-fork-changed..HEAD
```

无空白错误。

## 部署注意

- 本次未部署、未推送。
- 部署前建议确认当前分支为 `codex/before-fork-changed`。
- 本次没有合入 `main` 中当天其他非 GPT-5.6 改动。
