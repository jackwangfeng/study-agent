#!/bin/bash
# Build study-agent backend (which bakes in the frontend), push to ECR,
# then SSH into the shared EC2 (same one as recompdaily) and (re)launch the
# container on port 8001. First run also bootstraps a dedicated Postgres
# container with a docker volume.
#
# Prereqs:
#   - AWS profile `recompdaily` (ECR push + Secrets Manager)
#   - AWS profile `seesaw-dev`  (EC2 Instance Connect SSH — host belongs to seesaw)
#   - ~/.ssh/id_ed25519[.pub]
#   - docker buildx with linux/arm64
#
# Usage: ./deploy.sh

set -euo pipefail
cd "$(dirname "$0")/.."   # repo root

ECR_REPO="337909756117.dkr.ecr.ap-southeast-1.amazonaws.com/study-agent-backend"
EC2_INSTANCE="i-0141753dd11e7d9b8"
EC2_IP="10.0.10.152"
EICE="eice-0e268a2e1f59935d3"
SSH_KEY="$HOME/.ssh/id_ed25519"
REGION="ap-southeast-1"

# ------------------------------------------------------------------
# 0. one-time prerequisites: ECR repo + secrets in SM
# ------------------------------------------------------------------
echo "=== [0/4] preflight: ECR + secrets ==="
AWS_PROFILE=recompdaily aws ecr describe-repositories \
  --repository-names study-agent-backend --region "$REGION" >/dev/null 2>&1 || {
  echo "  creating ECR repo study-agent-backend"
  AWS_PROFILE=recompdaily aws ecr create-repository \
    --repository-name study-agent-backend --region "$REGION" >/dev/null
}

if [ -f backend/.env ]; then
  set -a; source backend/.env; set +a
fi
: "${QWEN_API_KEY:?QWEN_API_KEY missing — set in backend/.env or env}"

if ! AWS_PROFILE=recompdaily aws secretsmanager describe-secret \
  --secret-id recompdaily/prod/qwen-api-key --region "$REGION" >/dev/null 2>&1; then
  echo "  creating SM secret recompdaily/prod/qwen-api-key"
  AWS_PROFILE=recompdaily aws secretsmanager create-secret \
    --name recompdaily/prod/qwen-api-key --secret-string "$QWEN_API_KEY" \
    --region "$REGION" >/dev/null
else
  AWS_PROFILE=recompdaily aws secretsmanager put-secret-value \
    --secret-id recompdaily/prod/qwen-api-key --secret-string "$QWEN_API_KEY" \
    --region "$REGION" >/dev/null
fi

if ! AWS_PROFILE=recompdaily aws secretsmanager describe-secret \
  --secret-id recompdaily/prod/study-agent-jwt-secret --region "$REGION" >/dev/null 2>&1; then
  echo "  creating SM secret recompdaily/prod/study-agent-jwt-secret"
  AWS_PROFILE=recompdaily aws secretsmanager create-secret \
    --name recompdaily/prod/study-agent-jwt-secret \
    --secret-string "$(openssl rand -base64 48 | tr -d '\n')" \
    --region "$REGION" >/dev/null
fi

if ! AWS_PROFILE=recompdaily aws secretsmanager describe-secret \
  --secret-id recompdaily/prod/study-agent-pg-password --region "$REGION" >/dev/null 2>&1; then
  echo "  creating SM secret recompdaily/prod/study-agent-pg-password"
  AWS_PROFILE=recompdaily aws secretsmanager create-secret \
    --name recompdaily/prod/study-agent-pg-password \
    --secret-string "$(openssl rand -hex 24)" \
    --region "$REGION" >/dev/null
fi

# ------------------------------------------------------------------
# 1. build + push arm64 image
# ------------------------------------------------------------------
echo "=== [1/4] build + push arm64 image as :v1 ==="
AWS_PROFILE=recompdaily aws ecr get-login-password --region "$REGION" \
  | docker login --username AWS --password-stdin 337909756117.dkr.ecr.ap-southeast-1.amazonaws.com >/dev/null
docker buildx build --platform linux/arm64 -f backend/Dockerfile.prod \
  -t "$ECR_REPO:v1" --push . 2>&1 | tail -3

