# AGENTS.md

## Core Rule

This repository must follow the shared project constraints defined in:

- `../rigel-core/AGENTS.md`

## Core Docs Location

Overall project documentation, workspace-level architecture, database notes, and deployment files are centralized in:

- `../rigel-core`

## Usage Rule

When working in this repository:

1. Read and follow `../rigel-core/AGENTS.md` first.
2. Treat `rigel-core` as the source of truth for workspace-level documentation.
3. Use this repository's local README and code layout only as module-specific supplements.
4. If a local module document conflicts with `rigel-core`, pause and reconcile instead of guessing.

## Security Supplement

1. `rigel-jd-collector` 当前按内网服务设计，不作为默认公网入口。
2. 后台调度配置、采集触发和原始商品查询能力如果要扩大暴露范围，必须先同步更新共享安全文档。
3. 京东联盟密钥、签名参数和后续返佣相关私密字段，禁止写入仓库、日志或公开接口示例。
