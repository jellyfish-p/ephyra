# “复用 Hydro worker”的范围、接口与许可证据

> **性质：研究证据，不是已接受的架构决定。** 本文不选择复用路线、不实现适配、不修改 Ephyra 的许可证，也不声称任何路线已与 Nuxt / Go + Gin + GORM 验证兼容。许可证部分是需要继续核实的合规问题，不是法律意见。
>
> 核查日期：**2026-10-15（UTC）**。方法：读取官方仓库、官方文档源码、版本化 npm 元数据和 LICENSE；仓库检出均在系统临时目录。没有安装或运行评测栈，没有做安全审计或完整传递依赖审计。研究启动时 Ephyra 只有简介与根 MIT LICENSE，不能据此推定引入的第三方代码也属于 MIT。

## 1. 版本范围与名称辨析

| 对象 | 本次检查的版本 / 修订 | 证据范围 |
| --- | --- | --- |
| `hydro-dev/Hydro` | [`d6b633932cb51b6d76b678245b17a698489b826f`](https://github.com/hydro-dev/Hydro/commit/d6b633932cb51b6d76b678245b17a698489b826f)，提交日期 2026-10-14 | 当前单仓库；其中 `@hydrooj/hydrojudge` 的源码包版本为 `4.0.6` |
| Hydro 官方文档仓库 | [`hydro-dev/hydro-dev.github.io@7e6c11f7fa31a9b7523b1ce2860394e010393005`](https://github.com/hydro-dev/hydro-dev.github.io/commit/7e6c11f7fa31a9b7523b1ce2860394e010393005)，2026-06-21 | 当前公开文档地址为 `docs.hydro.ac`；主要核查 hydrojudge、测试数据文档 |
| 旧 `hydro-dev/HydroJudge` | [`7e9efc1980b9c291245a9d555c6aa884364f9bcf`](https://github.com/hydro-dev/HydroJudge/commit/7e9efc1980b9c291245a9d555c6aa884364f9bcf)，2020-11-08 | 已归档；README 明确写着迁移到 Hydro；包版本 `1.4.5`，不能当作当前实现 [L1] [L-legacy-readme] |
| `criyle/go-judge` | [`4ebd2ffe9474639c665fa21cfb438f68820fe77c`](https://github.com/criyle/go-judge/commit/4ebd2ffe9474639c665fa21cfb438f68820fe77c)，2026-10-13 | 通用受限执行服务，不是 Hydro 平台，也不是完整 OJ 判题策略 [G1] |
| `criyle/go-sandbox` | [`v0.14.2 / e9dd1e32c9fea093c80c9f885f7bd724ce70c86d`](https://github.com/criyle/go-sandbox/commit/e9dd1e32c9fea093c80c9f885f7bd724ce70c86d)，2026-10-11 | 上述 go-judge 的 `go.mod` 指定的 Linux 沙箱依赖 [G2] |

npm 的 [`@hydrooj/hydrojudge@4.0.6` 元数据](https://registry.npmjs.org/@hydrooj%2fhydrojudge/4.0.6)也声明 `AGPL-3.0-or-later`，但其依赖范围与本次 Git HEAD 中同版本号的 `package.json` **不同**（例如发布包的 `superagent ^10.3.0`，HEAD 为 `^10.4.1`）。因此，下文源码行为的依据是上述 SHA，**不能把它直接等同于 npm 发布包的逐字节内容**；实际采用的发布物、依赖解析结果仍待锁定。

### 实际上至少有四层

1. **Hydro 平台后端**：维护用户/权限、题目、提交记录、任务队列、语言配置、文件存储，以及回写分数、比赛统计等。[H-server] [H-record]
2. **`@hydrooj/hydrojudge` 高层评测器**：Node.js / TypeScript；取任务、同步数据、读题目配置、编译、组织测试点/子任务、调用比较器或交互器、上报判题结果。它有 **Hydro 内置插件**和**独立 daemon**两种入口，不是两个独立的 npm 判题产品。[H-package] [H-entry] [H-daemon] [H-task]
3. **`go-judge` 执行服务**：用 REST / WebSocket 等接口在受限环境运行进程，提供资源限制、文件缓存、进程管道等。它返回的 `Accepted` 表示程序在限制内以 0 退出，**不代表答案正确**。[G1] [G-model]
4. **`go-sandbox` Linux 底层库**：提供 namespace、cgroup、seccomp、预创建容器等机制；不是 Hydro 任务接收器，也不是完整网络评测服务。[G-sandbox]

**关键区别：** 独立 `hydrojudge` 可以与 Hydro 后端部署在不同机器，不必直接连接 Hydro 数据库；但“独立进程”不意味着“与 Hydro 协议和题目语义无关”。`go-judge` 则没有 Hydro 用户、题目、提交记录等概念，可以作为通用执行服务运行。[D-judge] [H-host] [G-model]

## 2. 复用层级对照：仅列候选含义，不作选择

| “复用”的可能含义 | 已有能力 / 可独立到哪一层 | 限制与尚未证明之处 |
| --- | --- | --- |
| 保留 Hydro 平台及其评测栈，Ephyra 与它交互 | 保留 Hydro 队列、权限、存储、判题语义；内置或独立评测器均为官方用法 | 仍然保留另一套平台后端和数据模型；没有证明账号、题目、提交记录能与 Ephyra 自然统一；不是直接替换为 GORM |
| 运行未修改的独立 `hydrojudge`，由其他后端提供它期望的接口 | 官方支持远程连接 Hydro；源码也内置 `vj4` host | **候选适配方式而非现成功能**：其他后端必须处理 Hydro 登录、WS、数据下载、任务/结果语义等；没有通用 `POST /judge` 接口可直接接 Go 后端 |
| 提取 / fork 高层评测器，替换 `Session` / host | 内部 `Session` 已把语言、文件和回报接口集中起来，存在可研究的接入点 | 不是稳定独立 SDK 承诺；默认入口是 Hydro 插件；仍依赖 Hydro 配置、状态、工具包、Node 运行时，并涉及 AGPL 代码修改与组合边界 |
| 只运行 `go-judge` 服务 | 与 Hydro 无关的进程执行、限制、文件、管道 API | 不附送取题、编译策略、SPJ 协议、分数聚合、OJ 队列和重测语义；“可调用”不等于“完整判题兼容” |
| 直接使用 `go-sandbox` 库 | Linux 底层隔离和执行机制，可用于 Go 程序 | 比 go-judge 更底层；没有其完整网络服务和文件管理层，更没有 Hydro 判题能力；本次没有验证与 Gin/GORM 的集成 |

前两行涉及 Hydro 平台/判题器的 AGPL 许可，后三行也不能仅凭进程边界或语言相同来推定整体许可；详见第 5 节。依据：[D-judge] [H-entry] [H-builtin] [H-daemon] [H-session] [G1] [G-sandbox]。

## 3. 独立 hydrojudge 的实际接入面

### 3.1 独立的是部署，不是上游协议

官方文档要求创建具有 `PRIV_JUDGE` 权限的账号，在 `judge.yaml` 的 `hosts` 中填写 `type: hydro`、`server_url`、`uname`、`password`，再运行 `hydrojudge`。当前 CLI 从 `bin/hydrojudge.js` 经 `@hydrooj/register` 进入 `src/daemon.ts`；daemon 内置选择 `HydroHost` 或 `Vj4Host`。[D-judge] [H-entry] [H-daemon]

与此不同，插件 `src/index.ts → hosts/builtin.ts` 直接使用 `hydrooj` 的 `TaskModel`、`RecordModel`、`StorageModel`、设置和结果回调；这一入口不是与 Hydro 后端脱耦的通用库。独立 Hydro host 的 `hydrooj` 引用是类型导入，`package.json` 的运行依赖也没有整个 `hydrooj` 平台包；其运行路径通过网络而非平台数据库访问任务和数据。[H-plugin] [H-builtin] [H-host] [H-package]

### 3.2 传输、鉴权和文件依赖

| 功能 | 检查到的客户端 / 服务端行为 | 对“接其他后端”的含义 |
| --- | --- | --- |
| 登录 | `POST login`，传 `uname/password/rememberme: on/judge: on`；保存响应 Cookie；用 `GET judge/files` 检查会话 | 不是一个任意共享密钥的标准 worker 协议；当前路径依赖 Hydro 会话登录行为 |
| WS 鉴权 | 客户端主动连接 `judge/conn`，把 Cookie 中的 `sid` 放进 `Authorization: Bearer …`；后端把它作为 session token 解释 | Bearer 在这里不是另发的 JWT；HTTP Cookie 与 WS 会话语义要对应 |
| 权限 | `/judge/files`、`/judge/upload`、`/judge/conn` 均要求 `PRIV_JUDGE` | 需要表达评测端的特权边界；不是普通选手权限 |
| 控制与任务 | 服务端发送 `{language: …}`、`{task: …}`；客户端发送 `config`、`start`、`status`、`ping`；配置包括优先级门槛、并发及语言筛选；还处理纯文本 `ping/pong` | 不是单次 HTTP 请求-响应；包含连接状态、能力/语言配置和任务生命周期 |
| 结果 | 客户端发带 `rid`、`key: next/end` 的消息；`next` 可多次发送进度、测试点、编译输出；`end` 结束任务，可携带批量 `cases` | 不能只返回一个 AC/WA 字符串；需要处理增量、终态和结果关联 |
| 题目数据 | `POST /d/{domainId}/judge/files`，请求 `{pid, files}`；返回 `{links}`，再逐个下载 | 需要提供文件清单与下载地址，不只是上传一个题目 ZIP |
| 提交附件 | `POST judge/files`，请求 `{id}`，获得 `{url}`，下载代码/答案/hack 等记录资源 | `files.code` 等引用必须能解析；源码也可直接放在任务的 `code` 中 |
| 生成文件回传 | `POST judge/upload`，multipart 字段 `rid/name/file`；Hydro 服务端处理题目修改权限、大小和文件数量限制 | 对涉及生成/回传的能力，仅实现下载接口不够 |

证据：[H-host] [H-server] [H-auth]。`judge.yaml` 的 `secret` 用于本地题目校验相关逻辑，不能与上述登录凭证或 go-judge 的 `auth-token` 混为一谈。[H-config] [H-task] [H-cases]

**队列语义也在后端。** `JudgeConnectionHandler` 按连接维护进行中的 `rid`，收到 `end` 才结束等待；断线时会重置并重新加入未完成任务。结果回写还可更新题目通过状态、比赛状态及统计。客户端断线后重连并不自动证明在新后端上具有恰好一次、幂等、取消或重测顺序保证；这些是兼容性待验事项。[H-server]

### 3.3 可公开核查的 schema / 配置入口

| 入口 | 作用 |
| --- | --- |
| `packages/common/types.ts` | `JudgeRequest`、`JudgeResultBody`、`FileInfo`、`ProblemConfigFile`、`JudgeMeta`、测试点和子任务结果 [H-types] |
| `packages/common/status.ts` | 数字状态码：如 AC=1、WA=2、CE=7、SE=8，以及 FETCHED、FORMAT_ERROR、HACK 等 [H-status] |
| `packages/common/lang.ts` | `LangConfig`：编译/运行命令、源码文件名、语言倍率、编译资源限制等 [H-lang] |
| `packages/hydrooj/src/model/record.ts` | 平台如何拼装任务、解析引用题目、填 `source/data/config/meta/trusted`，以及将远程题路由到另一类任务 [H-record] |
| `packages/hydrojudge/src/interface.ts` | 内部 `Session`：`getLang`、`getReporter`、`fetchFile`、`postFile` 及配置 [H-session] |
| `packages/hydrojudge/src/config.ts` | 沙箱地址、缓存/临时目录、并发、总时长、进程数、内存上限等 [H-config] |
| `packages/hydrojudge/src/sandbox/interface.ts` 与 `go-judge/cmd/go-judge/model/model.go` | 下层执行请求/响应；与 Hydro 上层任务不是同一 schema [H-sandbox-types] [G-model] |

`JudgeRequest` 延伸了 Hydro 的提交记录字段，包含 `rid/domainId/pid/uid/lang/code`、`source`、`data`、`config`、`meta`、`type`、`trusted`，并可含 `files/input/contest`。`source` 在 Hydro 中通常是 `domainId/pid`，用于题目寻址和缓存；`data` 中的 `name/etag/lastModified` 用于判定哪些文件需要刷新。不是任意文件 URL 列表即可代替。[H-types] [H-record] [H-task]

上层结果的 `time` 为毫秒、`memory` 为 KB；下层 go-judge 的时间为纳秒、内存为字节，Hydro 还会做倍率和状态映射。两层都出现 `Accepted/time/memory`，但不能直接互换。[H-types] [H-sandbox] [H-sandbox-types]

下层 go-judge 的公开入口包括 `POST /run`、`GET/POST /file`、`GET/DELETE /file/:fileId`、WebSocket `/ws` / `/stream`、`GET /version` / `/config`，也可选择启用 gRPC。`/run` 的核心是 `cmd[]`（命令参数、环境、资源限制、copyIn/copyOut）与可选 `pipeMapping`，返回各进程执行结果；并不直接接收 Hydro 的题目、提交或子任务对象。[G1] [G-model]

这些是**公开源码中的契约入口**，不是本次发现的一份独立、版本化、承诺兼容第三方后端的 worker 协议规范。本次没有发现或验证 Ephyra 适配器。

## 4. 与 Hydro 题目格式及评测类型的耦合

**事实：高层评测器不仅是一层沙箱代理。**

- `readCases` 将任务配置与数据目录的 `config.yaml` / `config.yml` / `config.ini` 合并；本地文件中的配置可以覆盖传入配置。没有显式测试点时还会按文件名自动识别。时间/内存单位、语言倍率、文件输入输出、子任务分组及依赖均参与解释。[H-cases] [H-common-cases] [D-data]
- `ProblemConfigFile` 含 `subtasks`、`min/max/sum` 聚分、`if` 依赖、`checker_type`、`checker/interactor/manager/validator`、`user_extra_files/judge_extra_files`、`answers` 等。检查器不是一个统一的任意命令：`testlib`、`hustoj`、`lemon`、`qduoj`、`syzoj`、`kattis` 等有不同参数、退出码或分数解析规则。[H-types] [H-checkers]
- `judge/index.ts` 分派 `default`、`interactive`、`communication`、`submit_answer`、`objective`，以及任务模式 `run`、`generate`、`hack`。自测 `run` 使用特定全零 `contest` 标识，生成任务看 `request.type`，hack 看附件；这说明复用还涉及 Hydro 的任务语义，而不只是题型名称。[H-judges] [H-task]
- `remote_judge` 虽出现在公共类型中，但平台会把一般远程题转换成 `remotejudge` 任务；上述本地 judge 分派表没有这一项。不能据“Hydro 支持 VJudge”推定这个 daemon 自带所有远程 OJ 提交能力。[H-types] [H-record] [H-judges]
- 语言的编译/执行命令来自 Hydro host 收到的 `language` 配置；编译器和解释器仍需安装在执行环境。当前源码没有证明只放一个独立 `langs.yaml` 就能覆盖 Hydro host 收到的语言表，官方文档与实际选定发布物的配置行为仍需对齐。[H-host] [H-lang] [D-judge]
- 当前校验有测试点数、总时长、单进程/多进程及内存限制；`trusted` / key 还会影响是否走普通配置校验。题包及配置本身是信任边界，不能把“可以运行用户代码”理解成“可以无条件信任任意题包”。[H-cases] [H-task]

**研究推论，不是决定：** 若复用完整评测语义，Ephyra 至少需要讨论是接受 Hydro 题包/计分语义、转换某个子集，还是维护另一套语义与适配层。字段转换的存在不等于各题型的判题行为已等价；尤其 SPJ、交互、提交答案和子任务计分尚未实测。

## 5. 许可证：必须按作用域看，不能只看仓库首页

### 5.1 核心组件与紧邻依赖

| 作用域 | 查到的确切声明 | 边界 / 未知 |
| --- | --- | --- |
| Hydro 根仓库 | `LICENSE` 是 GNU AGPL v3 正文；根 `package.json` 写 `AGPL-3.0-only` [L-root] [L-workspace] | 不能拿根字段覆盖所有有独立声明的子包 |
| `packages/hydrojudge`，4.0.6 | 包元数据 **`AGPL-3.0-or-later`** [H-package] | 本次未见该目录另有 LICENSE；不是 MIT，也不是旧库的 GPL-only |
| `framework/utils`，`@hydrooj/utils` 1.5.8 | **`AGPL-3.0-or-later`** [L-utils] | judge/common 的实际依赖；仅改 host 不会消除它 |
| `packages/common`，0.0.11 | 包元数据**没有 `license` 字段**；目录未见独立 LICENSE；[npm 同版本](https://registry.npmjs.org/@hydrooj%2fcommon/0.0.11)也未声明 [L-common] | 仓库层面有 AGPLv3、根元数据为 `-only`；不能推定为 MIT，亦不能擅自补成 `-or-later`。复制 schema/实现前应确认该包及发布物的具体授权 |
| `framework/register`，1.0.4 | **MIT** [L-register] | 同仓库中确有 MIT 子包，但不使 judge/utils 变成 MIT |
| `packages/hydrooj`，5.0.7 | **`AGPL-3.0-or-later OR Proprietary`** [L-core] | `Proprietary` 不是自动授予的任意使用权；本次没有额外授权协议，也不能据此推定 hydrojudge 享有相同双授权 |
| 旧 HydroJudge，1.4.5 | 包元数据 **`GPL-3.0-only`**，根 LICENSE 为 GPLv3 [L1] [L-legacy-license] | 已归档，不能用这个许可标签描述当前包 |
| go-judge（本次 SHA） | 根 LICENSE：**MIT**，Copyright 2019 Yang Gao [L-gojudge] | 仍有外部依赖及嵌入资源的许可证，不等于整个二进制只有 MIT |
| go-sandbox v0.14.2 | 根 LICENSE：**MIT**，Copyright 2019 criyle [L-sandbox] | 部分 `vfork` 汇编文件头另带 Go Authors 的 **BSD-style** 声明；其精确来源/文本匹配仍需文件级复核，不能删除通知 [L-vfork] |
| vendored testlib | Hydro 子模块固定于 [`2d20123984e9479b8a56ebe0d6a51e23ad7c35b3`](https://github.com/MikeMirzayanov/testlib/tree/2d20123984e9479b8a56ebe0d6a51e23ad7c35b3)；根 LICENSE 为 **MIT** [L-testlib] | `testlib.h` 文件头另有保留通知及标示修改的许可文字；不能只复制头文件却丢弃通知，也不能忽略其他题目自带 checker 的授权 [L-testlib-header] |

### 5.2 已核查的依赖许可（有界抽查，不是完整 SBOM）

下表 npm 版本取自本次 HEAD 中依赖范围的起始版本，读取了**该确切版本**的官方 registry `license` 元数据；它们**不是一次真实安装的锁定结果**。这覆盖 hydrojudge 的全部非 Hydro 直接运行依赖，以及 utils/register 声明的非 Hydro 直接运行依赖。可选 peer、开发依赖和继续向下的传递依赖未全量核查。

| 依赖及版本（链接为已查询元数据） | 声明 |
| --- | --- |
| [`@opentelemetry/api@1.9.1`](https://registry.npmjs.org/@opentelemetry%2fapi/1.9.1)、[`exporter-trace-otlp-http@0.223.0`](https://registry.npmjs.org/@opentelemetry%2fexporter-trace-otlp-http/0.223.0)、[`sdk-node@0.223.0`](https://registry.npmjs.org/@opentelemetry%2fsdk-node/0.223.0) | Apache-2.0 |
| [`@zip.js/zip.js@2.23.0`](https://registry.npmjs.org/@zip.js%2fzip.js/2.23.0) | BSD-3-Clause |
| [`semver@7.8.5`](https://registry.npmjs.org/semver/7.8.5) | ISC |
| [`cac@7.1.0`](https://registry.npmjs.org/cac/7.1.0)、[`p-queue@9.3.5`](https://registry.npmjs.org/p-queue/9.3.5)、[`schemastery@3.18.0`](https://registry.npmjs.org/schemastery/3.18.0)、[`shell-quote@1.12.0`](https://registry.npmjs.org/shell-quote/1.12.0)、[`superagent@10.4.1`](https://registry.npmjs.org/superagent/10.4.1)、[`ws@8.22.0`](https://registry.npmjs.org/ws/8.22.0) | MIT |
| utils 的 [`fs-extra@11.4.1`](https://registry.npmjs.org/fs-extra/11.4.1)、[`js-yaml@5.4.3`](https://registry.npmjs.org/js-yaml/5.4.3)、[`reggol@2.1.0`](https://registry.npmjs.org/reggol/2.1.0)、[`search-query-parser@1.6.0`](https://registry.npmjs.org/search-query-parser/1.6.0)、[`systeminformation@5.33.15`](https://registry.npmjs.org/systeminformation/5.33.15) | MIT |
| register 的 [`esbuild@0.25.2`](https://registry.npmjs.org/esbuild/0.25.2)、[`source-map-support@0.5.21`](https://registry.npmjs.org/source-map-support/0.5.21) | MIT |

Go 侧以 [go-judge 的 `go.mod`][G2] 和 [go-sandbox 的 `go.mod`][G-sandbox-mod] 为依赖证据，已直接读取下列确切版本的 LICENSE：

| 关键运行依赖 / 资源 | 查到的许可证 |
| --- | --- |
| [`elastic/go-seccomp-bpf v1.6.0`](https://github.com/elastic/go-seccomp-bpf/blob/v1.6.0/LICENSE.txt) | Apache-2.0；是 go-sandbox 的直接依赖，不是“名称里有 Elastic 就一定是非开源许可” |
| [`golang.org/x/net v0.61.0`](https://github.com/golang/net/blob/v0.61.0/LICENSE)、[`x/sys v0.49.0`](https://github.com/golang/sys/blob/v0.49.0/LICENSE) | BSD-3-Clause；这里核查的是 go-judge 更高的直接要求，go-sandbox 自身分别要求 v0.58.0 / v0.47.0 |
| [`coreos/go-systemd/v22 v22.7.0`](https://github.com/coreos/go-systemd/blob/v22.7.0/LICENSE)、[`grpc-go v1.84.0`](https://github.com/grpc/grpc-go/blob/v1.84.0/LICENSE) | Apache-2.0 |
| [`godbus/dbus/v5 v5.2.2`](https://github.com/godbus/dbus/blob/v5.2.2/LICENSE)、[`gorilla/websocket v1.5.3`](https://github.com/gorilla/websocket/blob/v1.5.3/LICENSE) | BSD-2-Clause |
| [`gin v1.12.0`](https://github.com/gin-gonic/gin/blob/v1.12.0/LICENSE) | MIT |
| [`protobuf-go v1.36.12`](https://github.com/protocolbuffers/protobuf-go/blob/v1.36.12/LICENSE) | BSD-3-Clause |
| go-judge 内嵌 Moby seccomp profile | Apache-2.0；子目录自带来源与 NOTICE，不能被根 MIT 覆盖 [G-notice] |

**范围限制：** Go 的其余直接及间接依赖、编译器/解释器、系统镜像、语言运行库、题目数据与题目自带 SPJ 并未完整审计。这里没有得出“整个可分发镜像全是 MIT”或“不存在其他义务”的结论。

### 5.3 对 Ephyra 根 MIT LICENSE 的具体影响

- **复制或改写 Hydro 判题源码**：不能仅保留 Ephyra 根 MIT LICENSE 就视为已合规，也不能通过换语言自动消除衍生作品问题。AGPL 第 4–6 节涉及通知、修改说明、被覆盖作品的许可及对应源码；第 5 节也区分真正独立作品组成的 aggregate 与组合程序。MIT 原创代码可以保留自己的授权，但不等于整个组合可重新宣称“仅 MIT”。[L-root]
- **以独立进程 / 服务运行**：这会改变技术耦合，却**不是统一的许可证豁免**。需要分别核实是否修改 AGPL 程序、是否分发程序/镜像、是否形成一个被覆盖的组合程序，以及哪些用户通过网络与修改版交互。AGPL 第 13 节明确针对修改版的远程网络交互规定对应源码提供义务，不能因“没有向用户发二进制”就略过。[L-root]
- **不能用 HTTP、容器或不同仓库作最终法律判断**：GNU 官方 FAQ 解释，判断独立性也看通信语义；交换复杂内部结构可能影响组合判断，容器本身不改变分析。反过来，也不能仅因调用 AGPL 服务就断言 Ephyra 全部源码必然需要改为 AGPL。该 FAQ 是许可制定方的解释，不是本项目的法律裁定。[L-faq]
- **只复用 MIT 的 go-judge / go-sandbox**：其根许可与 AGPL 不同，复制/分发仍需保留版权及许可通知，且还要满足相关 Apache/BSD/其他资源条款。它们的 MIT 许可不授权复制 Hydro 的上层 AGPL 实现。[L-gojudge] [L-sandbox] [G-notice]
- 若用户希望“Ephyra 原创代码仍为 MIT”，还需明确这个目标指原创源码、混合仓库、发行包还是完整部署镜像；没有授权范围和交付方式，无法替用户断言许可边界已经成立。必要时向维护者确认范围并取得专业法律意见。

## 6. 运行条件与安全边界

以下为部署事实与待验证约束，**不是已批准的运行方案**。

1. **两种服务的位置不能混淆。** 官方独立 hydrojudge 可以远离 Hydro 平台，但它向 go-judge 的 `copyIn` 经常传 `{src: 本地路径}`，包括题目缓存、提交附件和 testlib。`src` 由沙箱服务所在宿主机读取。因此只把 `sandbox_host` 改为另一台机器的 URL，并不能证明数据通路可用；须验证路径可见性或另外的文件传输机制。[H-task] [H-sandbox-types] [G-model]
2. **生产执行环境主要是 Linux。** Hydro 文档以 Linux、编译器和本地 go-judge 服务为部署基础；go-judge README 明示 Windows/macOS 支持为实验性、不应用于生产，go-sandbox 本身仅 Linux。[D-judge] [G1] [G-sandbox]
3. **内核最低版本的文档存在粒度差异。** go-judge README 列出 Linux ≥3.10，但本次所依赖 go-sandbox 的 README 又明确：使用 kill-default seccomp 策略的当前 builder 要求 ≥4.14。不能把一个历史最低版本直接当作完整栈生产支持承诺；必须按实际版本、策略与内核验证。[G1] [G-sandbox]
4. **权限和 cgroup 是实质要求。** Hydro 文档要求 cgroup v1 或非 systemd 发行版上的 go-judge 以 root 运行；go-judge 对 v2 使用 systemd/dbus scope，并警告无权创建 cgroup 时相关限制不会生效。官方 Docker 示例使用 `--privileged`；这不是“普通容器天然足够隔离”的证据，也不意味着所有 Node/网站进程都必须 root。[D-judge] [G1]
5. **资源和执行根文件系统需匹配。** 编译器/解释器必须安装且在沙箱挂载中可见；默认检查器还使用 `/bin/bash`、awk、cmp 等工具。测试数据缓存、`/dev/shm`、tmpfs、输出大小、并发与程序内存共同消耗宿主机资源。cgroup v2 在内核 <5.19 时内存峰值计量也有差异。[D-judge] [H-checkers] [G1]
6. **执行 API 是高信任接口。** go-judge 通常绑定 localhost，但源码在 PID 1 情况下默认改为 `:5050`；`auth-token` 可为空。`copyIn.src` 允许读服务宿主机文件，只有配置了 `src-prefix` 才做对应前缀限制。不能把它当作可直接开放给选手浏览器的公共执行 API。[G-config] [G-model]
7. **两套鉴权不能想当然地连起来。** go-judge 可启用 Bearer token；然而当前 Hydro 沙箱客户端只设置 User-Agent，没有看到传该 token 的配置/代码。开启 go-judge token 后是否可用需要适配验证；平台侧的 `sid` 不会自动传给沙箱。此外 `/version`、`/config` 在源码中注册于 token 中间件之前，不能假设 token 保护所有路由。[H-sandbox-client] [G-main]
8. **补丁版本和开关本身不构成安全证明。** go-judge 当前 `go.mod` 撤回了包含符号链接逃逸问题的 `[v0.3.0, v1.10.0]` 范围；Hydro `versionCheck` 检查 `fixSymlinkEscape`。但该项失败会报告错误，函数末尾仍返回 `true`，不能把 daemon 启动成功等同于已安全阻断漏洞版本。seccomp、cgroup 控制器、挂载和网络权限均需针对实际发行物检查。[G2] [H-sandbox] [G-config]
9. **独立评测器仍需要 Node 运行时。** 当前 judge 包自身未声明 `engines`；其 register 依赖声明 Node ≥20，HEAD 所用 cac 7.1.0 元数据声明 ≥20.19.0。完整兼容版本应按选定发布物验证，不能照搬 2020 年旧仓库的 Node 10 要求。[H-package] [L-register] [cac 元数据](https://registry.npmjs.org/cac/7.1.0)

## 7. 研究启动时的待澄清问题（历史快照）

> 后续用户已选择 Go 重写独立 ephyra-judge、直接集成 go-sandbox、自有 gRPC 协议与多组织评测池。以下保留研究时的问题，不代表这些决定仍未做出；最新范围见 [首版需求基线](../design/mvp-scope.md)。原有源码、许可及运行条件证据不因方案选择而改变。

1. **“worker”具体指哪层？** 想保留成熟题型/计分能力，还是只需要安全执行用户程序？是否愿意同时保留 Hydro 平台后端？
2. **首期题型边界是什么？** 只支持普通输入输出，还是需要 SPJ、子任务依赖、交互/通信、提交答案、自测、生成数据、hack 或远程 OJ？
3. **题目与语言配置的所有权是什么？** 是否接受 Hydro 的题包与语言 ID/命令格式？哪些计分和资源单位必须保持现有行为？
4. **“MIT 项目”的范围是什么？** 仅 Ephyra 原创源码，还是希望全仓库/发行镜像都只用 MIT？是否接受单独标注并履行 AGPL 组件义务，或需要另行授权？`@hydrooj/common` 的作用域声明还需确认。
5. **实际运行环境是什么？** Linux 内核、cgroup、systemd、允许的权限、机器/容器边界，以及 judge 与 sandbox 的文件路径可见性尚未确定。
6. **准备固定哪个发行物，并验证哪些兼容性？** 当前源码与 npm 同号包已有依赖差异；协议版本、断线重派/重复回报、语言配置、数据缓存一致性与所需题型都未做互操作测试。

以上问题需要访谈回答；本文没有默认采用任何一条路线。

## 来源索引（源码链接固定到本次修订）

[H-package]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/package.json
[H-entry]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/bin/hydrojudge.js
[H-plugin]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/index.ts
[H-daemon]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/daemon.ts
[H-builtin]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/hosts/builtin.ts
[H-host]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/hosts/hydro.ts
[H-server]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrooj/src/handler/judge.ts
[H-auth]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrooj/src/service/layers/base.ts
[H-record]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrooj/src/model/record.ts
[H-types]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/common/types.ts
[H-status]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/common/status.ts
[H-lang]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/common/lang.ts
[H-session]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/interface.ts
[H-config]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/config.ts
[H-task]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/task.ts
[H-cases]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/cases.ts
[H-common-cases]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/common/cases.ts
[H-checkers]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/checkers.ts
[H-judges]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/judge/index.ts
[H-sandbox]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/sandbox.ts
[H-sandbox-client]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/sandbox/client.ts
[H-sandbox-types]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrojudge/src/sandbox/interface.ts
[D-judge]: https://github.com/hydro-dev/hydro-dev.github.io/blob/7e6c11f7fa31a9b7523b1ce2860394e010393005/content/docs/Hydro/plugins/hydrojudge.mdx
[D-data]: https://github.com/hydro-dev/hydro-dev.github.io/blob/7e6c11f7fa31a9b7523b1ce2860394e010393005/content/docs/Hydro/user/testdata.mdx
[G1]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/README.md
[G2]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/go.mod
[G-model]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/cmd/go-judge/model/model.go
[G-config]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/cmd/go-judge/config/config.go
[G-main]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/cmd/go-judge/main.go
[G-notice]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/seccomp/NOTICE
[G-sandbox]: https://github.com/criyle/go-sandbox/blob/e9dd1e32c9fea093c80c9f885f7bd724ce70c86d/README.md
[G-sandbox-mod]: https://github.com/criyle/go-sandbox/blob/e9dd1e32c9fea093c80c9f885f7bd724ce70c86d/go.mod
[L-root]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/LICENSE
[L-workspace]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/package.json
[L-utils]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/framework/utils/package.json
[L-common]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/common/package.json
[L-register]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/framework/register/package.json
[L-core]: https://github.com/hydro-dev/Hydro/blob/d6b633932cb51b6d76b678245b17a698489b826f/packages/hydrooj/package.json
[L1]: https://github.com/hydro-dev/HydroJudge/blob/7e9efc1980b9c291245a9d555c6aa884364f9bcf/package.json
[L-legacy-readme]: https://github.com/hydro-dev/HydroJudge/blob/7e9efc1980b9c291245a9d555c6aa884364f9bcf/README.md
[L-legacy-license]: https://github.com/hydro-dev/HydroJudge/blob/7e9efc1980b9c291245a9d555c6aa884364f9bcf/LICENSE
[L-gojudge]: https://github.com/criyle/go-judge/blob/4ebd2ffe9474639c665fa21cfb438f68820fe77c/LICENSE
[L-sandbox]: https://github.com/criyle/go-sandbox/blob/e9dd1e32c9fea093c80c9f885f7bd724ce70c86d/LICENSE
[L-vfork]: https://github.com/criyle/go-sandbox/blob/e9dd1e32c9fea093c80c9f885f7bd724ce70c86d/pkg/forkexec/vfork/asm_linux_amd64.s
[L-testlib]: https://github.com/MikeMirzayanov/testlib/blob/2d20123984e9479b8a56ebe0d6a51e23ad7c35b3/LICENSE
[L-testlib-header]: https://github.com/MikeMirzayanov/testlib/blob/2d20123984e9479b8a56ebe0d6a51e23ad7c35b3/testlib.h
[L-faq]: https://www.gnu.org/licenses/gpl-faq.html#MereAggregation

补充公开入口：[Hydro 评测组件文档](https://docs.hydro.ac/docs/Hydro/plugins/hydrojudge)、[测试数据文档](https://docs.hydro.ac/docs/Hydro/user/testdata)、[go-judge API 文档](https://docs.goj.ac/api)、[GNU 关于容器边界的说明](https://www.gnu.org/licenses/gpl-faq.html#AggregateContainers)。这些地址会更新；上文实现结论优先以固定修订源码为准。
