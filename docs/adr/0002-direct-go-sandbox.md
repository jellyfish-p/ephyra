---
status: accepted
---

# ephyra-judge 直接集成 go-sandbox

用户选择在 Go 评测端直接集成 `go-sandbox` 库，而不是调用独立 `go-judge` 执行服务。该选择复用现成 Linux 隔离原语，而非从头自研内核沙箱；与此同时，Ephyra 必须自行组织所需的执行、文件生命周期、进程通信、资源限制与统计、异常清理等上层能力，不能假定自动获得 go-judge 的完整能力。

这是对现成执行服务与直接库集成之间的明确取舍，不代表相关能力已实现或已完成安全验证。应按实际选择的库版本验证支持范围和运行条件；首版设计基线已确认，尚未开始实现或进行运行安全验证。

背景见 [评测复用研究](../research/hydro-worker-reuse.md) 与 [自有协议评测端决策](./0001-ephyra-judge-own-protocol.md)。
