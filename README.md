# rigel-jd-collector

`rigel-jd-collector` 是当前系统的数据入口服务。

## 当前职责

- 调用京东联盟 / OpenAPI 查询电脑硬件商品
- 保存原始商品信息
- 追加价格快照
- 为后续价格清单整理提供原始数据

## 不负责什么

- 不负责型号级价格清单整理
- 不负责 AI 请求构建
- 不负责推荐结果生成
- 不负责页面展示

## 当前输入

- 关键词
- 类别
- 查询限制参数

## 当前输出

- 写入 `products`
- 写入 `price_snapshots`
- 写入 `jobs`
- 提供原始商品查询接口

## 当前接口

- `GET /healthz`
- `POST /api/v1/collect/search`
- `GET /api/v1/products`

## 当前目标

当前模块的目标很明确：

`京东联盟搜索 -> 原始商品入库 -> 原始价格入库`

## TODO / MOCK

- `TODO`: 接入经过验证的真实京东联盟客户端
- `MOCK`: 当前开发阶段可继续保留本地 mock adapter 作为过渡
