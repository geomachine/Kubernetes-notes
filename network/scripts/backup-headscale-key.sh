#!/bin/sh
# Backs up ${WORK_DIR}/headscale/lib (noise_private.key + any other local
# state) to the same S3 bucket the postgres-zenager/postgres-zenlabs backups
# use — see ../../infra/base/databases/postgres-zenager/BACKUP.md for that
# pipeline. This is the one piece of Headscale state that ISN'T in the Aiven
# Postgres DB: the node/user/policy data all lives there and survives host
# loss on its own, but the server's own noise-protocol private key (its trust
# identity to every already-enrolled Tailscale client) is a local file, and
# was never backed up until this script existed. Restoring it after a host
# loss means every client re-registers against a new identity regardless —
# this just means you don't ALSO have to regenerate keys/configs from
# scratch if the box is merely being rebuilt (disk failure, redeploy, etc.)
# rather than truly gone.
#
# Run from the `network/` directory (needs WORK_DIR and the AWS_* vars,
# normally sourced from .env — see docker-compose.yaml for the same pattern).
# Crontab entry: see ../BACKUP.md.
set -e

: "${WORK_DIR:?WORK_DIR not set — source .env first}"
: "${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID not set}"
: "${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY not set}"
S3_BUCKET="${S3_BUCKET:-krakens-bucket}"
S3_BACKUP_PREFIX="${S3_BACKUP_PREFIX:-kwsbackup-headscale}"
AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
FILENAME="headscale-lib-${TIMESTAMP}.tar.gz"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

tar czf "${TMPDIR}/${FILENAME}" -C "${WORK_DIR}/headscale" lib

# Same "container, not host package" approach as the postgres backup
# CronJobs (which install awscli fresh each run) — here we just use the
# official image directly, since this runs via host cron, not k8s.
docker run --rm \
  -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
  -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
  -e AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION}" \
  -v "${TMPDIR}:/backup:ro" \
  amazon/aws-cli:2.17.62 \
  s3 cp "/backup/${FILENAME}" "s3://${S3_BUCKET}/${S3_BACKUP_PREFIX}/${FILENAME}"

echo "Backup uploaded: ${FILENAME}"