# ------------------------------------------------------------------
# 2. push SSH key to EC2 (60-second window)
# ------------------------------------------------------------------
echo "=== [2/4] push SSH key to EC2 ==="
AWS_PROFILE=seesaw-dev aws ec2-instance-connect send-ssh-public-key \
  --region "$REGION" \
  --instance-id "$EC2_INSTANCE" \
  --instance-os-user ubuntu \
  --ssh-public-key "file://${SSH_KEY}.pub" \
  --output text >/dev/null

# ------------------------------------------------------------------
# 3. SSH in: bootstrap postgres + (re)launch backend
# ------------------------------------------------------------------
echo "=== [3/4] bootstrap postgres + redeploy backend ==="
ssh -i "$SSH_KEY" \
  -o ProxyCommand="aws ec2-instance-connect open-tunnel --profile seesaw-dev --region $REGION --instance-connect-endpoint-id $EICE --instance-id $EC2_INSTANCE" \
  -o StrictHostKeyChecking=no \
  ubuntu@"$EC2_IP" \
  "set -e
   export AWS_DEFAULT_REGION=$REGION
   QWEN_KEY=\$(aws secretsmanager get-secret-value --secret-id recompdaily/prod/qwen-api-key --query SecretString --output text)
   JWT_KEY=\$(aws secretsmanager get-secret-value --secret-id recompdaily/prod/study-agent-jwt-secret --query SecretString --output text)
   PG_PW=\$(aws secretsmanager get-secret-value --secret-id recompdaily/prod/study-agent-pg-password --query SecretString --output text)

   if ! sudo docker ps --format '{{.Names}}' | grep -q '^study_agent_db\$'; then
     echo '--- creating postgres container ---'
     sudo docker volume create study_agent_pgdata >/dev/null 2>&1 || true
     sudo docker pull m.daocloud.io/docker.io/library/postgres:16 2>&1 | tail -1
     sudo docker tag m.daocloud.io/docker.io/library/postgres:16 postgres:16 2>&1 | tail -1 || true
     sudo docker rm -f study_agent_db >/dev/null 2>&1 || true
     sudo docker run -d --name study_agent_db --restart always \\
       --network openim-docker_openim \\
       -v study_agent_pgdata:/var/lib/postgresql/data \\
       -e POSTGRES_PASSWORD=\"\$PG_PW\" \\
       -e POSTGRES_DB=study_agent \\
       postgres:16 >/dev/null
     for i in \$(seq 1 30); do
       if sudo docker exec study_agent_db pg_isready -U postgres >/dev/null 2>&1; then break; fi
       sleep 1
     done
   fi

   echo '--- pull + run backend ---'
   aws ecr get-login-password | sudo docker login --username AWS --password-stdin 337909756117.dkr.ecr.ap-southeast-1.amazonaws.com >/dev/null 2>&1
   sudo docker pull $ECR_REPO:v1 2>&1 | tail -1
   sudo docker rm -f study_agent_backend >/dev/null 2>&1 || true
   sudo docker run -d --name study_agent_backend --restart always \\
     --network openim-docker_openim \\
     -p 8001:8000 \\
     -e DATABASE_URL=\"postgres://postgres:\$PG_PW@study_agent_db:5432/study_agent?sslmode=disable\" \\
     -e QWEN_API_KEY=\"\$QWEN_KEY\" \\
     -e LLM_API_URL='https://dashscope.aliyuncs.com/compatible-mode/v1' \\
     -e LLM_MODEL='qwen3.5-omni-plus' \\
     -e SECRET_KEY=\"\$JWT_KEY\" \\
     -e GIN_MODE=release \\
     -e DEBUG=false \\
     -e SKIP_SMS_VERIFY=true \\
     $ECR_REPO:v1 >/dev/null
   sleep 4
   curl -sS http://localhost:8001/health; echo"

# ------------------------------------------------------------------
# 4. probe from local laptop
# ------------------------------------------------------------------
echo "=== [4/4] external probe ==="
curl -sS --max-time 5 http://13.215.200.80:8001/health || echo '(external port 8001 may need SG opening)'
echo
echo "Next steps:"
echo "  - CF DNS: add A study.recompdaily.com -> 13.215.200.80 (proxied)"
echo "  - CF Origin Rule (manual): hostname = study.recompdaily.com -> port 8001"
