# rigel-jd-collector

`rigel-jd-collector` 是当前系统的数据入口服务。

## 当前职责

- 读取已维护好的型号词库
- 根据后台配置的每日时间定时调用京东联盟 / OpenAPI 查询电脑硬件商品
- 支持后台配置每次接口请求的间隔和单次查询条数
- 保存原始商品信息
- 追加价格快照
- 整理标准型号对应的每日价格快照

## 不负责什么

- 不负责 AI 请求构建
- 不负责推荐结果生成
- 不负责页面展示

## 当前正式使用的接口

### 1. `jd.union.open.goods.query`

用途：

- 按关键词搜索商品
- 作为主采集接口

当前至少获取：

- `skuId`
- `title`
- `product_url`
- `image_url`
- `price`
- `shop_name`
- `brand_name`
- `category_info`
- `commission_rate`
- `is_promotable`
- `coupon_info`
- `raw_payload`

### 2. `jd.union.open.goods.promotiongoodsinfo.query`

用途：

- 按 `skuId` 补商品详情和推广信息

当前至少获取：

- `skuId`
- `title`
- `product_url`
- `image_url`
- `price`
- `commission_rate`
- `coupon_info`
- `promotion_status`
- `raw_payload`

### 3. `jd.union.open.category.goods.get`

用途：

- 获取类目树
- 支撑搜索分类和后台分类管理

### 4. 预留：`jd.union.open.promotion.common.get`

用途：

- 未来生成返佣推广链接

当前阶段：

- 第一版不接页面返佣功能
- 但当前商品结构需要为它预留字段

## 当前输入

- 来自词库的 `keyword`
- 类别
- 查询限制参数

## 当前输出

- 写入 `rigel_products`
- 写入 `rigel_price_snapshots`
- 写入 `rigel_parts`
- 写入 `rigel_product_part_mapping`
- 写入 `rigel_part_market_summary`
- 写入 `rigel_jobs`
- 提供原始商品查询接口

## 调度与间隔配置

当前定时采集配置由后台管理，不再以 YAML 作为业务配置真源。

当前规则：

- 没有调度配置时，服务不会自动启动定时采集
- 配置存在但 `enabled=false` 时，服务不会自动启动定时采集
- 后台可配置：
  - 是否启用
  - 每日执行时间，例如 `03:00`
  - 每次关键词请求间隔，单位秒
  - 单次查询条数限制

当前配置存储在：

- `rigel_collector_schedules`

## 当前接口

- `GET /healthz`
- `GET /api/v1/admin/schedule`
- `PUT /api/v1/admin/schedule`
- `POST /api/v1/collect/search`
- `GET /api/v1/products`

## 接口示例

### 1. 健康检查

请求：

```bash
curl http://localhost:18081/healthz
```

响应示例：

```json
{
  "status": "ok",
  "service": "rigel-jd-collector",
  "mode": "union"
}
```

### 2. 查询当前调度配置

请求：

```bash
curl http://localhost:18081/api/v1/admin/schedule
```

响应示例：

```json
{
  "configured": true,
  "config": {
    "id": "cfg-1",
    "service_name": "rigel-jd-collector",
    "enabled": true,
    "schedule_time": "03:00",
    "request_interval_seconds": 3,
    "query_limit": 5
  }
}
```

### 3. 更新调度配置

请求：

```bash
curl -X PUT http://localhost:18081/api/v1/admin/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "schedule_time": "03:00",
    "request_interval_seconds": 3,
    "query_limit": 5
  }'
```

### 4. 触发一次采集

请求：

```bash
curl -X POST http://localhost:18081/api/v1/collect/search \
  -H "Content-Type: application/json" \
  -d '{
    "keyword": "RTX 4060",
    "category": "GPU",
    "brand": "NVIDIA",
    "limit": 2,
    "persist": true
  }'
```

响应示例：

```json
{
  "job_id": "job-1",
  "mode": "union",
  "persisted": true,
  "persisted_count": 2,
  "products": [
    {
      "id": "1001001",
      "source_platform": "jd",
      "external_id": "1001001",
      "sku_id": "1001001",
      "title": "NVIDIA RTX 4060 8G 京东自营",
      "url": "https://item.jd.com/1001001.html",
      "image_url": "https://img.example.com/4060.jpg",
      "shop_name": "京东自营",
      "shop_type": "self_operated",
      "price": 2399,
      "currency": "CNY",
      "availability": "in_stock",
      "attributes": {
        "category": "GPU"
      }
    }
  ]
}
```

说明：

- `persist=true` 时会写入 `rigel_products` 和 `rigel_price_snapshots`
- 当前推荐运行模式是 `union`

### 5. 查询已采集商品

请求：

```bash
curl "http://localhost:18081/api/v1/products?category=GPU&self_operated_only=true&real_only=true&limit=20"
```

响应示例：

```json
{
  "count": 1,
  "items": [
    {
      "id": "gpu-real",
      "source_platform": "jd",
      "title": "NVIDIA RTX 4060 京东自营",
      "shop_type": "self_operated",
      "price": 2399,
      "availability": "in_stock",
      "attributes": {
        "category": "GPU"
      }
    }
  ]
}
```

说明：

- `self_operated_only=true` 只保留京东自营
- `real_only=true` 会过滤测试数据
- `category` 当前常见值包括 `CPU`、`GPU`、`RAM`、`SSD`

## 当前目标

当前模块的目标很明确：

`型号词库 -> 京东联盟搜索 -> 原始商品入库 -> 原始价格入库 -> 每日型号价格快照`

## TODO

- `TODO`: 接入经过验证的真实京东联盟客户端
- `TODO`: 将每日调度任务接到生产环境调度体系
