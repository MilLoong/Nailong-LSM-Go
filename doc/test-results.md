# 测试与性能记录 🧪

2026-11-25 · Windows/amd64 · Go 1.24.5 · Intel Core i5-13500H

另见 [C++ / Go 对照表](cpp-go-comparison.html)：包含同轨迹的块缓存实测及源码差异。

## 跨实现：TinyLSM 对 GoLevelDB 📊

同机、单线程，通过两个引擎的公开 `Get` API 比较。各写入 4096 个键（128 字节 value），确认生成磁盘表，关闭重开；计时前用同一条固定种子的乱序读轨迹预热并逐个验证结果。TinyLSM 使用默认引擎配置，GoLevelDB v1.0.0 关闭压缩、开启 10 bits/key 的 Bloom。两边保留各自的块缓存与实际表布局，未统一缓存字节预算，不清空操作系统页缓存。

- **命中：** TinyLSM **597.7 µs/op**（约 1,673 次/秒），GoLevelDB **3.211 µs/op**（约 311,429 次/秒），GoLevelDB 吞吐约 **186 倍**
- **范围内未命中：** TinyLSM **551.7 µs/op**（约 1,813 次/秒），GoLevelDB **2.329 µs/op**（约 429,369 次/秒），GoLevelDB 吞吐约 **237 倍**
- **每次读取的分配次数：** 命中 232 对 14，未命中 232 对 10（TinyLSM 对 GoLevelDB）

耗时为三轮中位数，吞吐按其倒数换算。数据表明当前 TinyLSM 在这个小规模、预热后的 SST 点读负载上明显落后。它不能说明差距中有多少来自 Bloom、缓存策略、文件布局或实现开销，也不能外推至并发、写入或冷盘。

```powershell
Set-Location bench/compare
go test -run '^$' -bench '^BenchmarkPointRead$' -benchtime=250ms -count=3 -cpu=1
```

## 读路径对照 ⚡

单 SST、1024 个键、128 字节 value、4 KiB 块。每个场景串行运行 3 次，下列耗时取中位数，括号内为最小–最大值。文件系统缓存未清空，结果不代表冷盘性能、尾延迟或服务器吞吐。

- **命中，关闭块缓存：** 113.1 µs/op（86.6–190.3），约 12,965 B/op、89 allocs/op
- **命中，128 块容量且已预热：** 85.0 µs/op（58.5–88.9），约 3,892 B/op、33 allocs/op。每次分配字节减少约 70%，分配次数减少约 63%；耗时波动较大，不把这次的耗时比例当作稳定收益
- **键在 SST 范围内但不存在，关闭 Bloom：** 336.1 µs/op（151.5–1091.6），约 12,966 B/op、89 allocs/op
- **同一负查询，注入内存 Bloom：** 326.1 ns/op（283.9–362.5），16 B/op、2 allocs/op；本批 1024 次探测（1023 个不同键）全部提前拒绝

Bloom 结果只衡量**过滤器已经在内存**时跳过数据块读取和解码的收益。当前 `Open` 用 `metaOffset - bloomOffset` 计算过滤器长度，但文件布局是块数据、元数据、Bloom、footer，正常布局下该差值为负，加载分支未进入。已有 `TestSSTBloomFilter` 的负样本都大于最大键，即使没有 Bloom 也会通过。未修改生产代码，也不把微基准结果算作当前磁盘 SST 的实际收益。

独立 Bloom 单测设置目标误判率为 5%，对 10,000 个不存在的键实测误判 537 次（5.37%）。微基准中的 100% 拒绝只对该组样本成立，不代表过滤器没有误判。

## 并发与存储行为 🔎

- **缓存抗扫描：** 32 块缓存，8 个热块各读 4 次，再插入 128 个不同的一次性块，扫描结束后仍命中 8/8 个热块。相同轨迹下，严格 LRU 的理论结果是 0/8（未实测其他实现）
- **缓存策略口径：** README 将缓存称为 LRU-K，当前实现实际先比较保留的访问次数（构造器默认上限 32），次数相同再比较最近访问时间；本用例验证的是这条淘汰规则的抗扫描能力，不是标准 LRU-K 算法对照
- **共享缓存冒烟：** `TestBlockCacheConcurrency` 启动 10 个写协程和 10 个读协程，各进行 100 次操作。普通测试和单次 `-race` 都通过，但读协程各查不同的 SST ID，并非共享热块压力测试；`BlockCache.Get` 持读锁时仍修改时间戳和堆
- **SST 并发读：** `TestSSTConcurrentAccess` 在含 100 条记录的 SST 上，用 10 个 goroutine 各读 20 次，200 次全部返回预期记录。这是功能覆盖，不是并发上限或 QPS
- **事务重叠：** `TestConcurrentTransactions` 让 10 个事务同时处于活跃状态，再在一个 goroutine 中逐个写入、提交和回读。不是 10 路并行提交
- **多版本和持久化：** 全量测试覆盖 MVCC、回滚、事务隔离和重开回读；`TestBasicCompaction` 写入 1200 个键并删除其中 120 个，手动刷盘和合并后重开验证全部键，未测后台合并吞吐或崩溃恢复上限

## 复跑命令 🛠️

```powershell
go test ./... -count=1 -timeout=90s
go test ./pkg/cache -run '^TestBlockCacheScanResistance$' -v -count=1
go test ./pkg/cache -race -run '^TestBlockCacheConcurrency$' -count=1
go test ./pkg/utils -run '^TestBloomFilterFalsePositiveRate$' -v -count=1
go test ./pkg/sst -run '^$' -bench '^BenchmarkSSTReadPath$' -benchtime=250ms -count=3
```

首次记录时全量普通测试通过。跨引擎结果只覆盖本页的单线程点读。若继续量化，优先补上 1/4/8/16/32 路端到端点读与提交的吞吐和 p95/p99；先验证缓存并发安全，再测共享缓存的并发扩展性，修正 Bloom 加载后再测实际 SST 负查询。
