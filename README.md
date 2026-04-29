# xfyun2openai

`xfyun2openai` 是一个轻量级 OpenAI Images API 兼容代理。它把客户端发来的 OpenAI 风格图片生成请求，转换成讯飞开放平台图片生成 WebAPI 请求，并提供基于 YAML 的模型映射、多 app 凭证池、轮询和失败切换能力。

## 项目简介

项目适合本地部署、私有网关封装或作为已有应用的图片代理层。

## 功能特性

- `GET /healthz` 健康检查接口。
- `GET /v1/models` 按配置动态返回 OpenAI 风格模型列表。
- `POST /v1/images/generations` 兼容 OpenAI Images API 的基础请求格式。
- 支持 `.env` 加载与 YAML 中 `${ENV_NAME}` 展开。
- 支持多模型映射到不同 `model_id` 和 endpoint。
- 支持单模型挂多 app，按 round-robin 选择。
- 支持 app 级并发限制、失败计数和 cooldown。
- 支持网络错误、HTTP 5xx、讯飞 `10008` 容量不足时切换 app 重试。
- 日志包含 `request_id`、`model`、`selected_app_name`、`xfyun_sid`、`latency`、`status`，不打印 prompt 全文和 `api_secret`。

## 兼容范围与限制

- 第一版只支持 `response_format=b64_json`。
- `response_format=url` 会返回 `400`。
- 暂不支持图片编辑、variation、Web 管理面板和数据库。
- 当前转发核心字段使用 `model_id`；`resource_id` 在配置中保留，便于与控制台模型信息对齐。

## 快速开始

1. 准备环境变量文件：

```powershell
Copy-Item .env.example .env
```

2. 准备配置文件：

```powershell
Copy-Item config.example.yaml config.yaml
```

3. 把你的 `APPID`、`APIKey`、`APISecret` 写入 `.env`，把 `PROXY_API_KEY` 改成自己的代理密钥。

4. 启动服务：

```powershell
$env:CONFIG_PATH = (Resolve-Path .\config.yaml).Path
go run .\cmd\server
```

5. 验证健康检查：

```powershell
curl.exe http://localhost:8787/healthz
```

## 配置说明

`config.yaml` 支持以下主要段落：

- `server.listen`: HTTP 监听地址，默认 `:8787`。
- `server.api_keys`: 代理自身的 Bearer Token 列表。为空时允许无鉴权访问，但生产环境应始终配置。
- `xfyun.default_endpoint`: 模型未单独指定 endpoint 时使用的默认值。
- `xfyun.default_timeout_seconds`: 上游 HTTP 超时时间。
- `routing.max_retries`: 单张图最大重试次数，不含首次调用。
- `routing.cooldown_seconds`: app 进入 cooldown 的时长。
- `apps.<name>`: 讯飞应用凭证与最大并发。
- `models.<name>`: OpenAI 模型名到讯飞 `model_id` 的映射，以及默认图片参数。
- `models.<name>.patch_id`: 某些非全量训练模型必需，可从讯飞控制台获取；请求体里的 `x_fyun.patch_id` 会覆盖它。

YAML 文件会经过 `os.ExpandEnv`，所以可以直接写 `${ENV_NAME}`。

最小可用 `config.yaml` 示例：

```yaml
server:
  listen: ":8787"
  api_keys:
    - "${PROXY_API_KEY}"

xfyun:
  default_endpoint: "https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti"
  default_timeout_seconds: 120

routing:
  max_retries: 2
  cooldown_seconds: 60

apps:
  app-main:
    app_id: "${XFYUN_APP_MAIN_APP_ID}"
    api_key: "${XFYUN_APP_MAIN_API_KEY}"
    api_secret: "${XFYUN_APP_MAIN_API_SECRET}"
    max_concurrency: 2

models:
  z-image-turbo:
    display_name: "Z-Image-Turbo"
    model_id: "xopzimageturbo"
    resource_id: "0"
    patch_id: "${Z_IMAGE_TURBO_PATCH_ID}"
    endpoint: "https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti"
    apps:
      - app-main
    defaults:
      size: "1024x1024"
      steps: 20
      guidance_scale: 5
      scheduler: "Euler"
```

## 请求映射说明

OpenAI 请求会被映射为讯飞 `header / parameter / payload` 结构：

