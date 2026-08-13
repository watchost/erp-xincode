# 销售管理

## 1. 目标
实现销售订单、销售出库、销售退货、客户管理、应收联动，与采购模块对称。

## 2. 数据模型
见 [schema.md P2-1](../schema.md#p2-1-销售模块)。

- `mdm_customer` 客户主数据（信用额度、等级）；
- `sal_sales_order` / `sal_sales_order_item`；
- `sal_sales_outbound` 销售出库；
- `sal_sales_return` 销售退货。

## 3. 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET/POST | /customers / /customers | sal:customer:* | 客户 CRUD |
| PUT/DELETE | /customers/:id | | |
| GET/POST | /sales/orders | sal:order:view/create | 销售订单 |
| GET | /sales/orders/:no | | 详情 |
| PUT | /sales/orders/:no | | 修改草稿 |
| POST | /sales/orders/:no/approve | sal:order:approve | 审批 |
| POST | /sales/orders/:no/cancel | | 取消 |
| POST | /sales/outbound/scan | sal:outbound | 销售出库扫码 |
| GET | /sales/outbound | | 出库单列表 |
| POST | /sales/returns | sal:return | 退货申请 |
| POST | /sales/returns/:no/receive | sal:return | 退货入库 |

## 4. 关键业务逻辑

### 4.1 销售订单创建
- 明细必须有物料、数量、单价、折扣；
- 后端计算 `amount = qty*price*(1-discount)`、`total_amount = SUM(amount)`；
- 状态：10 草稿 → 20 已审批 → 30 部分发货 → 40 已完成 → 50 已取消；
- 单号：`SO` + yyyyMMdd + 6 位流水（用 sys_seq 表）。

### 4.2 销售出库扫码（核心事务）
一次扫码完成：库存扣减 + 台账 + 更新 shipped_qty + 结转销售成本 + 生成应收单 + 生成凭证。全部在一个事务：

```go
txManager.WithTx(ctx, func(tx *gorm.DB) error {
    // 1. 原子扣减库存（含 available_qty 守卫）
    if err := invRepo.WithTx(tx).DecrAvailable(tx, invID, qty); err != nil { return err }

    // 2. 批次/序列号（P2 批次启用时）：扣减对应批次 qty_remaining，写 serial 状态
    if material.BatchManaged {
        if err := batchRepo.WithTx(tx).Ship(batchID, qty); err != nil { return err }
    }

    // 3. 写 inv_outbound / item
    outboundRepo.WithTx(tx).Create(outbound)

    // 4. 写台账（biz_type=销售出库）
    ledgerRepo.WithTx(tx).Create(ledger)

    // 5. 更新 sal_sales_order_item.shipped_qty，原子守卫 shipped_qty+? <= qty
    //    全发完则订单状态=已完成

    // 6. 按移动平均成本结转成本
    cost := inv.AvgCost * qty
    // 借：主营业务成本 贷：库存商品
    voucherRepo.WithTx(tx).Create(costVoucher)

    // 7. 生成应收单 fin_receivable
    // 借：应收账款 贷：主营业务收入、应交税费
    receivableRepo.WithTx(tx).Create(ar)
    voucherRepo.WithTx(tx).Create(arVoucher)

    return nil
})
```

### 4.3 客户信用额度
下单时检查：`已用额度 + 本单金额 <= customer.credit_limit`，否则拒绝（可配置绕过权限）。

### 4.4 销售退货
- 退货引用原出库单；
- 入库（库存增加，按原出库成本回填 avg_cost 或当前移动平均）；
- 冲销应收（贷项通知单）；
- 冲销成本凭证（红字/反向分录）。

## 5. 边界条件
- 超发：shipped_qty+? > qty 返回错误；
- 库存不足：原子 UPDATE RowsAffected=0 返回"库存不足"；
- 审批后不能改明细；
- 退货数量不能超过原出库数量；
- 负数/零数量拒绝。

## 6. 测试要点
- 100 并发扣 100 库存 → 最终 0，无超卖；
- 事务中第 7 步失败时库存回滚；
- 超发被拒；
- 信用额度拦截；
- 退货后库存/应收正确冲销。
