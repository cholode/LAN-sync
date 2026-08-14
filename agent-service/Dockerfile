FROM docker.m.daocloud.io/python:3.11-slim

WORKDIR /app

ENV AGENT_CONFIG_PATH=/app/config.yaml

COPY config.yaml ./config.yaml
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY app/agent ./agent

EXPOSE 50051

CMD ["python", "-m", "app.main"]