| OpenAI 字段 | 讯飞字段 | 说明 |
| --- | --- | --- |
| `model` | `parameter.chat.domain` | 先查 `models.<name>`，再取其中的 `model_id` |
| `user` | `header.uid` | 未传时使用 `request_id` |
| `x_fyun.patch_id` | `header.patch_id` | 请求级覆盖；未传时回退 `models.<name>.patch_id` |
| `prompt` | `payload.message.text[0].content` | `role` 固定为 `user` |
| `size` | `parameter.chat.width/height` | 仅支持 6 种尺寸 |
| `x_fyun.negative_prompt` | `payload.negative_prompts.text` | 可选 |
| `x_fyun.seed` | `parameter.chat.seed` | 未传时代理随机生成 |
| `x_fyun.steps` | `parameter.chat.num_inference_steps` | 请求优先级最高 |
| `x_fyun.guidance_scale` | `parameter.chat.guidance_scale` | 请求优先级最高 |
| `x_fyun.scheduler` | `parameter.chat.scheduler` | 请求优先级最高 |

参数优先级固定为：`请求中的 x_fyun` > `model.defaults` > 系统默认值。

## 路由与重试

- 每个 model 维护自己的 round-robin 游标。
- 每个 app 维护全局 `in_flight`、`fail_count` 和 `cooldown_until`。
- 选 app 时会跳过 cooldown 中的 app，以及已经达到 `max_concurrency` 的 app。
- 同一张图在重试链路中不会重复打到同一个 app。
- 以下情况会换下一个 app 重试：网络错误、超时、HTTP 5xx、讯飞 `10008`、明确的上游鉴权失败。
- 以下情况不会重试：`10003/10004/10005` 参数错误，`10021/10022` 内容审核失败。
- `n>1` 时按顺序生成多张图；显式传入 `seed` 时使用 `seed + index`。

## 调用示例

### OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-local-test",
    base_url="http://localhost:8787/v1",
)

result = client.images.generate(
    model="z-image-turbo",
    prompt="一只赛博朋克风格的橘猫，霓虹灯，电影感",
    size="1024x1024",
    n=1,
    response_format="b64_json",
)

print(result.data[0].b64_json[:100])
```

## 多模型 / 多 app 配置示例

```yaml
apps:
  app-main:
    app_id: "${XFYUN_APP_MAIN_APP_ID}"
    api_key: "${XFYUN_APP_MAIN_API_KEY}"
    api_secret: "${XFYUN_APP_MAIN_API_SECRET}"
    max_concurrency: 2
  app-backup:
    app_id: "${XFYUN_APP_BACKUP_APP_ID}"
    api_key: "${XFYUN_APP_BACKUP_API_KEY}"
    api_secret: "${XFYUN_APP_BACKUP_API_SECRET}"
    max_concurrency: 1

models:
  z-image-turbo:
    model_id: "xopzimageturbo"
    resource_id: "0"
    patch_id: "${Z_IMAGE_TURBO_PATCH_ID}"
    endpoint: "https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti"
    apps: ["app-main", "app-backup"]
    defaults:
      size: "1024x1024"
      steps: 20
      guidance_scale: 5
      scheduler: "Euler"

  qwen-image-2512:
    model_id: "xopqwentti20b"
    resource_id: "0"
    patch_id: "${QWEN_IMAGE_2512_PATCH_ID}"
    endpoint: "https://maas-api.cn-huabei-1.xf-yun.com/v2.1/tti"
    apps: ["app-main"]
    defaults:
      size: "1024x1024"
      steps: 20
      guidance_scale: 5
      scheduler: "Euler"
```

## Docker 部署

每次 push 到 GitHub 后，仓库会通过 GitHub Actions 自动构建并推送镜像到 GHCR：

`ghcr.io/futureppo/xfyun2openai`

镜像标签规则：

- `latest`：默认分支最新提交
- `sha-<commit>`：每次 push 的提交镜像
- `<branch-name>`：分支名镜像
- `v*`：Git tag 镜像

如果你的仓库或 package 还不是公开的，首次使用前需要先确认 GHCR package 可见性，或者在目标机器上先执行 `docker login ghcr.io`。

直接拉取镜像：

```powershell
docker pull ghcr.io/futureppo/xfyun2openai:latest
```

使用 compose 直接拉取并启动：

```powershell
docker compose -f .\docker-compose.example.yml pull
docker compose -f .\docker-compose.example.yml up -d
```

如果要固定到某个构建版本，可以覆盖 `IMAGE_TAG`，例如：

```powershell
$env:IMAGE_TAG = "sha-<commit>"
docker compose -f .\docker-compose.example.yml pull
docker compose -f .\docker-compose.example.yml up -d
```

容器默认读取 `/app/config.yaml`，请通过只读挂载提供真实配置文件。

## License

本仓库当前许可证为 `GNU AGPL-3.0`，以 [LICENSE](./LICENSE) 为准。
