ARG DOCKER_HUB_MIRROR=dockerproxy.net
FROM ${DOCKER_HUB_MIRROR}/library/python:3.11-slim

WORKDIR /app

ENV AGENT_CONFIG_PATH=/app/config.yaml

COPY config.yaml ./config.yaml
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY app/agent ./agent
COPY proto ./proto
RUN python -m grpc_tools.protoc -I./proto --python_out=. ./proto/im/v1/message.proto \
    && touch ./im/__init__.py ./im/v1/__init__.py

EXPOSE 50051 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
