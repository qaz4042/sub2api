.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical check-entrypoints secret-scan deploy-my2g deploy-my4g deploy-my4g-backend-only deploy-my2g-docker

FRONTEND_CRITICAL_VITEST := \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@pnpm --dir frontend run test:run

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

check-entrypoints:
	@python3 tools/check_entrypoints.py

secret-scan:
	@python3 tools/secret_scan.py

# 在当前 Mac 构建前后端一体 linux/amd64 二进制，并发布到 my2g systemd 服务。
deploy-my2g:
	@./deploy/deploy-my2g-binary.sh

# 在当前 Mac 构建前后端一体 linux/amd64 二进制，并发布到香港 my4g。
deploy-my4g:
	@SSH_TARGET=my4g PUBLIC_HEALTH_URL=https://codex.lizubin.online/health ./deploy/deploy-my2g-binary.sh

# 复用现有前端 dist，仅构建后端 linux/amd64 二进制并发布到香港 my4g。
deploy-my4g-backend-only:
	@SSH_TARGET=my4g PUBLIC_HEALTH_URL=https://codex.lizubin.online/health BUILD_FRONTEND=0 ./deploy/deploy-my2g-binary.sh

# 重装前的 Docker 应用容器发布方式，仅供历史回退场景手动使用。
deploy-my2g-docker:
	@./deploy/deploy-my2g.sh
