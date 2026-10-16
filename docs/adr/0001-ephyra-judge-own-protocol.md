---
status: accepted
---

# Go 重写 ephyra-judge 并使用自有协议

用户选择以 hydrojudge 源码为来源开展 Go 重写，独立仓库交付 `ephyra-judge`，使用 Ephyra 自有协议；不在 Go 后端模拟 Hydro 接口，也不保留 Node/TypeScript 判题器后只替换 host。节点主动建立 TLS gRPC 流，以独立节点凭据认证，协议规范由主仓库版本化发布。

这个取舍优先满足技术栈与协议自主，但不再是低改动接入现成 worker：编译编排、SPJ、交互、文件与结果管理需要重新实现和验证，上游演进须人工比对与移植，不能假定直接合并代码即可兼容。执行底座选择直接集成 go-sandbox，见 [ADR-0002](./0002-direct-go-sandbox.md)。

## 范围与合规

首版仅兼容基础 Hydro 题包的明确子集，支持 testlib 风格 checker／interactor，不承诺 Hydro 全功能兼容。精确协议与兼容行为须按 [架构与验收计划](../design/architecture.md) 验证；开源终端本身不构成节点可信保证。

所查 hydrojudge 包元数据为 AGPL-3.0-or-later；Go 语言转换、独立仓库或不同进程不自动消除派生代码义务。具体复制／移植文件与依赖按实际授权处理，发行前确认新仓库许可及组合义务；根 MIT LICENSE 未修改，不宣称法律范围已由部署方式解决。

背景见 [复用研究](../research/hydro-worker-reuse.md)，当前产品决定见 [首版需求基线](../design/mvp-scope.md)。本决策及首版设计基线已确认；未开始实现或创建评测仓库。
