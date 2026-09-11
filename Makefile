# Windows: use .\scripts\make.ps1 <target> instead of make (see docs/DEV.md)
.PHONY: up down restart ps logs init clean run-rgs run-consumer run-rollback run-admin run-broadcast build install-goctl codegen seed seed-admin migrate-security migrate-wallet migrate-replay migrate-orphan-trace migrate-antiabuse migrate-money migrate-totp obfuscate-client compress-client-brotli deploy-client fetch-cf-ips

seed:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/02-seed.sql

migrate-security:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/03-security-migration.sql

migrate-wallet:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/04-wallet-reconcile-migration.sql

migrate-replay:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/05-replay-migration.sql

migrate-antiabuse:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/07-antiabuse-migration.sql

migrate-orphan-trace:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/06-orphan-trace-migration.sql
	docker compose run --rm clickhouse-init

migrate-money:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/08-money-migration.sql
	docker compose run --rm clickhouse-init

migrate-ch-money:
	chmod +x scripts/migrate_ch_money.sh
	./scripts/migrate_ch_money.sh

migrate-totp:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/09-totp-recovery-migration.sql

migrate-rbac:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/10-rbac-roles-migration.sql

migrate-platform-games:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/11-platform-games-migration.sql

migrate-operations-audit:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/12-operations-audit-migration.sql

migrate-settlement-billing:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/13-settlement-billing-migration.sql

migrate-player-profiles:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/14-player-profiles-migration.sql

migrate-api-governance:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/15-api-governance-migration.sql

migrate-game-versions:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/16-game-versions-migration.sql

migrate-index-optimize:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/17-index-optimize-migration.sql
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/18-index-optimize-migration.sql

migrate-i18n:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/24-i18n-dictionary-migration.sql

migrate-schema-optimize:
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/19-normalize-merchant-refs-migration.sql
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/20-audit-archive-policy-migration.sql
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/21-merchant-lobby-view-migration.sql
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/22-status-checks-migration.sql
	docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame < docker/mysql/init/23-seed-extra-games-migration.sql

migrate-pending:
	$(MAKE) migrate-player-profiles migrate-api-governance migrate-game-versions migrate-index-optimize migrate-schema-optimize

migrate-all:
	$(MAKE) migrate-security migrate-wallet migrate-replay migrate-orphan-trace migrate-antiabuse migrate-totp migrate-rbac migrate-money

verify-staging-money:
	chmod +x scripts/verify_staging_money.sh
	./scripts/verify_staging_money.sh

seed-admin:
	go run scripts/seed_admin.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame

seed-rbac-users:
	go run scripts/seed_rbac_users.go | docker exec -i fastgame-mysql mysql -ufastgame -pfastgame_pass fastgame

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

obfuscate-client:
	cd client && npm install && chmod +x scripts/obfuscate-build.sh && ./scripts/obfuscate-build.sh

compress-client-brotli:
	cd client && chmod +x scripts/compress-brotli.sh && ./scripts/compress-brotli.sh

deploy-client: obfuscate-client compress-client-brotli
	rm -rf web/game/assets web/game/src web/game/cocos-js web/game/index.html 2>/dev/null || true
	cp -R client/build/web-mobile/. web/game/
	@echo "Game client deployed to web/game/ — restart gateway: docker compose up -d gateway"

fetch-cf-ips:
	chmod +x deploy/origin-shield/*.sh
	./deploy/origin-shield/fetch-cloudflare-ips.sh
