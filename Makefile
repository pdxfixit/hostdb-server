.PHONY: default help
default: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s %s\n\033[0m", $$1, $$2}'

# general
APP_CONTAINER      := hostdb
APP_NAME           := hostdb-server
APP_VER            := 0.1
CIRCLE_WORKFLOW_ID ?= ""
DB_CONTAINER       := mariadb
DB_HOST            ?= localhost
DB_VER             := 10.3
REMOTE_HOST        ?= localhost
REMOTE_PASS        ?= badpassword
REMOTE_PORT        ?= 8080
REMOTE_USER        ?= writer
SAMPLE_FILES       := $(shell find sample-data -type f -name '*.json')
WORK_DIR           := $(shell pwd)

# git
ifeq ($(CIRCLECI), true)
GIT_BRANCH_DIRTY = $(CIRCLE_BRANCH)
else
GIT_BRANCH_DIRTY = $(shell git rev-parse --abbrev-ref HEAD)
endif
GIT_BRANCH     = $(shell echo "$(GIT_BRANCH_DIRTY)" | sed s/[[:punct:]]/_/g | tr '[:upper:]' '[:lower:]')
GIT_COMMIT_MSG = $(shell git log -1 --pretty=%B | tr '[:cntrl:]' ',' | sed 's/,*$$//g' | sed 's/,/, /g')
GIT_COMMIT_NUM = $(shell git rev-list --count HEAD)
GIT_COMMIT_SHA = $(shell git describe --tags --match '[0-9]*' --dirty --always --long)

# version
ifeq ($(GIT_BRANCH), master)
TAG     = latest
VERSION = $(APP_VER).$(GIT_COMMIT_NUM)
else
TAG     = $(GIT_BRANCH)
VERSION = $(APP_VER).$(GIT_COMMIT_NUM)-$(GIT_BRANCH)
endif
export TAG
export VERSION

# container
REGISTRY_HOST  = registry.pdxfixit.com
CONTAINER_NAME = $(REGISTRY_HOST)/$(APP_NAME)

# docker env file
ifneq ("$(wildcard env.list)","")
	DOCKER_ENV=--env-file ./env.list
endif
DOCKER_RUN_OPTIONS=-it --rm --name $(APP_NAME) $(DOCKER_ENV)

# use a go mod proxy so that we don't need deploy keys for GHE
export GOPROXY = https://goproxy.pdxfixit.com
# disable go mod sum checking for now
export GONOSUMDB = *.pdxfixit.com,github.com/pdxfixit/*

.PHONY: mariadb_start
mariadb_start: ## start a mariadb container
ifeq ($(shell nc -z 127.0.0.1 3306 > /dev/null 2>&1 ; echo $$?),1)
	# we'll pull until docker 19.04 is available ... see: https://github.com/docker/cli/pull/1498
	docker pull $(DB_CONTAINER):$(DB_VER)
	docker run -d -p 3306:3306 --rm --name $(DB_CONTAINER) -e MYSQL_ALLOW_EMPTY_PASSWORD=1 $(DB_CONTAINER):$(DB_VER)
	sleep 15
else
    $(warning MariaDB already running)
endif

.PHONY: mariadb_stop
mariadb_stop: ## stop the mariadb container
	if [ "$$(docker ps -a -q -f 'name=$(DB_CONTAINER)')" ]; then docker stop -t0 $(DB_CONTAINER); fi

.PHONY: mariadb_restart
mariadb_restart: ## stop and start the mariadb container
	$(MAKE) mariadb_stop
	sleep 3
	$(MAKE) mariadb_start

.PHONY: mariadb_client
mariadb_client: ## launch a local mariadb client
	if [ "$$(docker ps -a -q -f 'name=$(DB_CONTAINER)')" == "" ]; then $(MAKE) mariadb_start; fi
	docker run -i -t --rm --name $(DB_CONTAINER)-client $(DB_CONTAINER):$(DB_VER) mysql -h "$$(docker inspect -f "{{ .NetworkSettings.IPAddress }}" $(DB_CONTAINER))" -uroot --protocol=tcp -Dhostdb

.PHONY: sample_data
sample_data: sample_data_aws sample_data_oneview sample_data_openstack sample_data_ucs sample_data_vrops ## inject all sample data into the HostDB API

.PHONY: sample_data_aws
sample_data_aws: ## inject aws sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/aws/*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_oneview
sample_data_oneview: ## inject OneView sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/oneview/*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_openstack
sample_data_openstack: ## inject openstack sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/openstack/*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_ucs
sample_data_ucs: ## inject ucs sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/ucs/*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_vrops
sample_data_vrops: ## inject vrops sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/vrops/*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_aws_east
sample_data_aws_east: ## inject aws-east sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/aws/*east*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: sample_data_aws_west
sample_data_aws_west: ## inject aws-west sample data into the HostDB API
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	$(foreach file, $(wildcard sample-data/aws/*west*.json), printf $(file) && time curl -s "http://$(REMOTE_USER):$(REMOTE_PASS)@$(REMOTE_HOST):$(REMOTE_PORT)/v0/records/?file=$(file)" -d @$(file) -H "Content-Type: application/json" -o /dev/null || exit;)

.PHONY: validate
validate: ## validate sample data
ifeq ($(shell nc -z $(REMOTE_HOST) $(REMOTE_PORT) > /dev/null 2>&1 ; echo $$?),1)
	$(error HostDB doesn't seem to be running @ $(REMOTE_HOST):$(REMOTE_PORT))
endif
	go run sample-data/validator.go

.PHONY: get
get: ## go get will ensure dependencies are present
	go get github.com/pdxfixit/hostdb
	go get -d

.PHONY: fmt
fmt: ## go fmt
	go fmt ./...

.PHONY: vet
vet: ## go vet
	go vet -v ./...

.PHONY: lint
lint: ## golint
ifeq (, $(shell which golint))
	go install golang.org/x/lint/golint@latest
endif
	go list ./... | xargs -L1 golint

.PHONY: errcheck
errcheck: ## errcheck
ifeq (, $(shell which errcheck))
	go install github.com/kisielk/errcheck@latest
endif
	errcheck ./...

.PHONY: test
test: fmt vet lint errcheck ## run the golang tests, and lint the openapi.yaml spec file
ifeq ($(shell nc -z $(DB_HOST) 3306 > /dev/null 2>&1 ; echo $$?),1)
	$(MAKE) mariadb_start
endif
	go test -v --failfast
	speccy lint openapi.yaml

.PHONY: compile
compile: $(APP_NAME) ## compile the linux/amd64 binary with no dependencies

$(APP_NAME):
ifeq (, $(shell which gox))
	go install github.com/mitchellh/gox@latest
endif
	env CGO_ENABLED=0 gox -osarch="linux/amd64" -tags netgo -output $(APP_NAME) \
		-ldflags="-X main.appVersion=$(VERSION) \
		-X main.gitCommit=$(GIT_COMMIT_SHA) \
		-X main.buildDate=`date -u '+%Y-%m-%dT%H:%M:%SZ'` \
		-X main.buildURL=https://builds.pdxfixit.com/workflow/$(WORKFLOW_ID) \
		-X main.goVersion=`go version | awk '{print $$3}'`"

.PHONY: build
build: $(APP_NAME) ## create container image
	docker build -t $(APP_NAME) --label "version=$(VERSION)" .

.PHONY: push
push: ## push container image to registry
ifeq ($(strip $(REGISTRY_USER)),)
	$(error "Username required (e.g. make push REGISTRY_USER=username REGISTRY_PASS=password)")
endif
ifeq ($(strip $(REGISTRY_PASS)),)
	$(error "Password required (e.g. make push REGISTRY_USER=username REGISTRY_PASS=password)")
endif
	if [ "$$(docker images -q $(APP_NAME))" == "" ]; then $(MAKE) build; fi
	docker tag $(APP_NAME) $(CONTAINER_NAME):$(GIT_COMMIT_SHA)
	docker tag $(APP_NAME) $(CONTAINER_NAME):$(VERSION)
	docker tag $(APP_NAME) $(CONTAINER_NAME):$(TAG)

	@echo $(REGISTRY_PASS) | docker login -u $(REGISTRY_USER) --password-stdin $(REGISTRY_HOST)
	docker push $(CONTAINER_NAME):$(GIT_COMMIT_SHA)
	docker push $(CONTAINER_NAME):$(VERSION)
	docker push $(CONTAINER_NAME):$(TAG)

.PHONY: deploy
deploy: ## deploys an upgrade to k8s using a helm chart; requires an existing installation with secrets
ifneq ($(GIT_BRANCH), master)
	@echo "branch is not master, refusing to deploy application"
	exit 0
endif
ifndef KUBE_CONFIG
	$(error Could not find kube config)
endif
ifndef NEWRELIC_APIKEY
	$(error New Relic API Key not set)
endif
	# create .kube/config
	mkdir ~/.kube
	echo $$KUBE_CONFIG > ~/.kube/config

	# clone hostdb chart
	rm -rf chart
	git clone git@github.com:pdxfixit/hostdb-server-chart.git chart

	# report which version (git sha) of the helm chart we're deploying
	git describe --tags --match '[0-9]*' --dirty --always --long -C ./chart

	# copy the hostdb config for use by the chart
	cp config.yaml chart/hostdb-server/config.yaml

	# upgrade!
	cd chart && make install_kubectl install_helm upgrade APP_TAG=$(GIT_COMMIT_SHA)

	# notify New Relic
	curl -X POST --data-urlencode "$$(jq -n --arg msg "$(GIT_COMMIT_MSG)" sha "$(GIT_COMMIT_SHA)" '{"deployment":{"revision":"$$sha","description":$$msg,"user":"hostdb-server"}}')" "https://api.newrelic.com/v2/applications/247220021/deployments.json"

.PHONY: run
run: ## start an instance of the app
	if [ "$$(docker images -q $(APP_NAME))" == "" ]; then $(MAKE) build; fi
	if [ "$$(docker ps -a -q -f 'name=$(DB_CONTAINER)')" == "" ]; then $(MAKE) mariadb_start; fi
	docker run $(DOCKER_RUN_OPTIONS) -p 8080:8080 -e HOSTDB_MARIADB_HOST="$$(docker inspect -f "{{ .NetworkSettings.IPAddress }}" $(DB_CONTAINER))" $(APP_NAME)

.PHONY: stop
stop: ## stop the container
	if [ "$$(docker ps -a -q -f 'name=$(APP_CONTAINER)')" ]; then docker stop -t0 $(APP_CONTAINER); fi

.PHONY: clean
clean: mariadb_stop stop ## clean up any artifacts
	rm -f $(WORK_DIR)/$(APP_NAME)
	rm -rf $(WORK_DIR)/chart
	if [ "$$(docker images -q $(APP_NAME))" ]; then docker rmi $(APP_NAME); fi
