.PHONY: deploy-my4g deploy-my4g-backend-only

deploy-my4g:
	@SSH_TARGET=my4g \
	REMOTE_DIR=/opt/sub2api \
	SERVICE_NAME=sub2api \
	HEALTH_URL=http://127.0.0.1:8080/health \
	./deploy/private/deploy-systemd-release.sh

deploy-my4g-backend-only:
	@SSH_TARGET=my4g \
	REMOTE_DIR=/opt/sub2api \
	SERVICE_NAME=sub2api \
	HEALTH_URL=http://127.0.0.1:8080/health \
	BUILD_FRONTEND=0 \
	./deploy/private/deploy-systemd-release.sh
