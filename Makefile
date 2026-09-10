.PHONY: up down restart ps logs init clean run-rgs build install-goctl codegen

# goctl 无独立 v1.10.3 tag，从 go-zero v1.10.3 源码编译安装
install-goctl:
	@tmp=$$(mktemp -d) && \
	git clone --depth 1 --branch v1.10.3 https://github.com/zeromicro/go-zero.git $$tmp && \
	cd $$tmp/tools/goctl && go install . && \
	rm -rf $$tmp && \
	goctl --version

codegen:
	cd services/rgs && goctl api go -api api/rgs.api -dir . -style goZero

up:
	docker compose up -d

down:
	docker compose down

restart:
	docker compose down && docker compose up -d

ps:
	docker compose ps

logs:
	docker compose logs -f

init:
	docker compose up -d kafka clickhouse
	docker compose run --rm kafka-init
	docker compose run --rm clickhouse-init

clean:
	docker compose down -v

build:
	go build -o bin/rgs-api ./services/rgs

run-rgs: build
	./bin/rgs-api -f services/rgs/etc/rgs-api.yaml
