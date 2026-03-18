# rigel-jd-collector

`rigel-jd-collector` 是当前系统的数据入口服务。

## 当前职责

- 读取已维护好的型号词库
- 调用京东联盟 / OpenAPI 查询电脑硬件商品
- 保存原始商品信息
- 追加价格快照
- 为后续价格清单整理提供原始数据

## 不负责什么

- 不负责型号级价格清单整理
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
- 写入 `rigel_jobs`
- 提供原始商品查询接口

## 当前接口

- `GET /healthz`
- `POST /api/v1/collect/search`
- `GET /api/v1/products`

## 当前目标

当前模块的目标很明确：

`型号词库 -> 京东联盟搜索 -> 原始商品入库 -> 原始价格入库`

## TODO / MOCK

- `TODO`: 接入经过验证的真实京东联盟客户端
- `TODO`: 把联盟字段正式映射到本地 `rigel_` 表结构
- `MOCK`: 当前开发阶段可继续保留本地 mock adapter 作为过渡
