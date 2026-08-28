#!/bin/bash
# Deploys robotstxt-svc (cmd/robotstxt-svc), a gRPC service, to Cloud Run
# (us-central1). Scale-to-zero,
# IAM-authenticated, no project roles on the runtime identity — it fetches
# robots.txt from the public internet and needs nothing from GCP.
#
# Mirrors the webrisk-svc deploy conventions (same project, region, Artifact
# Registry repo, per-service no-role SA, --no-allow-unauthenticated).
#
# The image is Dockerfile.svc: the Go service ONLY. The vendored C++ parser is
# the differential-test oracle and stays in CI (root Dockerfile) — see
# docker/README.md. Its tests run in the builder stage, so a failing service
# cannot be deployed.
#
# Knobs (env vars):
#   MAX_INSTANCES=4     upper bound on concurrent instances
#   CONCURRENCY=80      requests per instance (robots.txt work is small)
#   FETCH_TIMEOUT=20s   per-robots.txt HTTP timeout inside the service
set -euo pipefail

PROJECT="speax-498608"
REGION="us-central1"
IMAGE="us-west1-docker.pkg.dev/${PROJECT}/embedder/robotstxt-svc"
SA="robotstxt-svc@${PROJECT}.iam.gserviceaccount.com"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MAX_INSTANCES="${MAX_INSTANCES:-4}"
CONCURRENCY="${CONCURRENCY:-80}"
FETCH_TIMEOUT="${FETCH_TIMEOUT:-20s}"

gcloud services enable run.googleapis.com --project="${PROJECT}"

# Runtime identity with no project roles: the service only makes outbound HTTP.
gcloud iam service-accounts describe "${SA}" --project="${PROJECT}" >/dev/null 2>&1 ||
  gcloud iam service-accounts create robotstxt-svc --project="${PROJECT}" \
    --display-name="robotstxt-svc runtime (no project roles)"

# Artifact Registry push needs docker configured for this host (idempotent;
# rewrites ~/.docker/config.json only when the helper is missing).
gcloud auth configure-docker us-west1-docker.pkg.dev --quiet

SHA=$(git -C "${HERE}" rev-parse --short HEAD)
DOCKER_BUILDKIT=1 docker build --platform linux/amd64 \
  -f "${HERE}/Dockerfile.svc" \
  -t "${IMAGE}:latest" -t "${IMAGE}:${SHA}" "${HERE}"
docker push "${IMAGE}:latest"
docker push "${IMAGE}:${SHA}"

# --use-http2 is REQUIRED for gRPC: without it Cloud Run terminates HTTP/2 at
# the frontend and speaks HTTP/1.1 to the container, which a gRPC server cannot
# answer. The symptom is every RPC failing at the transport layer.
gcloud run deploy robotstxt-svc --project="${PROJECT}" --region="${REGION}" \
  --image="${IMAGE}:${SHA}" \
  --service-account="${SA}" \
  --no-allow-unauthenticated \
  --use-http2 \
  --min-instances=0 --max-instances="${MAX_INSTANCES}" \
  --memory=512Mi --cpu=1 --concurrency="${CONCURRENCY}" \
  --timeout=120 \
  --args="-fetch-timeout=${FETCH_TIMEOUT}"

URL=$(gcloud run services describe robotstxt-svc --project="${PROJECT}" \
  --region="${REGION}" --format='value(status.url)')
echo
echo "robotstxt-svc: ${URL}"
echo "Call it:  grpcurl -H \"authorization: Bearer \$(gcloud auth print-identity-token)\" \\"
echo "            -d '{\"domain\":\"example.com\"}' \\"
echo "            ${URL#https://}:443 robotstxt.svc.v1.RobotsService/Parse"
echo "Grant callers: gcloud run services add-iam-policy-binding robotstxt-svc \\"
echo "  --region=${REGION} --member=serviceAccount:<caller-sa> --role=roles/run.invoker"
