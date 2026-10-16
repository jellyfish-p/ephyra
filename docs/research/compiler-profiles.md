# Ephyra 首批编译器配置核验

研究启动时语言需求为 **C++23、C++26、C**。以下仅列当时核查的候选模式，不决定编译器版本或架构。

> 后续决定：首版采用 C++23、C17、C23，C++26 暂缓；C++23 接受稳定发行工具链的已验证配置，不承诺完整标准覆盖。最新范围见 [首版需求基线](../design/mvp-scope.md)，下面的官方支持状态仍保留为证据。

| 候选模式 | 官方列出的选项 | 支持边界 |
|---|---|---|
| GCC C++23 | `g++ -std=c++23` | 状态页仍标为 experimental。[1] |
| Clang C++23 | `clang++ -std=c++23` | 标为 Partial，仅实现部分特性。[2] |
| GCC C++26 | `g++ -std=c++26` | experimental；不保证与偏离最终标准的早期实现向后兼容。[1] |
| Clang C++26 | `clang++ -std=c++2c` | 页面以 C++2c 列出，明确仅部分支持 C++26。[2] |
| GCC C（待选择） | `gcc -std=c17` 或 `gcc -std=c23` | **C17／C23 仍需用户决定**。[3] |

GCC 另列 `-std=gnu++23`、`-std=gnu++26`，用于同时启用 GNU 扩展，不能与 ISO 模式混称。[1] GCC 页面称 C++26 特性自 GCC 14 起出现；这不代表完整实现，也不能据此保证旧版本接受当前页面的所有选项拼写。[1] C++26 应标注“实验性／部分支持”，不能承诺完整符合标准。[1][2]

**语言与库分开核验**：GCC 明确将 libstdc++ 状态另列；Clang 此页按语言特性列支持情况，不构成标准库覆盖承诺。[1][2] libc++／libstdc++ 的 C++23、C++26 库覆盖率本次**未核验**。C 的库设施也依赖所配 C 库，选择 `-std=c23` 不等于完整运行库支持。[3]

**日期与版本证据**：本次响应的 HTTP Date 为 2026-10-15。[1] 页脚修订日为 2026-10-03；[2] 抓取正文未见修订日期；[3] HTTP Last-Modified 为 2026-10-15。日期不代表发行版本。未访问发行公告，**当前稳定版及实际可部署版本未核验**；状态表中的版本号不应当作稳定发布证明。

## 官方来源（仅三页）

- [1] GCC C++ Standards Support：https://gcc.gnu.org/projects/cxx-status.html
- [2] Clang C++ Support：https://clang.llvm.org/cxx_status.html
- [3] GCC Language Standards：https://gcc.gnu.org/onlinedocs/gcc/Standards.html
