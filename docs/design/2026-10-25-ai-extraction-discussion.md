# AI 提取与多后端接入：讨论备忘

- 日期：2026-10-25
- 状态：方向讨论，尚未形成实施设计；本次仅授权记录，不授权实现或发布。
- 依据：当前源码、主设计 §1.7 / §4.4 / §8，以及下列官方 API 文档。

## 1. 用户明确的方向与优先级

先考虑 OpenAI 接入，希望用户连接自己的账号使用模型。代码不能绑定 OpenAI：未来可能支持 DeepSeek、阿里云百炼、AWS Bedrock，用户主要使用的后端也可能改变。第二个后端尚未选定。

优先探索图片提取，分为三个独立需求：

| 能力 | 目标 |
|---|---|
| OCR | 尽量完整、忠实地提取图片中的文字，不以摘要代替全文 |
| 图片理解 | 输出内容描述 `desc`；也考虑向 PDF 等文件扩展 |
| 结构化提取 | 理解内容后提取 KV，例如证件上的姓名、号码、生效日期 |

自然语言检索也值得做：将用户意图转换为 kist 可执行的搜索条件，并通过后续交互调整条件。它不依赖供应商的托管文件搜索。

辅助整理归档盒有价值，尤其适用于大量手机照片；目前没有手机客户端，用户希望往后放。选中文档问答、事件回顾、材料汇总、证件到期提醒等优先级较低。供应商未来可能开放更多能力，但没有确定承诺或时间表，不据此安排交付。

## 2. 当前代码提供的基础和限制

- `internal/extract/extract.go` 定义提取器身份、版本、命名空间和结果校验。
- `internal/library/extract.go` 在事务外运行提取器，通过 library 把结果写入操作日志。模型可以不确定，但重建必须重放已保存结果，不能重新请求模型。
- `internal/query/query.go` 当前主要表达名称、标签、盒、日期与类型等条件。将自然语言转成这些条件，不会自动获得 OCR 全文或图片语义检索。
- OCR 全文索引会触发主设计 §4.4 / §8 的检索逃生舱；字段比较涉及 open-questions B8。自定义 metadata 编辑及提醒仍需设计。

### 1024 字节限制的来源与重开问题

当前 `extract.MaxValueLen = 1024` 是**提取器输出校验限制**，不是 SQLite `file_metadata.value` 列的 1024 字节约束。Go 的 `len(string)` 按 UTF-8 字节计算。通用校验对超限结果报错，EXIF 的 `textOrSkip` 跳过超限值，不截断保存。

来源是 M3 的 EXIF 提取：实现计划认为“真正的标量标签没有这么长的，超过它的一定是被当成字符串的二进制块”。见 [M3 记录](../plans/2026-08-28-m3-extraction.md)。限制有约束异常输出、减少日志膨胀和资源消耗的作用，但未找到 1024 来自性能测量的依据。

这一判断不能推广到 OCR、长描述或注释；而短二进制也可能通过长度检查，EXIF 后来已增加其他过滤。主设计 §1.7 原本就列出 `ocr:text`、`ai:caption`，因此不能为保留此常数而限制新能力。

尚未决定如何调整。讨论过两条路线：

| 候选 | 收益 | 代价 |
|---|---|---|
| 文本继续作为 metadata 值，调整校验与读取方式 | 延续既有日志、撤销和重建模型 | 大文本增加日志与批处理成本；详情接口不能总是加载全文 |
| 长文本保存为 blob，日志记录引用及来源 | 独立读取大内容，复用内容引用 | 增加引用、同步、完整性检查及历史恢复复杂度 |

助手最初倾向 blob，追查后认为这一建议过早，应先用真实样本评估简单文本方案。**两者均未获选定**；不能将“无限长度”或“长文本必须存 blob”写成既定要求。任务总输出、单字段资源边界、页界及来源定位需要一起讨论。

## 3. 多后端设计建议（尚未定稿）

