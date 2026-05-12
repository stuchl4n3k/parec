.PHONY: build run test vet fmt tidy clean install-cron

BIN_DIR     := bin
BIN         := $(BIN_DIR)/parec
SYS_BIN     := /usr/local/bin/parec
CRON_SRC    := .cron/parec.cron
CRON_DST    := /etc/cron.d/parec
CRON_ENV    := .cron/parec.env
ENV_DST     := /etc/parec/parec.env
WRAPPER_SRC := .cron/parec-monthly.sh
WRAPPER_DST := /usr/local/bin/parec-monthly
STATE_DIR   := /var/lib/parec
LOG_FILE    := /var/log/parec.log
# User that runs the cron job, owns the state dir, and is the group
# owner of the env file. Defaults to the account running make (the
# committed cron file uses a __CRON_USER__ placeholder substituted at
# install time). Override on the cmdline:
#     make install-cron CRON_USER=someone
CRON_USER   ?= $(USER)

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) .

# Requires ROHLIK_EMAIL and ROHLIK_PASSWORD in the environment.
# Run from the repo root: paths to data/ are relative.
run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

# Install the binary, wrapper, cron entry, env file, and state dir at
# system paths so the cron schedule no longer depends on the repo.
# Requires sudo. Refuses to install if .cron/parec.env is missing — the
# wrapper needs credentials at runtime; see .cron/parec.env.example.
install-cron: build
	@if [ ! -f $(CRON_ENV) ]; then \
	  echo "ERROR: $(CRON_ENV) not found." >&2; \
	  echo "       cp .cron/parec.env.example $(CRON_ENV) and fill in ROHLIK_EMAIL/ROHLIK_PASSWORD." >&2; \
	  exit 1; \
	fi
	sudo install -m 0755 -o root -g root $(BIN) $(SYS_BIN)
	sudo install -m 0755 -o root -g root $(WRAPPER_SRC) $(WRAPPER_DST)
	@CRON_TMP=$$(mktemp) && \
	  sed 's|__CRON_USER__|$(CRON_USER)|g' $(CRON_SRC) > $$CRON_TMP && \
	  sudo install -m 0644 -o root -g root $$CRON_TMP $(CRON_DST) && \
	  rm -f $$CRON_TMP
	sudo install -d -m 0755 -o root -g root $(dir $(ENV_DST))
	sudo install -m 0640 -o root -g $(CRON_USER) $(CRON_ENV) $(ENV_DST)
	sudo install -d -m 0755 -o $(CRON_USER) -g $(CRON_USER) $(STATE_DIR)
	@# Create the log file if absent, then enforce ownership + mode every
	@# run so existing logs aren't truncated. `install /dev/null` would
	@# clobber a populated log; touch+chown+chmod is idempotent.
	sudo touch $(LOG_FILE)
	sudo chown $(CRON_USER):$(CRON_USER) $(LOG_FILE)
	sudo chmod 0640 $(LOG_FILE)
	@echo "Installed:"
	@echo "  $(SYS_BIN)"
	@echo "  $(WRAPPER_DST)"
	@echo "  $(CRON_DST)"
	@echo "  $(ENV_DST)             (root:$(CRON_USER) 0640)"
	@echo "  $(STATE_DIR)/          ($(CRON_USER):$(CRON_USER) 0755)"
	@echo "  $(LOG_FILE)         ($(CRON_USER):$(CRON_USER) 0640)"
	@echo ""
	@echo "First run will rebuild cookies + nav cache in $(STATE_DIR)/data/."
	@echo "To bootstrap from an existing repo cache:"
	@echo "  sudo cp -a data/. $(STATE_DIR)/data/ && sudo chown -R $(CRON_USER):$(CRON_USER) $(STATE_DIR)/data"
	@echo ""
	@echo "Email delivery is handled by msmtp. Set up if not already:"
	@echo "  apt install msmtp           # or pacman/dnf equivalent"
	@echo "  cp .cron/msmtprc.example ~$(CRON_USER)/.msmtprc"
	@echo "  chmod 0600 ~$(CRON_USER)/.msmtprc && \$$EDITOR ~$(CRON_USER)/.msmtprc"
	@echo "  echo 'Subject: msmtp test' | sudo -u $(CRON_USER) msmtp -t \$$PAREC_RECIPIENT"
	@echo ""
	@echo "Smoke-test:  sudo -u $(CRON_USER) $(WRAPPER_DST)"
