# 音视频语音转文字真实服务验收

- 日期：2026-10-31
- 触发：用户明确要求下载公开真人录音及视频，调用真实 OpenAI API 验证，而不只运行合成／模拟测试。
- 实现基线：`c178a60`。
- 模型：`gpt-transcribe`，实际调用 `/v1/audio/transcriptions`。

## 样本及结果

| 样本 | 来源 | 原始时长／大小 | 实际上传 | 从入队到结果就绪 | 文本比对 |
|---|---|---|---|---|---|
| JFK 演讲录音 | [whisper.cpp 公共录音样本](https://github.com/ggml-org/whisper.cpp/tree/master/samples) | 11 秒／352,078 字节 | 原始 WAV，`audio/wave`，352,078 字节，SHA-256 与下载文件相同 | 2.12 秒 | 22 个参考词，0 次词级编辑 |
| W3C Conformance Evaluation 教学视频 | [W3C 官方视频和文字稿](https://www.w3.org/WAI/test-evaluate/conformance/)，[媒体及字幕目录](https://media.w3.org/wai/evaluation-intros/) | 128.64 秒／30,767,440 字节；H.264 画面和双声道 AAC 音轨 | 先提取为 `audio.wav`，16 kHz 单声道 PCM，`audio/wav`，4,116,558 字节；没有上传 MP4 或画面 | 5.30 秒 | 与官方 SRT 比对，235 个参考词、236 个输出词，3 次编辑，WER 1.28% |

耗时来自主动推进隔离库队列的实际请求，不包含下载、导入和后台排队等待，不是纯 API 网络延迟。模型没有收到参考文本或字幕作为提示。词级比对忽略标点和大小写，保留英文缩写及缩合形式；WER 是与参考文本的差异，并不代表人工听辨确认的错误率。视频差异是一处 `in` 的省略及额外两个词，未据此自动改写转录结果。

固定下载来源及 SHA-256：

- `https://raw.githubusercontent.com/ggml-org/whisper.cpp/master/samples/jfk.wav`
  - `59dfb9a4acb36fe2a2affc14bacbee2920ff435cb13cc314a08c13f66ba7860e`
- `https://media.w3.org/wai/evaluation-intros/conformance-evaluation.mp4`
  - `ec419be88b5d6cf2354a366caae8012a979438decef0154a6087a9813c662dbb`
- `https://media.w3.org/wai/evaluation-intros/conformance-evaluation-cc.srt`
  - `4044ea879e264cb662f24c9c3eeefd9e55c3383353b53bb3bd20661be647ab31`

## 实际覆盖的链路

使用配置中指定 OpenAI API 账户的同一个 provider／代理工厂，真实发送 API 请求；输入只包含上述公开样本。凭据仅在测试进程内读取，不复制到临时配置、日志或报告。账户配置和私人归档库没有被修改；样本导入和结果持久化在独立临时库中完成。

测试走现有转录队列、原始字节或视频抽轨准备、模型调用、metadata 提交及输出模型来源；观察器记录上传文件名、MIME、字节数及校验值，确认视频先变成音频再上传。两个样本分别只上传一次。结果就绪后再次请求不会重复计费调用；归档 `rebuild` 后快照一致，`verify --deep` 通过。测试本身通过，未发现需要改变生产转录逻辑的问题。

此前的分段顺序、末段覆盖、中途失败不保存部分正文、取消、工具缺失和临时音频清理由合成测试覆盖；本轮两个真实样本均在一个准备段内完成，不能当作真实长录音跨段质量的证据。两个样本均为英语，不代表中文、歌曲、多人重叠说话或噪声场景已经验收。实际浏览器 UI 仍受原生连接故障影响，本轮验收的是服务端真实处理链路。

## 复验入口

`TestLiveTranscriptionPublicMedia` 默认跳过，不会在 CI 或普通 `go test` 中调用付费服务。复验前将上述公开文件以固定文件名放入独立目录，并显式指定有权限使用的本机配置及 OpenAI API 账户编号：

```sh
KIST_LIVE_TRANSCRIPTION_CONFIG=/path/to/source/kist.toml \
KIST_LIVE_TRANSCRIPTION_ACCOUNT=2 \
KIST_LIVE_TRANSCRIPTION_INPUTS=/tmp/public-transcription-samples \
go test -count=1 -timeout=8m ./internal/library \
  -run '^TestLiveTranscriptionPublicMedia$' -v
```

目录应包含 `jfk.wav` 和 `conformance-evaluation.mp4`，上传前校验上述固定 SHA-256，拒绝不匹配的文件。只允许按明确授权使用该入口；它会产生真实 API 请求和费用。成功后在该输入目录写出本机 `live-results.json`，包含公开样本的输出和上传观察数据，不包含凭据。媒体、字幕及完整转录结果不提交到仓库。
