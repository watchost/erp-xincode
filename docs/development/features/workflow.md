# 审批工作流

## 1. 目标
可配置的审批流：多级审批、会签/或签、转办加签、驳回，用于采购订单、销售订单、盘点、调拨等单据。

## 2. 数据模型
见 [schema.md P3-2](../schema.md#p3-2-审批工作流)。

- `wf_definition` 流程定义（按业务类型，JSON 配置节点）；
- `wf_instance` 流程实例；
- `wf_task` 待办任务；
- `wf_approval` 审批记录。

## 3. 流程定义 JSON

```json
{
  "nodes": [
    { "code": "start", "type": "start" },
    {
      "code": "dept_leader",
      "type": "approval",
      "name": "部门负责人",
      "assignee": { "type": "dept_leader" },
      "next": "finance"
    },
    {
      "code": "finance",
      "type": "approval",
      "name": "财务审批",
      "assignee": { "type": "role", "value": "finance" },
      "next": "end",
      "condition": { "field": "amount", "op": ">", "value": 10000 }
    },
    { "code": "end", "type": "end" }
  ]
}
```

审批人类型：
- `user`：指定用户；
- `role`：角色下任何人（或签）；
- `dept_leader`：发起人部门负责人；
- `dept_leader_up`：上级部门负责人；
- `initiator_leader`：发起人直属上级。

节点类型：
- `approval` 审批（支持会签 all / 或签 any）；
- `cc` 抄送；
- `condition` 条件分支；
- `parallel` 并行分支。

## 4. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /workflow/definitions | 流程定义 CRUD |
| POST | /workflow/definitions/:id/deploy | 部署启用 |
| POST | /workflow/instances/start | 启动流程（biz_type+biz_no） |
| GET | /workflow/tasks/todo | 我的待办 |
| GET | /workflow/tasks/done | 我的已办 |
| POST | /workflow/tasks/:id/approve | 通过 |
| POST | /workflow/tasks/:id/reject | 驳回 |
| POST | /workflow/tasks/:id/transfer | 转办 |
| POST | /workflow/tasks/:id/add-sign | 加签 |
| GET | /workflow/instances/:no | 审批历史 |
| POST | /workflow/instances/:no/cancel | 撤销 |

## 5. 关键逻辑

### 5.1 启动流程
业务单据提交时：
1. 找到该 biz_type 已部署的定义；
2. 创建 wf_instance，状态=审批中；
3. 解析第一个审批节点，为每个审批人创建 wf_task；
4. 发站内信通知。

### 5.2 审批通过
- 写 wf_approval；
- 会签：所有审批人通过才进入下一节点；或签：任一通过即进入；
- 解析下一节点，按 condition 判断是否跳过；
- 到达 end 节点 → instance 状态=通过，回调业务 service 的 `OnApproved(biz_no)`（更新单据状态为已审批）。

### 5.3 驳回
- instance 状态=驳回；
- 回到 start 或指定节点；
- 回调业务 service 的 `OnRejected(biz_no)`。

### 5.4 业务回调
每个接入工作流的 service 实现：
```go
type Approvable interface {
    OnApproved(ctx context.Context, bizNo string) error
    OnRejected(ctx context.Context, bizNo string, reason string) error
}
```
用一个 `map[bizType]Approvable` 注册。

## 6. 边界条件
- 审批人离职/禁用：任务转交给其上级或管理员；
- 单据被撤回/删除：工作流同步撤销；
- 一个单据同一时间只能有一个运行中的流程实例；
- 审批意见必填（驳回时）。
