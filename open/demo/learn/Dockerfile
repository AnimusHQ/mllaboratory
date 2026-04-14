FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

COPY sdk/python /app/sdk/python
RUN pip install --no-cache-dir numpy pillow /app/sdk/python

COPY open/demo/learn /app/learn

ENTRYPOINT ["python", "/app/learn/main.py"]
