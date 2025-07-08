# 使用 Zenfeed 将文章转换为播客

Zenfeed 的播客功能可以将任何文章源自动转换为一场引人入胜的多人对话式播客。该功能利用大语言模型（LLM）生成对话脚本和文本转语音（TTS），并将最终的音频文件托管在您自己的对象存储中。

## 工作原理

1.  **提取内容**: Zenfeed 首先通过重写规则提取文章的全文内容。
2.  **生成脚本**: 使用一个指定的 LLM（如 GPT-4o-mini）将文章内容改编成一个由多位虚拟主播对话的脚本。您可以定义每个主播的角色（人设）来控制对话风格。
3.  **语音合成**: 调用另一个支持 TTS 的 LLM（目前仅支持 Google Gemini）将脚本中的每一句对话转换为语音。
4.  **音频合并**: 将所有语音片段合成为一个完整的 WAV 音频文件。
5.  **上传存储**: 将生成的播客文件上传到您配置的 S3 兼容对象存储中。
6.  **保存链接**: 最后，将播客文件的公开访问 URL 保存为一个新的 Feed 标签，方便您在通知、API 或其他地方使用。

## 配置步骤

要启用播客功能，您需要完成以下三项配置：LLM、对象存储和重写规则。

### 1. 配置 LLM

您需要至少配置两个 LLM：一个用于生成对话脚本，另一个用于文本转语音（TTS）。

-   **脚本生成 LLM**: 可以是任何性能较好的聊天模型，例如 OpenAI 的 `gpt-4o-mini` 或 Google 的 `gemini-1.5-pro`。
-   **TTS LLM**: 用于将文本转换为语音。**注意：目前此功能仅支持 `provider` 为 `gemini` 的 LLM。**

**示例 `config.yaml`:**

```yaml
llms:
  # 用于生成播客脚本的 LLM
  - name: openai-chat
    provider: openai
    api_key: "sk-..."
    model: gpt-4o-mini
    default: true

  # 用于文本转语音 (TTS) 的 LLM
  - name: gemini-tts
    provider: gemini
    api_key: "..." # 你的 Google AI Studio API Key
    tts_model: "gemini-2.5-flash-preview-tts" # Gemini 的 TTS 模型
```

### 2. 配置对象存储

生成的播客音频文件需要一个地方存放。Zenfeed 支持任何 S3 兼容的对象存储服务。这里我们以 [Cloudflare R2](https://www.cloudflare.com/zh-cn/products/r2/) 为例。

首先，您需要在 Cloudflare R2 中创建一个存储桶（Bucket）。然后获取以下信息：

-   `endpoint`: 您的 R2 API 端点。通常格式为 `https://<account_id>.r2.cloudflarestorage.com`。您可以在 R2 存储桶的主页找到它。
-   `access_key_id` 和 `secret_access_key`: R2 API 令牌。您可以在 "R2" -> "管理 R2 API 令牌" 页面创建。
-   `bucket`: 您创建的存储桶的名称。
-   `bucket_url`: 存储桶的公开访问 URL。要获取此 URL，您需要将存储桶连接到一个自定义域，或者使用 R2 提供的 `r2.dev` 公开访问地址。

**示例 `config.yaml`:**

```yaml
storage:
  object:
    endpoint: "https://<your_account_id>.r2.cloudflarestorage.com"
    access_key_id: "..."
    secret_access_key: "..."
    bucket: "zenfeed-podcasts"
    bucket_url: "https://pub-xxxxxxxx.r2.dev"
```

### 3. 配置重写规则

最后一步是创建一个重写规则，告诉 Zenfeed 如何将文章转换为播客。这个规则定义了使用哪个标签作为源文本、由谁来对话、使用什么声音等。

**关键配置项:**

-   `source_label`: 包含文章全文的标签。
-   `label`: 用于存储最终播客 URL 的新标签名称。
-   `transform.to_podcast`: 播客转换的核心配置。
    -   `llm`: 用于生成脚本的 LLM 名称（来自 `llms` 配置）。
    -   `tts_llm`: 用于 TTS 的 LLM 名称（来自 `llms` 配置）。
    -   `speakers`: 定义播客的演讲者。
        -   `name`: 演讲者的名字。
        -   `role`: 演讲者的角色和人设，将影响脚本内容。
        -   `voice`: 演讲者的声音。请参考 [Google Cloud TTS 文档](https://cloud.google.com/text-to-speech/docs/voices) 获取可用的声音名称（例如 `en-US-Standard-C`，`en-US-News-N`）。

**示例 `config.yaml`:**

```yaml
storage:
  feed:
    rewrites:
      - source_label: "content"
        label: "podcast_url"
        transform:
          to_podcast:
            llm: "openai-chat"
            tts_llm: "gemini-tts"
            transcript_additional_prompt: "请让对话更生动有趣一些。使用中文回复"
            speakers:
              - name: "主持人小雅"
                role: "一位经验丰富、声音甜美、风格活泼的科技播客主持人。"
                voice: "zh-CN-Standard-A" # 女声
              - name: "技术评论员老王"
                role: "一位对技术有深入见解、观点犀利的评论员，说话直接，偶尔有些愤世嫉俗。"
                voice: "zh-CN-Standard-B" # 男声
```

配置完成后，Zenfeed 将在每次抓取到新文章时，自动执行上述流程。可以在通知模版中使用 podcast_url label，或 Web 中直接收听（Web 固定读取 podcast_url label，若使用别的名称则无法读取）