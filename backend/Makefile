.PHONY: help local build clean

help:
	@echo "可用命令:"
	@echo "  make local  - 本地开发：source .env + go run（跳验证码、连本地 DB）"
	@echo "  make build  - 编译生产二进制 → ./server"
	@echo "  make clean  - 清理"

# 本地开发：从 .env 读 QWEN_API_KEY / DATABASE_URL / 等等。
# .env 是 gitignored，自己维护。最少需要：
#   QWEN_API_KEY=sk-...
#   DATABASE_URL=postgresql://postgres:postgres@localhost:5434/study_agent?sslmode=disable
local:
	@if [ ! -f .env ]; then echo "❌ 缺 backend/.env，参见 README"; exit 1; fi
	@echo "🚀 study-agent backend 本地启动"
	@set -a && . ./.env && set +a && go run cmd/server/main.go -config config.test.yaml

build:
	@go build -o server cmd/server/main.go
	@echo "✅ 编译完成：./server"

clean:
	@rm -f server