用户已明确要求不写死 OpenAI。以下是实现前需要细化的建议：

1. 提取任务（OCR、描述、模板字段）与后端适配（认证、模型协议、输入转换）分离，最终结果仍经 library 保存。
2. 持久化字段表达业务语义，不使用厂商名作为字段名。后端、模型、提取规则版本和输入内容标识单独记录，切换默认后端不改历史结果、不自动重跑全库。
3. 逻辑提取器拥有命名空间，实际后端由配置选择；避免多个厂商提取器争抢同一个 `ocr:` 命名空间。
4. 能力具体到模型和 API 路径，区分原生输入、平台代解析、kist 本地转换。不能用一个“OpenAI 兼容”布尔值假设全部功能一致。
5. OAuth、API key 等凭据属于连接配置，不进入归档日志或普通库同步。NAS 登录、凭据保存与刷新需专门设计。
6. 未来可按任务选后端，例如本地 OCR、云端描述；首版不必先构建复杂路由系统。

仍需讨论：固定字段模板还是自由 KV、字段类型与日期精度、原文证据与页码、用户纠正如何保护、重提取版本与配置变化的关系、手动或自动提取、批量进度与重试、发送范围及额度控制。助手建议先手动小批量、以模板提取为主，但用户尚未逐项确认。

搜索建议以现有可编辑条件作为最终结果，模型输出结构化条件，由后端校验并执行，不生成 SQL 或 shell 命令。多轮交互可以查找候选盒与标签，再修正条件。OCR/描述内容搜索是独立扩展；不得把“找红色汽车照片”静默替换为“文件名含红色汽车”。

## 4. 供应商能力调研快照

以下均为 **2026-10-25 官方文档核查，未实测**。模型、价格、区域和接口限制会变化，实施时重新核实。“平台支持”不表示所有模型或同一个聊天接口都支持。

### OpenAI

讨论首先面向 **Sign in with ChatGPT 的套餐授权路径**，不是泛指全部 OpenAI API。开源应用文档描述了授权符合条件的 Responses 请求使用用户套餐；不会因此获得用户的 ChatGPT 历史对话。

该路径预览文档支持模型允许的文本、图片和文件输入，但不支持音视频输入、Files 上传 API、转录 API、托管 file search 等。不能据此断言普通 API key 路径也不支持这些能力。NAS/远端主机的官方流程涉及本地 OAuth 后安全转移凭据，不能假设加一个网页按钮即可完成。账号资格和实际流程待验证。

