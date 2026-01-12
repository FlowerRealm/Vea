# 任务清单: 默认 TUN 网卡名改为 vea

目录: `helloagents/plan/202601112135_default-tun-interface-name-vea/`

---

## 1. 后端默认值与就绪判定
- [ ] 1.1 在 `backend/service/proxy/service.go` 将默认接口名从 `tun0` 改为 `vea`，并确保非 Linux 下默认值不按名称强制，验证 why.md#需求-默认-tun-网卡名为-vea-场景-linux-默认创建-vea
- [ ] 1.2 在 `backend/service/proxy/service.go` 兼容非 Linux 的 legacy 默认值 `tun0`（仍按地址就绪判定），验证 why.md#需求-兼容旧配置-tun0-场景-windowsmacos-旧配置仍可用

## 2. 内核配置生成（sing-box / mihomo）
- [ ] 2.1 在 `backend/service/adapters/singbox.go` 非 Linux 下当 interfaceName 为 `vea` 或 legacy `tun0` 时不写 `tun.interface_name`，验证 why.md#需求-默认-tun-网卡名为-vea-场景-windowsmacos-默认不强制名称
- [ ] 2.2 在 `backend/service/adapters/clash.go` 非 Linux 下当 interfaceName 为 `vea` 或 legacy `tun0` 时不写 `tun.device`，验证 why.md#需求-默认-tun-网卡名为-vea-场景-windowsmacos-默认不强制名称

## 3. 后端测试用例调整
- [ ] 3.1 更新 `backend/service/adapters/singbox_tun_settings_test.go` 覆盖默认值与 legacy 行为，验证 why.md#需求-兼容旧配置-tun0-场景-windowsmacos-旧配置仍可用
- [ ] 3.2 更新 `backend/service/adapters/clash_test.go` 覆盖默认值与 legacy 行为，验证 why.md#需求-默认-tun-网卡名为-vea-场景-linux-默认创建-vea
- [ ] 3.3 更新 `backend/service/proxy/service_test.go`（如涉及默认值断言）并补齐回归覆盖，验证 why.md#需求-默认-tun-网卡名为-vea-场景-windowsmacos-默认不强制名称

## 4. 前端默认值同步
- [ ] 4.1 更新前端 settings schema 默认值为 `vea`（保持三处一致），验证 why.md#需求-默认-tun-网卡名为-vea-场景-linux-默认创建-vea

## 5. 文档与知识库更新
- [ ] 5.1 更新 `docs/SING_BOX_INTEGRATION.md` 默认值说明为 `vea` 并补充非 Linux 默认策略，验证 why.md#需求-默认-tun-网卡名为-vea-场景-windowsmacos-默认不强制名称
- [ ] 5.2 更新 `helloagents/wiki/modules/backend.md` 等知识库对默认值的描述，确保与代码一致

## 6. 安全检查
- [ ] 6.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB 风险规避）

## 7. 测试
- [ ] 7.1 运行 `go test ./...`，确保后端测试全绿

