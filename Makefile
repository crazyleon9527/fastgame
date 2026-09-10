.PHONY: up down restart ps logs init clean run-rgs run-consumer run-rollback run-admin run-broadcast build install-goctl codegen seed seed-admin

seed:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/02-seed.sql

seed-admin:
	go run scripts/seed_admin.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame

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
	go build -o bin/consumer ./services/consumer
	go build -o bin/rollback ./services/rollback
	go build -o bin/admin-api ./services/admin
	go build -o bin/broadcast ./services/broadcast

run-rgs: build
	./bin/rgs-api -f services/rgs/etc/rgs-api.yaml

run-consumer: build
	./bin/consumer -f services/consumer/etc/consumer.yaml

run-rollback: build
	./bin/rollback -f services/rollback/etc/rollback.yaml

run-admin: build
	./bin/admin-api -f services/admin/etc/admin-api.yaml

run-broadcast: build
	./bin/broadcast -f services/broadcast/etc/broadcast.yaml