- [套餐授权概览](https://developers.openai.com/siwc/token-sharing-open-source)
- [预览限制](https://developers.openai.com/siwc/token-sharing-open-source/preview-limitations)
- [自托管 VM](https://developers.openai.com/siwc/token-sharing-open-source/self-hosted-vms)

### DeepSeek

当前模型表列出 `deepseek-flash` 与 `deepseek-v4-pro`：两者支持文本推理、JSON 输出和工具调用，**只有 Flash 支持视觉**。支持图片 OCR、描述及结合输出约束的字段提取；识别效果尚未验证。

视觉输入支持 JPEG、PNG、GIF、WebP；Files API 是上传图片，不代表通用文档解析。Responses 文档明确不支持文件输入，因此 PDF、DOC/DOCX 需要 kist 先提取文本或转图片。本次未找到官方音频转录或直接视频输入支持依据。托管 `file_search` 不支持，不影响本地查询工具。

JSON Output 与严格 schema 约束不同；工具调用有 strict Beta 模式，语义正确性仍需校验。图片会缩放，大图约缩至 1300×1300 等效总像素，每图最多 1024 token；密集小字需要实测，分块可能增加成本。

价格快照：每百万未缓存输入/输出 token，Flash 低峰 $0.15/$0.60、高峰 $0.30/$1.20；Pro 低峰 $0.66/$1.98、高峰 $1.32/$3.96。高峰为中国非节假日周一至周五北京时间 09:00–12:00、14:00–18:00。这不是每页固定价格，实际消耗取决于输入、输出、推理和重试。

- [模型与价格](https://api-docs.deepseek.com/quick_start/pricing/)
- [视觉](https://api-docs.deepseek.com/guides/vision/)
- [Files API](https://api-docs.deepseek.com/guides/files_api/)
- [Responses 支持范围](https://api-docs.deepseek.com/guides/responses_api/)
- [JSON Output](https://api-docs.deepseek.com/guides/json_mode/)、[工具调用](https://api-docs.deepseek.com/guides/tool_calls/)

### 阿里云百炼

- 语音转文字：Qwen-ASR、Fun-ASR 等覆盖实时和录音文件识别，部分支持时间戳、说话人分离及视频音轨输入。
- 图片与视频：有支持相应输入的视觉模型；视频画面理解和音轨转录要分别核对。
- OCR：Qwen3.5-OCR 文档列出文字定位、关键信息提取、卡证、多轮对话和 PDF 解析。
- PDF：有专门的 PDF 理解路径，处理文字和图片。
- Word、Excel、PPT：文件问答支持 DOC/DOCX、XLS/XLSX、PPT/PPTX 等；走配置好的应用和解析流程，调用前需发布应用，不等同于任意模型接口直接接收 Office 文件。

Qwen3.5-OCR 北京地域价格快照：每百万输入 token ¥0.5、输出 ¥2。不同模型的图片计费和输出量不同，不能仅按 token 单价比较每份档案成本。OpenAI 兼容接口不覆盖全部 DashScope OCR 高级参数。

- [语音识别](https://help.aliyun.com/zh/model-studio/asr-model)
- [视觉理解](https://help.aliyun.com/zh/model-studio/vision)
- [Qwen-OCR](https://help.aliyun.com/zh/model-studio/qwen-vl-ocr)
- [PDF 理解](https://help.aliyun.com/zh/model-studio/pdf-understanding)
- [文件问答](https://help.aliyun.com/zh/model-studio/file-q-a)
- [价格](https://help.aliyun.com/zh/model-studio/model-pricing)

### AWS Bedrock

多模型平台，输入、结构化输出及区域支持必须具体到模型。Converse 文档格式包含 PDF、DOC、DOCX、XLS、XLSX、CSV、HTML、TXT、Markdown，不能据此认为支持 PPT 或所有格式的完整视觉布局理解。

另有 Bedrock Data Automation（BDA）处理文档、图片、音频、视频，音频可输出转录、时间戳和摘要。普通模型调用与 BDA 是不同接入路径，各有格式与限制；不把 AWS 所有服务合并成一个聊天接口的能力。

- [模型目录](https://docs.aws.amazon.com/bedrock/latest/userguide/model-cards.html)
- [Converse 文档类型](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_DocumentBlock.html)
- [结构化输出](https://docs.aws.amazon.com/bedrock/latest/userguide/structured-output.html)
- [BDA 格式与限制](https://docs.aws.amazon.com/bedrock/latest/userguide/bda-limits.html)
- [BDA 音频](https://docs.aws.amazon.com/bedrock/latest/userguide/audio-processing.html)

## 5. 后续选择依据

用户调研第二后端时关心的是**语音转文字及图片、视频、PDF、Word 等文件理解的覆盖面**，不只是图片 OCR 单价。

助手建议：若补齐媒体与文件类别优先，先评估百炼；若希望增加低价图片与文本模型，评估 DeepSeek Flash；Bedrock 作为覆盖广但需区分模型调用与 BDA 的候选。**这不是已选定的接入顺序。**

正式选型前用同一批真实样本比较漏字、证件号码与日期错误、字段虚构、来源定位、延迟和总费用。评价“每份档案正确提取需要多少钱、用户需要改多少”，而不只看每百万 token 价格。OCR、描述、KV 是否一起首发、是否首版即可检索提取结果，均待确定。
